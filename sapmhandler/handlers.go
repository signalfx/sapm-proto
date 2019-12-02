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

	acceptEncodingHeader  = "Accept-Encoding"
	contentEncodingHeader = "Content-Encoding"
	gzipEncoding          = "gzip"
)

var (
	// ErrBadContentType indicates an incompatible content type was received
	ErrBadContentType = errors.New("bad content type")

	// ErrBadRequest indicates that the request couldn't be decoded
	ErrBadRequest = errors.New("bad request")

	gzipReaderPool = &sync.Pool{
		New: func() interface{} {
			// create a new gzip reader with a bytes reader and array of bytes containing only the gzip header
			r, _ := gzip.NewReader(bytes.NewReader([]byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 0, 0, 0, 255, 255, 1, 0, 0, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0}))
			return r
		},
	}

	gzipWriterPool = &sync.Pool{
		New: func() interface{} {
			return gzip.NewWriter(ioutil.Discard)
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
func NewTraceHandlerV2(receiver func(sapm *splunksapm.PostSpansRequest, err error) error) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		sapm, err := ParseTraceV2Request(req)
		// errors processing the request should return http.StatusBadRequest
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
		}

		// pass the SAPM and error to the receiver function
		err = receiver(sapm, err)

		// handle errors from the receiver function
		if err != nil {
			// write a 500 error and return if the error isn't ErrBadRequest
			if err == ErrBadRequest {
				rw.WriteHeader(http.StatusBadRequest)
			} else {
				// return a 500 when an unknown error occurs in receiver
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		// respBytes are bytes to write to the http.Response
		var respBytes []byte

		// build the response message
		respBytes, err = proto.Marshal(&splunksapm.PostSpansResponse{})
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		rw.Header().Set(contentTypeHeader, xprotobuf)

		// write the response if client does not accept gzip encoding
		if req.Header.Get(acceptEncodingHeader) != gzipEncoding {
			// write the response bytes
			rw.Write(respBytes)
			return
		}

		// gzip the response

		// get the gzip writer
		writer := gzipWriterPool.Get().(*gzip.Writer)
		defer gzipWriterPool.Put(writer)

		var gzipBuffer bytes.Buffer

		// reset the writer with the gzip buffer
		writer.Reset(&gzipBuffer)

		// gzip the responseBytes
		_, err = writer.Write(respBytes)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		// flush gzip writer
		err = writer.Flush()
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		// write the successfully gzipped payload
		rw.Header().Set(contentEncodingHeader, gzipEncoding)
		rw.Write(gzipBuffer.Bytes())
		return
	}
}
