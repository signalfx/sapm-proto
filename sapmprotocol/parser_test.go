package sapmprotocol

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"testing"
	"testing/iotest"

	"github.com/golang/protobuf/proto"
	splunksapm "github.com/signalfx/sapm-proto/gen"
)

func TestNewV2TraceHandler(t *testing.T) {
	var zipper *gzip.Writer
	validSapm := &splunksapm.PostSpansRequest{}
	validProto, _ := proto.Marshal(validSapm)
	uncompressedValidProtobufReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(validProto))
	uncompressedValidProtobufReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)

	var gzippedValidProtobufBuf bytes.Buffer
	zipper = gzip.NewWriter(&gzippedValidProtobufBuf)
	zipper.Write(validProto)
	zipper.Close()
	gzippedValidProtobufReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(gzippedValidProtobufBuf.Bytes()))
	gzippedValidProtobufReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)
	gzippedValidProtobufReq.Header.Set(ContentEncodingHeaderName, GZipEncodingHeaderValue)
	gzippedValidProtobufReq.Header.Set(AcceptEncodingHeaderName, GZipEncodingHeaderValue)

	badContentTypeReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader([]byte{}))
	badContentTypeReq.Header.Set(ContentTypeHeaderName, "application/json")

	errReader := iotest.TimeoutReader(bytes.NewReader([]byte{}))
	errReader.Read([]byte{}) // read once so that subsequent reads return an error

	badBodyReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), errReader)
	badBodyReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)

	badGzipReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewBuffer([]byte("hello world")))
	badGzipReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)
	badGzipReq.Header.Set(ContentEncodingHeaderName, GZipEncodingHeaderValue)

	var emptyGZipBuf bytes.Buffer
	zipper = gzip.NewWriter(&emptyGZipBuf)
	zipper.Write([]byte{})
	zipper.Close()
	emptyGZipReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(emptyGZipBuf.Bytes()))
	emptyGZipReq.Header.Set(ContentTypeHeaderName, ContentTypeHeaderValue)
	emptyGZipReq.Header.Set(ContentEncodingHeaderName, GZipEncodingHeaderValue)

	var invalidProtubfBuf bytes.Buffer
	zipper = gzip.NewWriter(&invalidProtubfBuf)
	zipper.Write([]byte("invalid protbuf body"))
	zipper.Close()
	invalidProtobufReq := httptest.NewRequest(http.MethodPost, path.Join("http://localhost", TraceEndpointV2), bytes.NewReader(invalidProtubfBuf.Bytes()))
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
