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
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	splunksapm "github.com/signalfx/sapm-proto/gen"
	otlpcoltrace "github.com/signalfx/sapm-proto/gen/otlp/collector/trace/v1"
	otlptrace "github.com/signalfx/sapm-proto/gen/otlp/trace/v1"
)

func TestParseRequest(t *testing.T) {
	otlp := otlpcoltrace.ExportTraceServiceRequest{
		ResourceSpans: []*otlptrace.ResourceSpans{
			{
				Resource: generateOtlpResource(),
				InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
					{
						Spans: []*otlptrace.Span{
							generateOtlpSpan(),
						},
					},
				},
			},
		},
	}
	expected := splunksapm.PostSpansRequest{
		Batches: []*model.Batch{
			{
				Process: generateProtoProcess(),
				Spans: []*model.Span{
					generateProtoSpan(),
				},
			},
		},
	}
	reqBody, err := otlp.Marshal()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "http://signalfx.com/v1/traces", bytes.NewReader(reqBody))
	var sapm *splunksapm.PostSpansRequest
	sapm, err = ParseRequest(req)
	// No "Content-Type".
	assert.Error(t, err)
	assert.Nil(t, sapm)

	req = httptest.NewRequest("GET", "http://signalfx.com/v1/traces", bytes.NewReader(reqBody))
	req.Header["Content-Type"] = []string{"application/x-protobuf"}
	sapm, err = ParseRequest(req)
	assert.NoError(t, err)
	assert.EqualValues(t, &expected, sapm)
}
