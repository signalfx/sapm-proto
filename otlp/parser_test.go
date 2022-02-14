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
	"encoding/binary"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jaegertracing/jaeger/model"
	splunksapm "github.com/signalfx/sapm-proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.6.1"
	otlpcoltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	otlpcommon "go.opentelemetry.io/proto/otlp/common/v1"
	otlpresource "go.opentelemetry.io/proto/otlp/resource/v1"
	otlptrace "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// Use timespamp with microsecond granularity to work well with jaeger thrift translation
var (
	testSpanStartTime      = time.Date(2020, 2, 11, 20, 26, 12, 321000, time.UTC)
	testSpanStartTimestamp = testSpanStartTime.UnixNano()
	testSpanEventTime      = time.Date(2020, 2, 11, 20, 26, 13, 123000, time.UTC)
	testSpanEventTimestamp = testSpanEventTime.UnixNano()
	testSpanEndTime        = time.Date(2020, 2, 11, 20, 26, 13, 789000, time.UTC)
	testSpanEndTimestamp   = testSpanEndTime.UnixNano()
)

func TestParseRequest(t *testing.T) {
	otlp := &otlpcoltrace.ExportTraceServiceRequest{
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
	expected := &splunksapm.PostSpansRequest{
		Batches: []*model.Batch{
			{
				Process: generateProtoProcess(),
				Spans: []*model.Span{
					generateProtoSpan(),
				},
			},
		},
	}
	reqBody, err := proto.Marshal(otlp)
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
	assert.EqualValues(t, expected, sapm)
}

func generateOtlpResource() *otlpresource.Resource {
	return &otlpresource.Resource{
		Attributes: []*otlpcommon.KeyValue{
			{
				Key:   conventions.AttributeServiceName,
				Value: &otlpcommon.AnyValue{Value: &otlpcommon.AnyValue_StringValue{StringValue: "service"}},
			},
			{
				Key:   "int-attr",
				Value: &otlpcommon.AnyValue{Value: &otlpcommon.AnyValue_IntValue{IntValue: 123}},
			},
		},
	}
}

func generateOtlpSpan() *otlptrace.Span {
	span := &otlptrace.Span{}
	span.SpanId = []byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8}
	span.TraceId = []byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80}
	span.Name = "operationA"
	span.StartTimeUnixNano = uint64(testSpanStartTimestamp)
	span.EndTimeUnixNano = uint64(testSpanEndTimestamp)
	span.Kind = otlptrace.Span_SPAN_KIND_CLIENT
	span.Events = []*otlptrace.Span_Event{
		{
			TimeUnixNano: uint64(testSpanEventTimestamp),
			Name:         "event-with-attr",
			Attributes: []*otlpcommon.KeyValue{
				{
					Key:   "span-event-attr",
					Value: &otlpcommon.AnyValue{Value: &otlpcommon.AnyValue_StringValue{StringValue: "span-event-attr-val"}},
				},
			},
		},
		{
			TimeUnixNano: uint64(testSpanEventTimestamp),
			Attributes: []*otlpcommon.KeyValue{
				{
					Key:   "attr-int",
					Value: &otlpcommon.AnyValue{Value: &otlpcommon.AnyValue_IntValue{IntValue: 123}},
				},
			},
		},
	}
	span.Status = &otlptrace.Status{
		Code:    otlptrace.Status_STATUS_CODE_ERROR,
		Message: "status-cancelled",
	}
	return span
}

func generateProtoProcess() *model.Process {
	return &model.Process{
		ServiceName: "service",
		Tags: []model.KeyValue{
			{
				Key:    "int-attr",
				VType:  model.ValueType_INT64,
				VInt64: 123,
			},
		},
	}
}

func generateProtoSpan() *model.Span {
	return &model.Span{
		TraceID: model.NewTraceID(
			binary.BigEndian.Uint64([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}),
			binary.BigEndian.Uint64([]byte{0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80}),
		),
		SpanID:        model.NewSpanID(binary.BigEndian.Uint64([]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})),
		OperationName: "operationA",
		StartTime:     testSpanStartTime,
		Duration:      testSpanEndTime.Sub(testSpanStartTime),
		Logs: []model.Log{
			{
				Timestamp: testSpanEventTime,
				Fields: []model.KeyValue{
					{
						Key:   "message",
						VType: model.ValueType_STRING,
						VStr:  "event-with-attr",
					},
					{
						Key:   "span-event-attr",
						VType: model.ValueType_STRING,
						VStr:  "span-event-attr-val",
					},
				},
			},
			{
				Timestamp: testSpanEventTime,
				Fields: []model.KeyValue{
					{
						Key:    "attr-int",
						VType:  model.ValueType_INT64,
						VInt64: 123,
					},
				},
			},
		},
		Tags: []model.KeyValue{
			{
				Key:   "span.kind",
				VType: model.ValueType_STRING,
				VStr:  "client",
			},
			{
				Key:   conventions.OtelStatusCode,
				VType: model.ValueType_STRING,
				VStr:  "ERROR",
			},
			{
				Key:   "error",
				VBool: true,
				VType: model.ValueType_BOOL,
			},
			{
				Key:   conventions.OtelStatusDescription,
				VType: model.ValueType_STRING,
				VStr:  "status-cancelled",
			},
		},
	}
}
