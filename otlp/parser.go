// Copyright 2020 Splunk, Inc.
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

package otlp

import (
	"net/http"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"go.opentelemetry.io/collector/model/otlpgrpc"

	splunksapm "github.com/signalfx/sapm-proto/gen"
	"github.com/signalfx/sapm-proto/sapmprotocol"
)

// otlpRequestUnmarshaler helper to implement proto.Unmarshaler, since the TracesRequest does not
type otlpRequestUnmarshaler struct {
	otlpgrpc.TracesRequest
}

func (oru *otlpRequestUnmarshaler) Unmarshal(buf []byte) error {
	var err error
	oru.TracesRequest, err = otlpgrpc.UnmarshalTracesRequest(buf)
	return err
}

// ParseRequest parses from the request (unzip if needed) from OTLP protobuf,
// and converts it to SAPM.
func ParseRequest(req *http.Request) (*splunksapm.PostSpansRequest, error) {
	otlpUnmarshaler := otlpRequestUnmarshaler{}
	if err := sapmprotocol.ParseSapmRequest(req, &otlpUnmarshaler); err != nil {
		return nil, err
	}

	batches, err := jaeger.InternalTracesToJaegerProto(otlpUnmarshaler.TracesRequest.Traces())
	if err != nil {
		return nil, err
	}
	return &splunksapm.PostSpansRequest{Batches: batches}, nil
}
