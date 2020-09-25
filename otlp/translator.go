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
	"fmt"
	"time"

	"github.com/jaegertracing/jaeger/model"

	splunksapm "github.com/signalfx/sapm-proto/gen"
	otlpcoltrace "github.com/signalfx/sapm-proto/gen/otlp/collector/trace/v1"
	otlpcommon "github.com/signalfx/sapm-proto/gen/otlp/common/v1"
	otlpresource "github.com/signalfx/sapm-proto/gen/otlp/resource/v1"
	otlptrace "github.com/signalfx/sapm-proto/gen/otlp/trace/v1"
)

// otlpToSAPM translates otlp trace proto into the SAPM Proto.
func otlpToSAPM(td otlpcoltrace.ExportTraceServiceRequest) (*splunksapm.PostSpansRequest, error) {
	sapm := &splunksapm.PostSpansRequest{}
	if len(td.ResourceSpans) == 0 {
		return sapm, nil
	}

	sapm.Batches = make([]*model.Batch, 0, len(td.ResourceSpans))

	for _, rs := range td.ResourceSpans {
		batch, err := resourceSpansToJaegerProto(rs)
		if err != nil {
			return nil, err
		}
		// Ignore nil batches. Even if no error (spans may not be present).
		if batch != nil {
			sapm.Batches = append(sapm.Batches, batch)
		}
	}

	return sapm, nil
}

func resourceSpansToJaegerProto(rs *otlptrace.ResourceSpans) (*model.Batch, error) {
	// If no spans do not propagate just the Process.
	if rs == nil || len(rs.InstrumentationLibrarySpans) == 0 {
		return nil, nil
	}

	// Approximate the number of the spans.
	jSpans := make([]*model.Span, 0, spanCount(rs.InstrumentationLibrarySpans))
	for _, ils := range rs.InstrumentationLibrarySpans {
		if ils == nil {
			continue // ignore nil InstrumentationLibrarySpans
		}

		spans := ils.Spans
		for _, span := range spans {
			if span == nil {
				continue // ignore nil Span
			}

			jSpan, err := spanToJaegerProto(span, ils.InstrumentationLibrary)
			if err != nil {
				return nil, err
			}

			jSpans = append(jSpans, jSpan)
		}
	}

	// If failed to convert all the spans, no need to return only the process.
	if len(jSpans) == 0 {
		return nil, nil
	}

	return &model.Batch{
		Process: resourceToJaegerProtoProcess(rs.Resource),
		Spans:   jSpans,
	}, nil
}

func spanCount(ilss []*otlptrace.InstrumentationLibrarySpans) int {
	sCount := 0
	for _, ils := range ilss {
		if ils == nil {
			continue
		}
		sCount += len(ils.Spans)
	}
	return sCount
}

func resourceToJaegerProtoProcess(resource *otlpresource.Resource) *model.Process {
	process := model.Process{}
	if resource == nil {
		process.ServiceName = resourceNotSet
		return &process
	}

	var serviceName string
	process.Tags, serviceName = appendTagsFromResourceAttributes(resource.Attributes)
	process.ServiceName = serviceName
	return &process

}

func appendTagsFromResourceAttributes(attrs []*otlpcommon.KeyValue) ([]model.KeyValue, string) {
	serviceName := resourceNoServiceName
	if len(attrs) == 0 {
		return nil, serviceName
	}

	tags := make([]model.KeyValue, 0, len(attrs))
	for _, kv := range attrs {
		if kv.GetKey() == attributeServiceName {
			serviceName = kv.GetValue().GetStringValue()
			continue
		}
		tags = append(tags, attributeToJaegerProtoTag(kv.GetKey(), kv.GetValue()))
	}
	return tags, serviceName
}

func appendTagsFromAttributes(dest []model.KeyValue, attrs []*otlpcommon.KeyValue) []model.KeyValue {
	if len(attrs) == 0 {
		return dest
	}
	for _, kv := range attrs {
		dest = append(dest, attributeToJaegerProtoTag(kv.Key, kv.Value))
	}
	return dest
}

func attributeToJaegerProtoTag(key string, attr *otlpcommon.AnyValue) model.KeyValue {
	tag := model.KeyValue{Key: key}
	switch v := attr.GetValue().(type) {
	case *otlpcommon.AnyValue_StringValue:
		// Jaeger-to-Internal maps binary tags to string attributes and encodes them as
		// base64 strings. Blindingly attempting to decode base64 seems too much.
		tag.VType = model.ValueType_STRING
		tag.VStr = v.StringValue
	case *otlpcommon.AnyValue_BoolValue:
		tag.VType = model.ValueType_BOOL
		tag.VBool = v.BoolValue
	case *otlpcommon.AnyValue_IntValue:
		tag.VType = model.ValueType_INT64
		tag.VInt64 = v.IntValue
	case *otlpcommon.AnyValue_DoubleValue:
		tag.VType = model.ValueType_FLOAT64
		tag.VFloat64 = v.DoubleValue
	case *otlpcommon.AnyValue_KvlistValue:
		tag.VType = model.ValueType_STRING
		tag.VStr = keyValueListToJSONString(v.KvlistValue)
	case *otlpcommon.AnyValue_ArrayValue:
		tag.VType = model.ValueType_STRING
		tag.VStr = arrayValueToJSONString(v.ArrayValue)
	}
	return tag
}

