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

package otlp

import (
	"net/http"

	splunksapm "github.com/signalfx/sapm-proto/gen"
	otlpcoltrace "github.com/signalfx/sapm-proto/gen/otlp/collector/trace/v1"
	"github.com/signalfx/sapm-proto/sapmprotocol"
)

// ParseRequest parses from the request (unzip if needed) an OTLP protobuf,
// and converts it to SAPM.
func ParseRequest(req *http.Request) (*splunksapm.PostSpansRequest, error) {
	otlp := otlpcoltrace.ExportTraceServiceRequest{}
	if err := sapmprotocol.ParseSapmRequest(req, &otlp); err != nil {
		return nil, err
	}
	return otlpToSAPM(otlp)
}
