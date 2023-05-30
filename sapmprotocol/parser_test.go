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

package sapmprotocol

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/jaegertracing/jaeger/model"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	splunksapm "github.com/signalfx/sapm-proto/gen"
	"github.com/signalfx/sapm-proto/internal/testhelpers"
)

func TestNewV2TraceHandler(t *testing.T) {
	var zipper *gzip.Writer
	validSapm := &splunksapm.PostSpansRequest{
		Batches: []*model.Batch{
			{
				Spans: []*model.Span{
					{
						OperationName: "hello",
					},
				},
				Process: &model.Process{
					ServiceName: "test_service",
				},
			},
		},
	}
	validProto, _ := validSapm.Marshal()
	uncompressedValidProtobufReq := httptest.NewRequest(
		http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(validProto),
	)
	uncompressedValidProtobufReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)

	var gzippedValidProtobufBuf bytes.Buffer
	zipper = gzip.NewWriter(&gzippedValidProtobufBuf)
	zipper.Write(validProto)
	zipper.Close()
	gzippedValidProtobufReq := httptest.NewRequest(
		http.MethodPost, path.Join("http://localhost", TraceEndpointV2),
		bytes.NewReader(gzippedValidProtobufBuf.Bytes()),
	)
	gzippedValidProtobufReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)
	gzippedValidProtobufReq.Header.Set(ContentEncodingHeaderName, GZipEncodingHeaderValue)
	gzippedValidProtobufReq.Header.Set(AcceptEncodingHeaderName, GZipEncodingHeaderValue)

	var zstdValidProtobufBuf bytes.Buffer
	zstder, err := zstd.NewWriter(&zstdValidProtobufBuf)
	require.NoError(t, err)
	zstder.Write(validProto)
	zstder.Close()
	zstdValidProtobufReq := httptest.NewRequest(
		http.MethodPost, path.Join("http://localhost", TraceEndpointV2),
		bytes.NewReader(zstdValidProtobufBuf.Bytes()),
	)
	zstdValidProtobufReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)
	zstdValidProtobufReq.Header.Set(ContentEncodingHeaderName, ZStdEncodingHeaderValue)
	zstdValidProtobufReq.Header.Set(AcceptEncodingHeaderName, ZStdEncodingHeaderValue)

	badContentTypeReq := httptest.NewRequest(
		http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader([]byte{}),
	)
	badContentTypeReq.Header.Set(ContentTypeHeaderName, "application/json")

	errReader := iotest.TimeoutReader(bytes.NewReader([]byte{}))
	errReader.Read([]byte{}) // read once so that subsequent reads return an error

	badBodyReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), errReader)
	badBodyReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)

	badGzipReq := httptest.NewRequest(
		http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewBuffer([]byte("hello world")),
	)
	badGzipReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)
	badGzipReq.Header.Set(ContentEncodingHeaderName, GZipEncodingHeaderValue)

	badZstdReq := httptest.NewRequest(
		http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewBuffer([]byte("hello world")),
	)
	badZstdReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)
	badZstdReq.Header.Set(ContentEncodingHeaderName, ZStdEncodingHeaderValue)

	var emptyGZipBuf bytes.Buffer
	zipper = gzip.NewWriter(&emptyGZipBuf)
	zipper.Write([]byte{})
	zipper.Close()
	emptyGZipReq := httptest.NewRequest(
		http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(emptyGZipBuf.Bytes()),
	)
	emptyGZipReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)
	emptyGZipReq.Header.Set(ContentEncodingHeaderName, GZipEncodingHeaderValue)

	var emptyZstdBuf bytes.Buffer
	zstder, err = zstd.NewWriter(&emptyZstdBuf)
	assert.NoError(t, err)
	zstder.Write([]byte{})
	zstder.Close()
	emptyZstdReq := httptest.NewRequest(
		http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(emptyZstdBuf.Bytes()),
	)
	emptyZstdReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)
	emptyZstdReq.Header.Set(ContentEncodingHeaderName, ZStdEncodingHeaderValue)

	var invalidProtubfBuf bytes.Buffer
	zipper = gzip.NewWriter(&invalidProtubfBuf)
	zipper.Write([]byte("invalid protbuf body"))
	zipper.Close()
	invalidProtobufReq := httptest.NewRequest(
		http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(invalidProtubfBuf.Bytes()),
	)
	invalidProtobufReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)
	invalidProtobufReq.Header.Set(ContentEncodingHeaderName, GZipEncodingHeaderValue)

	type want struct {
		wantErr bool
		sapm    *splunksapm.PostSpansRequest
	}
	tests := []struct {
		name string
		req  *http.Request
		want want
	}{
		{
			name: "valid protobuf returns and valid sapm",
			req:  uncompressedValidProtobufReq,
			want: want{
				sapm:    validSapm,
				wantErr: false,
			},
		},
		{
			name: "a bad request body returns error and nil sapm",
			req:  badBodyReq,
			want: want{
				sapm:    nil,
				wantErr: true,
			},
		},
		{
			name: "valid gzipped protobuf returns and valid sapm",
			req:  gzippedValidProtobufReq,
			want: want{
				sapm:    validSapm,
				wantErr: false,
			},
		},
		{
			name: "empty gzipped protobuf returns and valid sapm",
			req:  emptyGZipReq,
			want: want{
				sapm:    &splunksapm.PostSpansRequest{},
				wantErr: false,
			},
		},
		{
			name: "valid zstd protobuf returns and valid sapm",
			req:  zstdValidProtobufReq,
			want: want{
				sapm:    validSapm,
				wantErr: false,
			},
		},
		{
			name: "empty zstd protobuf returns and valid sapm",
			req:  emptyZstdReq,
			want: want{
				sapm:    &splunksapm.PostSpansRequest{},
				wantErr: false,
			},
		},
		{
			name: "invalid content type returns error and nil sapm",
			req:  badContentTypeReq,
			want: want{
				sapm:    nil,
				wantErr: true,
			},
		},
		{
			name: "invalid gzip data returns error and nil sapm",
			req:  badGzipReq,
			want: want{
				sapm:    nil,
				wantErr: true,
			},
		},
		{
			name: "invalid zstd data returns error and nil sapm",
			req:  badZstdReq,
			want: want{
				sapm:    nil,
				wantErr: true,
			},
		},
		{
			name: "invalid protobuf payload returns error and nil sapm",
			req:  invalidProtobufReq,
			want: want{
				sapm:    nil,
				wantErr: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sapm, err := ParseTraceV2Request(tt.req)
			if tt.want.wantErr != (err != nil) {
				t.Errorf("ParseTraceV2Request() returned err = %v, wantErr = %v", err, tt.want.wantErr)
				return
			}
			if !reflect.DeepEqual(sapm, tt.want.sapm) {
				t.Errorf("ParseTraceV2Request() sapm returned = %v, wanted = %v", sapm, tt.want.sapm)
			}
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	batch := &model.Batch{
		Process: &model.Process{ServiceName: "spring"},
		Spans:   []*model.Span{},
	}
	smallN := 150
	for i := 0; i < smallN; i++ {
		batch.Spans = append(batch.Spans, &model.Span{TraceID: model.NewTraceID(0, 1), SpanID: model.NewSpanID(2), OperationName: "jonatan", Duration: time.Microsecond * 1,
			Tags:      model.KeyValues{{Key: "span.kind", VStr: "client", VType: model.StringType}},
			StartTime: time.Now().UTC()})
	}

	sapmReq := splunksapm.PostSpansRequest{Batches: []*model.Batch{batch}}
	bb, err := sapmReq.Marshal()
	if err != nil {
		b.Fatal(err.Error())
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		valid := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(bb))
		valid.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)
		sapm, err := ParseTraceV2Request(valid)
		if err != nil {
			b.Fatal(err.Error())
		}
		batch := sapm.Batches[0]
		if len(batch.Spans) != smallN {
			b.Fatalf("wrong size %d", len(batch.Spans))
		}
	}
}

