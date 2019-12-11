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
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/golang/protobuf/proto"

	splunksapm "github.com/signalfx/sapm-proto/gen"
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
	if req.Header.Get(ContentTypeHeaderName) != ContentTypeHeaderValue {
		return nil, ErrBadContentType
	}

	var err error
	var reader io.Reader

	// content encoding SHOULD be gzip
	if req.Header.Get(ContentEncodingHeaderName) == GZipEncodingHeaderValue {
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
		return nil, err
	}

	return sapm, err
}
