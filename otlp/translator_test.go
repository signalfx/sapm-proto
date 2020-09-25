// Copyright 2020 Splunk, Inc.
// Copyright The OpenTelemetry Authors
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
	"encoding/binary"
	"testing"
	"time"

	"github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	splunksapm "github.com/signalfx/sapm-proto/gen"
	otlpcoltrace "github.com/signalfx/sapm-proto/gen/otlp/collector/trace/v1"
	otlpcommon "github.com/signalfx/sapm-proto/gen/otlp/common/v1"
	otlpresource "github.com/signalfx/sapm-proto/gen/otlp/resource/v1"
	otlptrace "github.com/signalfx/sapm-proto/gen/otlp/trace/v1"
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

func TestGetTagFromStatusCode(t *testing.T) {
	tests := []struct {
		name string
		code otlptrace.Status_StatusCode
		tag  model.KeyValue
	}{
		{
			name: "ok",
			code: otlptrace.Status_STATUS_CODE_OK,
			tag: model.KeyValue{
				Key:    tagStatusCode,
				VInt64: int64(otlptrace.Status_STATUS_CODE_OK),
				VType:  model.ValueType_INT64,
			},
		},

		{
			name: "unknown",
			code: otlptrace.Status_STATUS_CODE_UNKNOWN_ERROR,
			tag: model.KeyValue{
				Key:    tagStatusCode,
				VInt64: int64(otlptrace.Status_STATUS_CODE_UNKNOWN_ERROR),
				VType:  model.ValueType_INT64,
			},
		},

		{
			name: "not-found",
			code: otlptrace.Status_STATUS_CODE_NOT_FOUND,
			tag: model.KeyValue{
				Key:    tagStatusCode,
				VInt64: int64(otlptrace.Status_STATUS_CODE_NOT_FOUND),
				VType:  model.ValueType_INT64,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := getTagFromStatusCode(test.code)
			assert.True(t, ok)
			assert.EqualValues(t, test.tag, got)
		})
	}
}

func TestGetErrorTagFromStatusCode(t *testing.T) {
	errTag := model.KeyValue{
		Key:   tagError,
		VBool: true,
		VType: model.ValueType_BOOL,
	}

	_, ok := getErrorTagFromStatusCode(otlptrace.Status_STATUS_CODE_OK)
	assert.False(t, ok)

	got, ok := getErrorTagFromStatusCode(otlptrace.Status_STATUS_CODE_UNKNOWN_ERROR)
	assert.True(t, ok)
	assert.EqualValues(t, errTag, got)

	got, ok = getErrorTagFromStatusCode(otlptrace.Status_STATUS_CODE_NOT_FOUND)
	assert.True(t, ok)
	assert.EqualValues(t, errTag, got)
}

func TestGetTagFromStatusMsg(t *testing.T) {
	got, ok := getTagFromStatusMsg("")
	assert.False(t, ok)

	got, ok = getTagFromStatusMsg("test-error")
	assert.True(t, ok)
	assert.EqualValues(t, model.KeyValue{
		Key:   tagStatusMsg,
		VStr:  "test-error",
		VType: model.ValueType_STRING,
	}, got)
}

func TestUnixNanoToTime(t *testing.T) {
	tt := unixNanoToTime(0)
	assert.True(t, tt.IsZero())
}

func TestGetTagFromSpanKind(t *testing.T) {
	tests := []struct {
		name string
		kind otlptrace.Span_SpanKind
		tag  model.KeyValue
		ok   bool
	}{
		{
			name: "unspecified",
			kind: otlptrace.Span_SPAN_KIND_UNSPECIFIED,
			tag:  model.KeyValue{},
			ok:   false,
		},

		{
			name: "client",
			kind: otlptrace.Span_SPAN_KIND_CLIENT,
			tag: model.KeyValue{
				Key:   tagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  openTracingSpanKindClient,
			},
			ok: true,
		},

		{
			name: "server",
			kind: otlptrace.Span_SPAN_KIND_SERVER,
			tag: model.KeyValue{
				Key:   tagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  openTracingSpanKindServer,
			},
			ok: true,
		},

		{
			name: "producer",
			kind: otlptrace.Span_SPAN_KIND_PRODUCER,
			tag: model.KeyValue{
				Key:   tagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  openTracingSpanKindProducer,
			},
			ok: true,
		},

		{
			name: "consumer",
			kind: otlptrace.Span_SPAN_KIND_CONSUMER,
			tag: model.KeyValue{
				Key:   tagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  openTracingSpanKindConsumer,
			},
			ok: true,
		},

		{
			name: "internal",
			kind: otlptrace.Span_SPAN_KIND_INTERNAL,
			tag: model.KeyValue{
				Key:   tagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  openTracingSpanKindInternal,
			},
			ok: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := getTagFromSpanKind(test.kind)
			assert.Equal(t, test.ok, ok)
			assert.EqualValues(t, test.tag, got)
		})
	}
}

