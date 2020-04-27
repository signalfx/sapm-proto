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
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/gogo/protobuf/proto"
	jaegerpb "github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gen "github.com/signalfx/sapm-proto/gen"
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
	return newWorker(c, "http://local", "", false)
}

func TestPrepare(t *testing.T) {
	w := newTestWorker(newMockHTTPClient(&mockTransport{}))
	sr, err := w.prepare(context.Background(), testBatches, testSpansCount)
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
	w := newWorker(newMockHTTPClient(&mockTransport{}), "http://local", "", true)
	sr, err := w.prepare(context.Background(), testBatches, testSpansCount)
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

	ctx := context.Background()
	sr, err := w.prepare(ctx, testBatches, testBatchesCount)
	require.NoError(t, err)

	err = w.send(ctx, sr)
	require.Nil(t, err)

	received := transport.requests()
	require.Len(t, received, 1)

	r := received[0].r
	assert.Equal(t, r.Method, "POST")
	assert.Equal(t, r.Header.Get(headerContentEncoding), headerValueGZIP)
	assert.Equal(t, r.Header.Get(headerContentType), headerValueXProtobuf)
}

func TestWorkerSendNoCompression(t *testing.T) {
	transport := &mockTransport{}
	w := newWorker(newMockHTTPClient(transport), "http://local", "", true)

	ctx := context.Background()
	sr, err := w.prepare(ctx, testBatches, testBatchesCount)
	require.NoError(t, err)

	err = w.send(ctx, sr)
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

	ctx := context.Background()
	sr, err := w.prepare(ctx, testBatches, testBatchesCount)
	require.NoError(t, err)

	sendErr := w.send(ctx, sr)
	require.NotNil(t, sendErr)
	assert.Equal(t, 400, sendErr.StatusCode)
	assert.True(t, sendErr.Permanent)
	assert.Equal(t, 0, sendErr.RetryDelaySeconds)

	transport.reset(500)
	sendErr = w.send(ctx, sr)
	require.NotNil(t, sendErr)
	assert.Equal(t, 500, sendErr.StatusCode)
	assert.False(t, sendErr.Permanent)
	assert.Equal(t, 0, sendErr.RetryDelaySeconds)

	transport.reset(429)
	sendErr = w.send(ctx, sr)
	require.NotNil(t, sendErr)
	assert.Equal(t, 429, sendErr.StatusCode)
	assert.False(t, sendErr.Permanent)
	assert.Equal(t, defaultRateLimitingBackoffSeconds, sendErr.RetryDelaySeconds)

	transport.reset(429)
	transport.headers = map[string]string{headerRetryAfter: "100"}
	sendErr = w.send(ctx, sr)
	require.NotNil(t, sendErr)
	assert.Equal(t, 429, sendErr.StatusCode)
	assert.False(t, sendErr.Permanent)
	assert.Equal(t, 100, sendErr.RetryDelaySeconds)

	transport.reset(200)
	transport.err = errors.New("test error")
	sendErr = w.send(ctx, sr)
	require.NotNil(t, sendErr)
	assert.Equal(t, "Post \"http://local\": test error", sendErr.Error())
	assert.Equal(t, 0, sendErr.StatusCode)
	assert.False(t, sendErr.Permanent)
	assert.Equal(t, 0, sendErr.RetryDelaySeconds)
}