func zstdBytes(uncompressedBytes []byte) []byte {
	buf := bytes.NewBuffer(nil)
	w, err := zstd.NewWriter(buf)
	if err != nil {
		panic(err)
	}
	w.Write(uncompressedBytes)
	w.Close()
	zstdBytes := buf.Bytes()
	return zstdBytes
}

func gzipBytes(uncompressedBytes []byte) []byte {
	buf := bytes.NewBuffer(nil)
	w := gzip.NewWriter(buf)
	w.Write(uncompressedBytes)
	w.Close()
	gzipBytes := buf.Bytes()
	return gzipBytes
}

type decodeTest struct {
	name            string
	contentEncoding string
	requestBytes    []byte
}

func benchmarkDecodeTest(b *testing.B, test decodeTest, batchSize int) {
	// How many concurrent goroutines to use to generate requests and decode them.
	// Using numbers larger than 1 better mimics what a typical http server does when
	// it receives concurrent incoming requests.
	const requestConcurrency = 100

	var wg sync.WaitGroup
	var requestCount int64
	for i := 0; i < requestConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for atomic.AddInt64(&requestCount, 1) < int64(b.N) {
				// Create a request with precomputed payload.
				valid := httptest.NewRequest(
					http.MethodPost, path.Join("http://localhost", TraceEndpointV2),
					bytes.NewReader(test.requestBytes),
				)
				valid.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)
				if test.contentEncoding != "" {
					valid.Header.Set(ContentEncodingHeaderName, test.contentEncoding)
				}

				// And parse the request. This is the part we want to measure,
				// but the preceding request creation is so fast it does not
				// impact the benchmark in any significant way.
				sapm, err := ParseTraceV2Request(valid)
				if err != nil {
					require.FailNow(b, err.Error())
				}
				batch := sapm.Batches[0]
				require.EqualValues(b, len(batch.Spans), batchSize)
			}
		}()
		wg.Wait()
	}
}

func BenchmarkDecodeMatrix(b *testing.B) {

	batchSizes := []int{1, 10, 100, 1000}
	for _, batchSize := range batchSizes {

		// Encode the batch to binary ProtoBuf.
		sapmData := testhelpers.CreateSapmData(batchSize)

		uncompressedBytes, err := sapmData.Marshal()
		if err != nil {
			b.Fatal(err.Error())
		}

		gzipBytes := gzipBytes(uncompressedBytes)
		zstdBytes := zstdBytes(uncompressedBytes)

		tests := []decodeTest{
			{
				name:         "none",
				requestBytes: uncompressedBytes,
			},
			{
				name:            "gzip",
				requestBytes:    gzipBytes,
				contentEncoding: GZipEncodingHeaderValue,
			},
			{
				name:            "zstd",
				requestBytes:    zstdBytes,
				contentEncoding: ZStdEncodingHeaderValue,
			},
		}

		for _, test := range tests {
			b.Run(
				test.name+"/batch="+strconv.Itoa(batchSize), func(b *testing.B) {
					benchmarkDecodeTest(b, test, batchSize)
				},
			)
		}
	}
}