func TestAttributesToJaegerProtoTags(t *testing.T) {
	attributes := []*otlpcommon.KeyValue{
		{
			Key: "bool-val",
			Value: &otlpcommon.AnyValue{
				Value: &otlpcommon.AnyValue_BoolValue{
					BoolValue: true,
				},
			},
		},
		{
			Key: "int-val",
			Value: &otlpcommon.AnyValue{
				Value: &otlpcommon.AnyValue_IntValue{
					IntValue: 123,
				},
			},
		},
		{
			Key: "string-val",
			Value: &otlpcommon.AnyValue{
				Value: &otlpcommon.AnyValue_StringValue{
					StringValue: "abc",
				},
			},
		},
		{
			Key: "double-val",
			Value: &otlpcommon.AnyValue{
				Value: &otlpcommon.AnyValue_DoubleValue{
					DoubleValue: 1.23,
				},
			},
		},
		{
			Key: "map-val",
			Value: &otlpcommon.AnyValue{
				Value: &otlpcommon.AnyValue_KvlistValue{
					KvlistValue: &otlpcommon.KeyValueList{
						Values: []*otlpcommon.KeyValue{
							{
								Key: "key-map-val",
								Value: &otlpcommon.AnyValue{
									Value: &otlpcommon.AnyValue_IntValue{
										IntValue: 123,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Key: "array-val",
			Value: &otlpcommon.AnyValue{
				Value: &otlpcommon.AnyValue_ArrayValue{
					ArrayValue: &otlpcommon.ArrayValue{
						Values: []*otlpcommon.AnyValue{
							{
								Value: &otlpcommon.AnyValue_IntValue{
									IntValue: 123,
								},
							},
						},
					},
				},
			},
		},
		{
			Key: attributeServiceName,
			Value: &otlpcommon.AnyValue{
				Value: &otlpcommon.AnyValue_StringValue{
					StringValue: "service-name",
				},
			},
		},
	}

	expected := []model.KeyValue{
		{
			Key:   "bool-val",
			VType: model.ValueType_BOOL,
			VBool: true,
		},
		{
			Key:    "int-val",
			VType:  model.ValueType_INT64,
			VInt64: 123,
		},
		{
			Key:   "string-val",
			VType: model.ValueType_STRING,
			VStr:  "abc",
		},
		{
			Key:      "double-val",
			VType:    model.ValueType_FLOAT64,
			VFloat64: 1.23,
		},
		{
			Key:   "map-val",
			VType: model.ValueType_STRING,
			VStr:  `{"key-map-val":123}`,
		},
		{
			Key:   "array-val",
			VType: model.ValueType_STRING,
			VStr:  "[123]",
		},
		{
			Key:   attributeServiceName,
			VType: model.ValueType_STRING,
			VStr:  "service-name",
		},
	}

	gotTags := appendTagsFromAttributes(make([]model.KeyValue, 0, len(expected)), attributes)
	require.EqualValues(t, expected, gotTags)

	// The last item in expected ("service-name") must be skipped in resource tags translation
	gotResourceTags, getServiceName := appendTagsFromResourceAttributes(attributes)
	require.EqualValues(t, expected[:len(expected)-1], gotResourceTags)
	require.EqualValues(t, "service-name", getServiceName)
}

func TestInternalTracesToJaegerProto(t *testing.T) {
	tests := []struct {
		name   string
		td     otlpcoltrace.ExportTraceServiceRequest
		jb     splunksapm.PostSpansRequest
		hasErr bool
	}{
		{
			name: "empty",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans(nil),
			},
		},

		{
			name: "no-spans",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans{
					{
						Resource: generateOtlpResource(),
					},
					{
						Resource: generateOtlpResource(),
						InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
							nil,
							{},
							{
								Spans: []*otlptrace.Span{
									nil,
								},
							},
						},
					},
					nil,
				},
			},
			jb: splunksapm.PostSpansRequest{
				Batches: []*model.Batch{},
			},
		},

		{
			name: "no-resource-attrs",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans{
					{
						Resource: &otlpresource.Resource{},
						InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
							{
								Spans: []*otlptrace.Span{
									generateOtlpSpan(),
								},
							},
						},
					},
				},
			},
			jb: splunksapm.PostSpansRequest{
				Batches: []*model.Batch{
					{
						Process: &model.Process{
							ServiceName: resourceNoServiceName,
						},
						Spans: []*model.Span{
							generateProtoSpan(),
						},
					},
				},
			},
		},

		{
			name: "one-span-with-trace-state",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans{
					{
						InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
							{
								Spans: []*otlptrace.Span{
									generateOtlpSpanWithTraceState(),
								},
							},
						},
					},
				},
			},
			jb: splunksapm.PostSpansRequest{
				Batches: []*model.Batch{
					{
						Process: &model.Process{
							ServiceName: resourceNotSet,
						},
						Spans: []*model.Span{
							generateProtoSpanWithTraceState(),
						},
					},
				},
			},
		},

		{
			name: "library-info",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans{
					{
						InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
							{
								InstrumentationLibrary: &otlpcommon.InstrumentationLibrary{
									Name:    "io.opentelemetry.test",
									Version: "0.42.0",
								},
								Spans: []*otlptrace.Span{
									generateOtlpSpan(),
								},
							},
						},
					},
				},
			},
			jb: splunksapm.PostSpansRequest{
				Batches: []*model.Batch{
					{
						Process: &model.Process{
							ServiceName: resourceNotSet,
						},
						Spans: []*model.Span{
							generateProtoSpanWithLibraryInfo("io.opentelemetry.test"),
						},
					},
				},
			},
		},

		{
			name: "two-spans-child-parent",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans{
					{
						Resource: generateOtlpResource(),
						InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
							{
								Spans: []*otlptrace.Span{
									generateOtlpSpan(),
									generateOtlpChildSpan(),
								},
							},
						},
					},
				},
			},
			jb: splunksapm.PostSpansRequest{
				Batches: []*model.Batch{
					{
						Process: generateProtoProcess(),
						Spans: []*model.Span{
							generateProtoSpan(),
							generateProtoChildSpanWithErrorTags(),
						},
					},
				},
			},
		},

		{
			name: "two-spans-with-follower",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans{
					{
						Resource: generateOtlpResource(),
						InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
							{
								Spans: []*otlptrace.Span{
									generateOtlpSpan(),
									generateOtlpFollowerSpan(),
								},
							},
						},
					},
				},
			},
			jb: splunksapm.PostSpansRequest{
				Batches: []*model.Batch{
					{
						Process: generateProtoProcess(),
						Spans: []*model.Span{
							generateProtoSpan(),
							generateProtoFollowerSpan(),
						},
					},
				},
			},
		},

		{
			name: "root-span-nil-links-no-tags",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans{
					{
						Resource: generateOtlpResource(),
						InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
							{
								Spans: []*otlptrace.Span{
									{
										TraceId: []byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80},
										SpanId:  []byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8},
										Links: []*otlptrace.Span_Link{
											nil,
										},
									},
								},
							},
						},
					},
				},
			},
			jb: splunksapm.PostSpansRequest{
				Batches: []*model.Batch{
					{
						Process: generateProtoProcess(),
						Spans: []*model.Span{
							{
								TraceID: model.NewTraceID(
									binary.BigEndian.Uint64([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}),
									binary.BigEndian.Uint64([]byte{0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80}),
								),
								SpanID: model.NewSpanID(binary.BigEndian.Uint64([]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})),
							},
						},
					},
				},
			},
		},

		{
			name: "error-trace-id",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans{
					{
						InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
							{
								Spans: []*otlptrace.Span{
									{
										TraceId: []byte{1, 2},
										SpanId:  []byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8},
									},
								},
							},
						},
					},
				},
			},
			hasErr: true,
		},

		{
			name: "error-span-id",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans{
					{
						InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
							{
								Spans: []*otlptrace.Span{
									{
										TraceId: []byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80},
										SpanId:  []byte{1, 2},
									},
								},
							},
						},
					},
				},
			},
			hasErr: true,
		},

		{
			name: "error-parent-span-id",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans{
					{
						InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
							{
								Spans: []*otlptrace.Span{
									{
										TraceId:      []byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80},
										SpanId:       []byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8},
										ParentSpanId: []byte{1, 2},
									},
								},
							},
						},
					},
				},
			},
			hasErr: true,
		},

		{
			name: "error-linked-trace-id",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans{
					{
						InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
							{
								Spans: []*otlptrace.Span{
									{
										TraceId: []byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80},
										SpanId:  []byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8},
										Links: []*otlptrace.Span_Link{
											{
												TraceId: []byte{1, 2},
												SpanId:  []byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			hasErr: true,
		},

		{
			name: "error-linked-span-id",
			td: otlpcoltrace.ExportTraceServiceRequest{
				ResourceSpans: []*otlptrace.ResourceSpans{
					{
						InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
							{
								Spans: []*otlptrace.Span{
									{
										TraceId: []byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80},
										SpanId:  []byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8},
										Links: []*otlptrace.Span_Link{
											{
												TraceId: []byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80},
												SpanId:  []byte{1, 2},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			hasErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			jbs, err := otlpToSAPM(test.td)
			if test.hasErr {
				assert.Error(t, err)
				assert.Nil(t, jbs)
				return
			}
			assert.NoError(t, err)
			assert.EqualValues(t, &test.jb, jbs)
		})
	}
}

func generateProtoChildSpanWithErrorTags() *model.Span {
	span := generateProtoChildSpan()
	span.Tags = append(span.Tags, model.KeyValue{
		Key:    tagStatusCode,
		VType:  model.ValueType_INT64,
		VInt64: int64(otlptrace.Status_STATUS_CODE_NOT_FOUND),
	})
	span.Tags = append(span.Tags, model.KeyValue{
		Key:   tagError,
		VBool: true,
		VType: model.ValueType_BOOL,
	})
	return span
}

func BenchmarkInternalTracesToJaegerProto(b *testing.B) {
	td := otlpcoltrace.ExportTraceServiceRequest{
		ResourceSpans: []*otlptrace.ResourceSpans{
			{
				Resource: generateOtlpResource(),
				InstrumentationLibrarySpans: []*otlptrace.InstrumentationLibrarySpans{
					{
						Spans: []*otlptrace.Span{
							generateOtlpSpan(),
							generateOtlpFollowerSpan(),
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := otlpToSAPM(td)
		assert.NoError(b, err)
	}
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
						Key:   tagMessage,
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
				Key:   tagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  openTracingSpanKindClient,
			},
			{
				Key:    tagStatusCode,
				VType:  model.ValueType_INT64,
				VInt64: int64(otlptrace.Status_STATUS_CODE_CANCELLED),
			},
			{
				Key:   tagError,
				VBool: true,
				VType: model.ValueType_BOOL,
			},
			{
				Key:   tagStatusMsg,
				VType: model.ValueType_STRING,
				VStr:  "status-cancelled",
			},
		},
	}
}

func generateProtoChildSpan() *model.Span {
	traceID := model.NewTraceID(
		binary.BigEndian.Uint64([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}),
		binary.BigEndian.Uint64([]byte{0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80}),
	)
	return &model.Span{
		TraceID:       traceID,
		SpanID:        model.NewSpanID(binary.BigEndian.Uint64([]byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18})),
		OperationName: "operationB",
		StartTime:     testSpanStartTime,
		Duration:      testSpanEndTime.Sub(testSpanStartTime),
		Tags: []model.KeyValue{
			{
				Key:    tagHTTPStatusCode,
				VType:  model.ValueType_INT64,
				VInt64: 404,
			},
			{
				Key:   tagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  openTracingSpanKindServer,
			},
		},
		References: []model.SpanRef{
			{
				TraceID: traceID,
				SpanID:  model.NewSpanID(binary.BigEndian.Uint64([]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})),
				RefType: model.SpanRefType_CHILD_OF,
			},
		},
	}
}

func generateProtoFollowerSpan() *model.Span {
	traceID := model.NewTraceID(
		binary.BigEndian.Uint64([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}),
		binary.BigEndian.Uint64([]byte{0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80}),
	)
	return &model.Span{
		TraceID:       traceID,
		SpanID:        model.NewSpanID(binary.BigEndian.Uint64([]byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18})),
		OperationName: "operationC",
		StartTime:     testSpanEndTime,
		Duration:      time.Millisecond,
		Tags: []model.KeyValue{
			{
				Key:   tagSpanKind,
				VType: model.ValueType_STRING,
				VStr:  openTracingSpanKindConsumer,
			},
			{
				Key:    tagStatusCode,
				VType:  model.ValueType_INT64,
				VInt64: int64(otlptrace.Status_STATUS_CODE_OK),
			},
			{
				Key:   tagStatusMsg,
				VType: model.ValueType_STRING,
				VStr:  "status-ok",
			},
		},
		References: []model.SpanRef{
			{
				TraceID: traceID,
				SpanID:  model.NewSpanID(binary.BigEndian.Uint64([]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})),
				RefType: model.SpanRefType_FOLLOWS_FROM,
			},
		},
	}
}

func generateProtoSpanWithLibraryInfo(libraryName string) *model.Span {
	span := generateProtoSpan()
	span.Tags = append([]model.KeyValue{
		{

			Key:   tagInstrumentationName,
			VType: model.ValueType_STRING,
			VStr:  libraryName,
		},
		{
			Key:   tagInstrumentationVersion,
			VType: model.ValueType_STRING,
			VStr:  "0.42.0",
		},
	}, span.Tags...)

	return span
}

func generateProtoSpanWithTraceState() *model.Span {
	span := generateProtoSpan()
	span.Tags = []model.KeyValue{
		{
			Key:   tagSpanKind,
			VType: model.ValueType_STRING,
			VStr:  openTracingSpanKindClient,
		},
		{
			Key:   tagW3CTraceState,
			VType: model.ValueType_STRING,
			VStr:  "lasterror=f39cd56cc44274fd5abd07ef1164246d10ce2955",
		},
	}
	return span
}

func generateOtlpResource() *otlpresource.Resource {
	return &otlpresource.Resource{
		Attributes: []*otlpcommon.KeyValue{
			{
				Key:   attributeServiceName,
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
		Code:    otlptrace.Status_STATUS_CODE_CANCELLED,
		Message: "status-cancelled",
	}
	return span
}

func generateOtlpChildSpan() *otlptrace.Span {
	span := generateOtlpSpan()
	originalSpanID := span.SpanId
	span.Name = "operationB"
	span.SpanId = []byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18}
	span.ParentSpanId = originalSpanID
	span.Kind = otlptrace.Span_SPAN_KIND_SERVER
	span.Status = &otlptrace.Status{
		Code: otlptrace.Status_STATUS_CODE_NOT_FOUND,
	}
	span.Events = nil
	span.Attributes = []*otlpcommon.KeyValue{
		{
			Key:   tagHTTPStatusCode,
			Value: &otlpcommon.AnyValue{Value: &otlpcommon.AnyValue_IntValue{IntValue: 404}},
		},
	}

	return span
}

func generateOtlpFollowerSpan() *otlptrace.Span {
	span := generateOtlpSpan()
	originalSpanID := span.SpanId
	span.Name = "operationC"
	span.SpanId = []byte{0x1F, 0x1E, 0x1D, 0x1C, 0x1B, 0x1A, 0x19, 0x18}
	span.StartTimeUnixNano = span.EndTimeUnixNano
	span.EndTimeUnixNano = span.EndTimeUnixNano + 1000000
	span.Kind = otlptrace.Span_SPAN_KIND_CONSUMER
	span.Events = []*otlptrace.Span_Event{
		nil,
	}
	span.Status = &otlptrace.Status{
		Code:    otlptrace.Status_STATUS_CODE_OK,
		Message: "status-ok",
	}
	span.Links = []*otlptrace.Span_Link{
		{
			TraceId: span.TraceId,
			SpanId:  originalSpanID,
		},
	}
	return span
}

func generateOtlpSpanWithTraceState() *otlptrace.Span {
	span := generateOtlpSpan()
	span.Status = nil
	span.TraceState = "lasterror=f39cd56cc44274fd5abd07ef1164246d10ce2955"
	span.Attributes = nil
	return span
}
