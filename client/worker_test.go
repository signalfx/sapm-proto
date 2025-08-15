// Copyright 2019 Splunk, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"

	"github.com/gogo/protobuf/proto"
	jaegerpb "github.com/jaegertracing/jaeger-idl/model/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	gen "github.com/signalfx/sapm-proto/gen"
	"github.com/signalfx/sapm-proto/internal/testhelpers"
)

var (
	testBatches = []*jaegerpb.Batch{
		{
			Process: &jaegerpb.Process{
				ServiceName: "serviceA",
				Tags:        []jaegerpb.KeyValue{{Key: "k", VStr: "v", VType: jaegerpb.ValueType_STRING}},
			},
			Spans: []*jaegerpb.Span{{
				TraceID:       jaegerpb.NewTraceID(1, 1),
				SpanID:        jaegerpb.NewSpanID(1),
				OperationName: "op1",
			}, {
				TraceID:       jaegerpb.NewTraceID(2, 2),
				SpanID:        jaegerpb.NewSpanID(2),
				OperationName: "op2",
			}},
		},
		{
			Process: &jaegerpb.Process{
				ServiceName: "serviceB",
				Tags:        []jaegerpb.KeyValue{{Key: "k", VInt64: 123, VType: jaegerpb.ValueType_INT64}},
			},
			Spans: []*jaegerpb.Span{{
				TraceID:       jaegerpb.NewTraceID(3, 3),
				SpanID:        jaegerpb.NewSpanID(3),
				OperationName: "op3",
			}, {
				TraceID:       jaegerpb.NewTraceID(3, 3),
				SpanID:        jaegerpb.NewSpanID(4),
				OperationName: "op4",
			}},
		},
	}
	testBatchesCount = 2
	testSpansCount   = 4
)

func newTestWorker(c *http.Client) *worker {
	w, err := newWorker(c, "http://local", "", false, CompressionMethodGzip, trace.NewNoopTracerProvider())
	if err != nil {
		panic(err)
	}
	return w
}

func newTestWorkerWithCompression(c *http.Client, disableCompression bool) *worker {
	w, err := newWorker(c, "http://local", "", disableCompression, CompressionMethodGzip, trace.NewNoopTracerProvider())
	if err != nil {
		panic(err)
	}
	return w
}

func TestPrepare(t *testing.T) {
	w := newTestWorker(newMockHTTPClient(&mockTransport{}))
	sr, err := w.prepare(testBatches, testSpansCount)
	assert.NoError(t, err)

	assert.Equal(t, testBatchesCount, int(sr.batches))
	assert.Equal(t, int64(testSpansCount), sr.spans)

	// cannot unmarshal compressed message
	err = proto.Unmarshal(sr.message, &gen.PostSpansRequest{})
	require.Error(t, err)

	gz, err := gzip.NewReader(bytes.NewReader(sr.message))
	require.NoError(t, err)
	defer gz.Close()

	contents, err := ioutil.ReadAll(gz)
	require.NoError(t, err)

	psr := &gen.PostSpansRequest{}
	err = proto.Unmarshal(contents, psr)
	require.NoError(t, err)

	require.Len(t, psr.Batches, testBatchesCount)

	require.EqualValues(t, testBatches, psr.Batches)
}

func TestPrepareNoCompression(t *testing.T) {
	w := newTestWorkerWithCompression(newMockHTTPClient(&mockTransport{}), true)
	sr, err := w.prepare(testBatches, testSpansCount)
	assert.NoError(t, err)

	assert.Equal(t, testBatchesCount, int(sr.batches))
	assert.Equal(t, int64(testSpansCount), sr.spans)

	psr := &gen.PostSpansRequest{}
	err = proto.Unmarshal(sr.message, psr)
	require.NoError(t, err)

	require.Len(t, psr.Batches, testBatchesCount)

	require.EqualValues(t, testBatches, psr.Batches)
}