func spanToJaegerProto(span *otlptrace.Span, instLibrary *otlpcommon.InstrumentationLibrary) (*model.Span, error) {
	traceID, err := model.TraceIDFromBytes(span.TraceId)
	if err != nil {
		return nil, fmt.Errorf("incorrect trace ID: %w", err)
	}

	spanID, err := model.SpanIDFromBytes(span.SpanId)
	if err != nil {
		return nil, fmt.Errorf("incorrect span ID: %w", err)
	}

	jReferences, err := makeJaegerProtoReferences(span.Links, span.ParentSpanId, traceID)
	if err != nil {
		return nil, fmt.Errorf("error converting span links to Jaeger references: %w", err)
	}

	startTime := unixNanoToTime(span.StartTimeUnixNano)

	return &model.Span{
		TraceID:       traceID,
		SpanID:        spanID,
		OperationName: span.Name,
		References:    jReferences,
		StartTime:     startTime,
		Duration:      unixNanoToTime(span.EndTimeUnixNano).Sub(startTime),
		Tags:          getJaegerProtoSpanTags(span, instLibrary),
		Logs:          spanEventsToJaegerProtoLogs(span.Events),
	}, nil
}

func getJaegerProtoSpanTags(span *otlptrace.Span, instLibrary *otlpcommon.InstrumentationLibrary) []model.KeyValue {
	var spanKindTag, statusCodeTag, errorTag, statusMsgTag model.KeyValue
	var spanKindTagFound, statusCodeTagFound, errorTagFound, statusMsgTagFound bool

	libraryTags, libraryTagsFound := getTagsFromInstrumentationLibrary(instLibrary)

	tagsCount := len(span.Attributes) + len(libraryTags)

	spanKindTag, spanKindTagFound = getTagFromSpanKind(span.Kind)
	if spanKindTagFound {
		tagsCount++
	}
	status := span.Status
	if status != nil {
		statusCodeTag, statusCodeTagFound = getTagFromStatusCode(status.Code)
		if statusCodeTagFound {
			tagsCount++
		}

		errorTag, errorTagFound = getErrorTagFromStatusCode(status.Code)
		if errorTagFound {
			tagsCount++
		}

		statusMsgTag, statusMsgTagFound = getTagFromStatusMsg(status.Message)
		if statusMsgTagFound {
			tagsCount++
		}
	}

	traceStateTags, traceStateTagsFound := getTagFromTraceState(span.TraceState)
	if traceStateTagsFound {
		tagsCount++
	}

	if tagsCount == 0 {
		return nil
	}

	tags := make([]model.KeyValue, 0, tagsCount)
	if libraryTagsFound {
		tags = append(tags, libraryTags...)
	}
	tags = appendTagsFromAttributes(tags, span.Attributes)
	if spanKindTagFound {
		tags = append(tags, spanKindTag)
	}
	if statusCodeTagFound {
		tags = append(tags, statusCodeTag)
	}
	if errorTagFound {
		tags = append(tags, errorTag)
	}
	if statusMsgTagFound {
		tags = append(tags, statusMsgTag)
	}
	if traceStateTagsFound {
		tags = append(tags, traceStateTags)
	}
	return tags
}

// makeJaegerProtoReferences constructs jaeger span references based on parent span ID and span links
func makeJaegerProtoReferences(links []*otlptrace.Span_Link, parentSpanID []byte, traceID model.TraceID) ([]model.SpanRef, error) {
	parentSpanIDSet := len(parentSpanID) != 0
	if !parentSpanIDSet && len(links) == 0 {
		return nil, nil
	}

	refsCount := len(links)
	if parentSpanIDSet {
		refsCount++
	}

	refs := make([]model.SpanRef, 0, refsCount)

	// Put parent span ID at the first place because usually backends look for it
	// as the first CHILD_OF item in the model.SpanRef slice.
	if parentSpanIDSet {
		jParentSpanID, err := model.SpanIDFromBytes(parentSpanID)
		if err != nil {
			return nil, fmt.Errorf("incorrect parent span ID: %w", err)
		}

		refs = append(refs, model.SpanRef{
			TraceID: traceID,
			SpanID:  jParentSpanID,
			RefType: model.SpanRefType_CHILD_OF,
		})
	}

	for _, link := range links {
		if link == nil {
			continue
		}

		traceID, err := model.TraceIDFromBytes(link.TraceId)
		if err != nil {
			return nil, fmt.Errorf("incorrect linked trace ID: %w", err)
		}

		spanID, err := model.SpanIDFromBytes(link.SpanId)
		if err != nil {
			return nil, fmt.Errorf("incorrect linked span ID: %w", err)
		}

		refs = append(refs, model.SpanRef{
			TraceID: traceID,
			SpanID:  spanID,

			// Since Jaeger RefType is not captured in internal data,
			// use SpanRefType_FOLLOWS_FROM by default.
			// SpanRefType_CHILD_OF supposed to be set only from parentSpanID.
			RefType: model.SpanRefType_FOLLOWS_FROM,
		})
	}

	if len(refs) == 0 {
		return nil, nil
	}

	return refs, nil
}

