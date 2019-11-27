package sapmhandler

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"testing/iotest"

	"github.com/golang/protobuf/proto"
	splunksapm "github.com/signalfx/sapm-proto/gen"
)

func TestNewV2TraceHandler(t *testing.T) {
	var zipper *gzip.Writer
	validProto, _ := proto.Marshal(&splunksapm.PostSpansRequest{})
	uncompressedValidProtobufReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(validProto))
	uncompressedValidProtobufReq.Header.Set(contentTypeHeader, xprotobuf)

	var gzipedValidProtobufBuf bytes.Buffer
	zipper = gzip.NewWriter(&gzipedValidProtobufBuf)
	zipper.Write(validProto)
	zipper.Flush()
	zipper.Close()
	gzipedValidProtobufReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(gzipedValidProtobufBuf.Bytes()))
	gzipedValidProtobufReq.Header.Set(contentTypeHeader, xprotobuf)
	gzipedValidProtobufReq.Header.Set(contentEncodingHeader, gzipEncoding)

	badContentTypeReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader([]byte{}))
	badContentTypeReq.Header.Set(contentTypeHeader, "application/json")

	errReader := iotest.TimeoutReader(bytes.NewReader([]byte{}))
	errReader.Read([]byte{}) // read once so that subsequent reads return an error

	badBodyReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), errReader)
	badBodyReq.Header.Set(contentTypeHeader, xprotobuf)

	badGzipReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewBuffer([]byte("hello world")))
	badGzipReq.Header.Set(contentTypeHeader, xprotobuf)
	badGzipReq.Header.Set(contentEncodingHeader, gzipEncoding)

	var emptyGZipBuf bytes.Buffer
	zipper = gzip.NewWriter(&emptyGZipBuf)
	zipper.Write([]byte{})
	zipper.Flush()
	zipper.Close()
	emptyGZipReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(emptyGZipBuf.Bytes()))
	emptyGZipReq.Header.Set(contentTypeHeader, xprotobuf)
	emptyGZipReq.Header.Set(contentEncodingHeader, gzipEncoding)

	var invalidProtubfBuf bytes.Buffer
	zipper = gzip.NewWriter(&invalidProtubfBuf)
	zipper.Write([]byte("invalid protbuf body"))
	zipper.Flush()
	zipper.Close()
	invalidProtobufReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(invalidProtubfBuf.Bytes()))
	invalidProtobufReq.Header.Set(contentTypeHeader, xprotobuf)
	invalidProtobufReq.Header.Set(contentEncodingHeader, gzipEncoding)

	type want struct {
		wantErr    bool
		statusCode int
	}
	tests := []struct {
		name string
		req  *http.Request
		want want
	}{
		{
			name: "valid protobuf returns a 200 status code",
			req:  uncompressedValidProtobufReq,
			want: want{
				statusCode: http.StatusOK,
				wantErr:    false,
			},
		},
		{
			name: "a bad request body returns error and 400 status code",
			req:  badBodyReq,
			want: want{
				statusCode: http.StatusBadRequest,
				wantErr:    true,
			},
		},
		{
			name: "valid gzipped protobuf returns a 200 status code",
			req:  gzipedValidProtobufReq,
			want: want{
				statusCode: http.StatusOK,
				wantErr:    false,
			},
		},
		{
			name: "invalid content type returns error and 400 status code",
			req:  badContentTypeReq,
			want: want{
				statusCode: http.StatusBadRequest,
				wantErr:    true,
			},
		},
		{
			name: "invalid gzip data returns error and 400 status code",
			req:  badGzipReq,
			want: want{
				statusCode: http.StatusBadRequest,
				wantErr:    true,
			},
		},
		{
			name: "invalid protobuf payload returns error and 400 status code",
			req:  invalidProtobufReq,
			want: want{
				statusCode: http.StatusBadRequest,
				wantErr:    true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var returnedErr error
			rw := httptest.NewRecorder()

			receiver := func(sapm *splunksapm.PostSpansRequest, err error) {
				returnedErr = err
			}

			handler := NewTraceHandlerV2(receiver)
			handler(rw, tt.req)
			if tt.want.wantErr != (returnedErr != nil) {
				t.Errorf("NewTraceHandlerV2() returned err = %v, wantErr = %v", returnedErr, tt.want.wantErr)
				return
			}
			if statusCode := rw.Code; tt.want.statusCode != statusCode {
				t.Errorf("NewTraceHandlerV2() returned status code '%v', wanted '%v'", statusCode, tt.want.statusCode)
			}

		})
	}
}
