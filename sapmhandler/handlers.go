package sapmhandler

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/golang/protobuf/proto"
	splunksapm "github.com/signalfx/sapm-proto/gen"
)

const (
	// TraceEndpointV2 is the endpoint used for SAPM v2 traces.  The SAPM protocol started with v2.  There is no v1.
	TraceEndpointV2 = "/v2/trace"

	contentTypeHeader = "Content-Type"
	xprotobuf         = "application/x-protobuf"

	contentEncodingHeader = "Content-Encoding"
	gzipEncoding          = "gzip"
)

var (
	// ErrBadContentType indicates an incompatible content type was received
	ErrBadContentType = errors.New("bad content type")

	gzipReaderPool = &sync.Pool{
		New: func() interface{} {
			// create a new gzip reader with a bytes reader and array of bytes containing only the gzip header
			r, _ := gzip.NewReader(bytes.NewReader([]byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 0, 0, 0, 255, 255, 1, 0, 0, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0}))
			return r
		},
	}
)

// ParseTraceV2Request processes an http request request into SAPM
func ParseTraceV2Request(req *http.Request) (*splunksapm.PostSpansRequest, error) {
	// content type MUST be application/x-protobuf
	if req.Header.Get(contentTypeHeader) != xprotobuf {
		return nil, ErrBadContentType
	}

	var err error
	var reader io.Reader

	// content encoding SHOULD be gzip
	if req.Header.Get(contentEncodingHeader) == gzipEncoding {
		// get the gzip reader
		reader = gzipReaderPool.Get().(*gzip.Reader)
		defer gzipReaderPool.Put(reader)

		// reset the reader with the request body
		err = reader.(*gzip.Reader).Reset(req.Body)
		if err != nil {
			return nil, err
		}
	} else {
		reader = req.Body
	}

	var reqBytes []byte

	// read all of the data
	reqBytes, err = ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var sapm = &splunksapm.PostSpansRequest{}

	// unmarshal request body
	err = proto.Unmarshal(reqBytes, sapm)
	if err != nil {
		return sapm, err
	}

	return sapm, err
}

// NewTraceHandlerV2 returns an http.HandlerFunc for receiving SAPM requests and passing the SAPM to a receiving function
func NewTraceHandlerV2(receiver func(sapm *splunksapm.PostSpansRequest, err error)) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		sapm, err := ParseTraceV2Request(req)
		// errors processing the request should return http.StatusBadRequest
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
		}

		// pass the SAPM and error to the receiver function
		receiver(sapm, err)

		// return an response
		rw.Header().Set(contentTypeHeader, xprotobuf)
		respBytes, _ := proto.Marshal(&splunksapm.PostSpansRequest{})
		rw.Write(respBytes)
	}
}