func TestWorkerSend(t *testing.T) {
	transport := &mockTransport{}
	w := newTestWorker(newMockHTTPClient(transport))

	sr, err := w.prepare(testBatches, testBatchesCount)
	require.NoError(t, err)

	_, err = w.send(context.Background(), sr, "")
	require.Nil(t, err)

	received := transport.requests()
	require.Len(t, received, 1)

	r := received[0].r
	assert.Equal(t, r.Method, "POST")
	assert.EqualValues(t, r.Header.Get(headerContentEncoding), CompressionMethodGzip)
	assert.Equal(t, r.Header.Get(headerContentType), headerValueXProtobuf)
}

func TestWorkerSendWithAccessToken(t *testing.T) {
	transport := &mockTransport{}
	w := newTestWorker(newMockHTTPClient(transport))

	sr, err := w.prepare(testBatches, testBatchesCount)
	require.NoError(t, err)

	_, err = w.send(context.Background(), sr, "Preferential")
	require.Nil(t, err)

	received := transport.requests()
	require.Len(t, received, 1)

	r := received[0].r
	assert.Equal(t, r.Method, "POST")
	assert.EqualValues(t, r.Header.Get(headerContentEncoding), CompressionMethodGzip)
	assert.Equal(t, r.Header.Get(headerContentType), headerValueXProtobuf)
	assert.Equal(t, r.Header.Get(headerAccessToken), "Preferential")
}

func TestWorkerSendDefaultsToWorkerToken(t *testing.T) {
	transport := &mockTransport{}
	w := newTestWorker(newMockHTTPClient(transport))
	w.accessToken = "WorkerToken"

	sr, err := w.prepare(testBatches, testBatchesCount)
	require.NoError(t, err)

	_, err = w.send(context.Background(), sr, "")
	require.Nil(t, err)

	received := transport.requests()
	require.Len(t, received, 1)

	r := received[0].r
	assert.Equal(t, r.Method, "POST")
	assert.EqualValues(t, r.Header.Get(headerContentEncoding), CompressionMethodGzip)
	assert.Equal(t, r.Header.Get(headerContentType), headerValueXProtobuf)
	assert.Equal(t, r.Header.Get(headerAccessToken), "WorkerToken")
}

func TestWorkerSendNoCompression(t *testing.T) {
	transport := &mockTransport{}
	w := newTestWorkerWithCompression(newMockHTTPClient(transport), true)

	sr, err := w.prepare(testBatches, testBatchesCount)
	require.NoError(t, err)

	_, err = w.send(context.Background(), sr, "")
	require.Nil(t, err)

	received := transport.requests()
	require.Len(t, received, 1)

	r := received[0].r
	assert.Equal(t, r.Method, "POST")
	assert.Equal(t, r.Header.Get(headerContentEncoding), "")
	assert.Equal(t, r.Header.Get(headerContentType), headerValueXProtobuf)
}

func TestWorkerSendErrors(t *testing.T) {
	transport := &mockTransport{statusCode: 400}
	w := newTestWorker(newMockHTTPClient(transport))

	sr, err := w.prepare(testBatches, testBatchesCount)
	require.NoError(t, err)

	_, sendErr := w.send(context.Background(), sr, "")
	require.NotNil(t, sendErr)
	assert.Equal(t, 400, sendErr.StatusCode)
	assert.True(t, sendErr.Permanent)
	assert.Equal(t, 0, sendErr.RetryDelaySeconds)

	transport.reset(500)
	_, sendErr = w.send(context.Background(), sr, "")
	require.NotNil(t, sendErr)
	assert.Equal(t, 500, sendErr.StatusCode)
	assert.False(t, sendErr.Permanent)
	assert.Equal(t, 0, sendErr.RetryDelaySeconds)

	transport.reset(429)
	_, sendErr = w.send(context.Background(), sr, "")
	require.NotNil(t, sendErr)
	assert.Equal(t, 429, sendErr.StatusCode)
	assert.False(t, sendErr.Permanent)
	assert.Equal(t, defaultRateLimitingBackoffSeconds, sendErr.RetryDelaySeconds)

	transport.reset(429)
	transport.headers = map[string]string{headerRetryAfter: "100"}
	_, sendErr = w.send(context.Background(), sr, "")
	require.NotNil(t, sendErr)
	assert.Equal(t, 429, sendErr.StatusCode)
	assert.False(t, sendErr.Permanent)
	assert.Equal(t, 100, sendErr.RetryDelaySeconds)

	transport.reset(200)
	transport.err = errors.New("test error")
	_, sendErr = w.send(context.Background(), sr, "")
	require.NotNil(t, sendErr)
	assert.Contains(t, sendErr.Error(), "test error")
	assert.Equal(t, 0, sendErr.StatusCode)
	assert.False(t, sendErr.Permanent)
	assert.Equal(t, 0, sendErr.RetryDelaySeconds)
}