func spanEventsToJaegerProtoLogs(events []*otlptrace.Span_Event) []model.Log {
	if len(events) == 0 {
		return nil
	}

	logs := make([]model.Log, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}

		hasName := event.Name != ""
		fieldsCount := len(event.Attributes)
		if hasName {
			fieldsCount++
		}

		fields := make([]model.KeyValue, 0, fieldsCount)
		if hasName {
			fields = append(fields, model.KeyValue{
				Key:   tagMessage,
				VType: model.ValueType_STRING,
				VStr:  event.Name,
			})
		}
		fields = appendTagsFromAttributes(fields, event.Attributes)
		logs = append(logs, model.Log{
			Timestamp: unixNanoToTime(event.TimeUnixNano),
			Fields:    fields,
		})
	}

	// If no logs, then return a nil slice to not reference memory unnecessary.
	if len(logs) == 0 {
		logs = nil
	}
	return logs
}

func getTagFromSpanKind(spanKind otlptrace.Span_SpanKind) (model.KeyValue, bool) {
	var tagStr string
	switch spanKind {
	case otlptrace.Span_SPAN_KIND_CLIENT:
		tagStr = openTracingSpanKindClient
	case otlptrace.Span_SPAN_KIND_SERVER:
		tagStr = openTracingSpanKindServer
	case otlptrace.Span_SPAN_KIND_PRODUCER:
		tagStr = openTracingSpanKindProducer
	case otlptrace.Span_SPAN_KIND_CONSUMER:
		tagStr = openTracingSpanKindConsumer
	case otlptrace.Span_SPAN_KIND_INTERNAL:
		tagStr = openTracingSpanKindInternal
	default:
		return model.KeyValue{}, false
	}

	return model.KeyValue{
		Key:   tagSpanKind,
		VType: model.ValueType_STRING,
		VStr:  tagStr,
	}, true
}

func getTagFromStatusCode(statusCode otlptrace.Status_StatusCode) (model.KeyValue, bool) {
	return model.KeyValue{
		Key:    tagStatusCode,
		VInt64: int64(statusCode),
		VType:  model.ValueType_INT64,
	}, true
}

func getErrorTagFromStatusCode(statusCode otlptrace.Status_StatusCode) (model.KeyValue, bool) {
	if statusCode == otlptrace.Status_STATUS_CODE_OK {
		return model.KeyValue{}, false
	}
	return model.KeyValue{
		Key:   tagError,
		VBool: true,
		VType: model.ValueType_BOOL,
	}, true
}

func getTagFromStatusMsg(statusMsg string) (model.KeyValue, bool) {
	if statusMsg == "" {
		return model.KeyValue{}, false
	}
	return model.KeyValue{
		Key:   tagStatusMsg,
		VStr:  statusMsg,
		VType: model.ValueType_STRING,
	}, true
}

func getTagFromTraceState(traceState string) (model.KeyValue, bool) {
	if traceState != "" {
		// TODO Bring this inline with solution for jaegertracing/jaeger-client-java #702 once available
		return model.KeyValue{
			Key:   tagW3CTraceState,
			VStr:  traceState,
			VType: model.ValueType_STRING,
		}, true
	}
	return model.KeyValue{}, false
}

func getTagsFromInstrumentationLibrary(il *otlpcommon.InstrumentationLibrary) ([]model.KeyValue, bool) {
	var keyValues []model.KeyValue
	if il == nil {
		return keyValues, false
	}
	if il.Name != "" {
		kv := model.KeyValue{
			Key:   tagInstrumentationName,
			VStr:  il.Name,
			VType: model.ValueType_STRING,
		}
		keyValues = append(keyValues, kv)
	}
	if il.Version != "" {
		kv := model.KeyValue{
			Key:   tagInstrumentationVersion,
			VStr:  il.Version,
			VType: model.ValueType_STRING,
		}
		keyValues = append(keyValues, kv)
	}

	return keyValues, true
}

func unixNanoToTime(t uint64) time.Time {
	// 0 is a special case and want to make sure we return a time that IsZero() returns true.
	if t == 0 {
		return time.Time{}
	}
	return time.Unix(0, int64(t)).UTC()
}