func TestWorkerIngestResponse(t *testing.T) {
	response := `{"valid": 3, "invalid":{"invalidSpanID": ["traceID:invalidSpanID"],
		"invalidTraceID": ["invalidTraceID:spanid"],
		"zeroTraceID": ["0000000000000000:0000000000000000"]}}`
	transport := &mockTransport{statusCode: 200, body: response}
	w := newTestWorker(newMockHTTPClient(transport))

	sr, err := w.prepare(testBatches, testBatchesCount)
	require.NoError(t, err)

	ingestResponse, sendErr := w.send(context.Background(), sr, "")
	require.Nil(t, sendErr)
	assert.Equal(t, response, string(ingestResponse.Body))

	transport.reset(400)
	ingestResponse, sendErr = w.send(context.Background(), sr, "")
	require.NotNil(t, sendErr)
	assert.Equal(t, 400, sendErr.StatusCode)
	assert.Equal(t, response, string(ingestResponse.Body))

	transport.reset(429)
	ingestResponse, sendErr = w.send(context.Background(), sr, "")
	require.NotNil(t, sendErr)
	assert.Equal(t, 429, sendErr.StatusCode)
	assert.Equal(t, response, string(ingestResponse.Body))

	transport.reset(500)
	ingestResponse, sendErr = w.send(context.Background(), sr, "")
	require.NotNil(t, sendErr)
	assert.Equal(t, 500, sendErr.StatusCode)
	assert.Equal(t, response, string(ingestResponse.Body))
}

func TestCompressionSize(t *testing.T) {
	fmt.Println("Message byte size by batch size and compression method.")
	fmt.Printf("Compression ")
	for _, test := range compressionTests {
		fmt.Printf("%10v", test.name)
	}
	fmt.Printf("\n")

	batchSizes := []int{1, 10, 100, 1000, 10000}
	for _, batchSize := range batchSizes {

		fmt.Printf("Batch=%-5v ", batchSize)

		var byteSizes []int
		for _, test := range compressionTests {
			sapmData := testhelpers.CreateSapmData(batchSize)

			transport := &mockTransport{}
			client := newMockHTTPClient(transport)
			w, err := newWorker(
				client,
				"http://local",
				"",
				test.disableCompression,
				test.compressionMethod,
				trace.NewNoopTracerProvider(),
			)
			require.NoError(t, err)
			sr, err := w.prepare(sapmData.Batches, len(sapmData.Batches))
			require.NoError(t, err)

			byteSizes = append(byteSizes, len(sr.message))
		}
		for _, s := range byteSizes {
			fmt.Printf("%10v", s)
		}
		fmt.Printf("\n")
	}
}

func BenchmarkCompression(b *testing.B) {

	batchSizes := []int{1, 100, 1000}
	for _, batchSize := range batchSizes {
		for _, test := range compressionTests {
			sapmData := testhelpers.CreateSapmData(batchSize)

			b.Run(
				test.name+"/batch="+strconv.Itoa(batchSize), func(b *testing.B) {
					transport := &mockTransport{}
					client := newMockHTTPClient(transport)
					w, err := newWorker(
						client,
						"http://local",
						"",
						test.disableCompression,
						test.compressionMethod,
						trace.NewNoopTracerProvider(),
					)
					require.NoError(b, err)

					for i := 0; i < b.N; i++ {
						sr, err := w.prepare(sapmData.Batches, len(sapmData.Batches))
						require.NoError(b, err)

						_, err = w.send(context.Background(), sr, "")
						require.Nil(b, err)

						received := transport.requests()
						require.Len(b, received, i+1)

						r := received[i].r
						assert.EqualValues(b, r.Header.Get(headerContentEncoding), test.compressionMethod)
					}
				},
			)
		}
	}
}
