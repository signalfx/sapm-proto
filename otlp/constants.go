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

// Some of the keys used to represent OTLP constructs as tags or annotations in Jaeger.
const (
	attributeServiceName = "service.name"

	tagMessage        = "message"
	tagSpanKind       = "span.kind"
	tagStatusCode     = "status.code"
	tagStatusMsg      = "status.message"
	tagError          = "error"
	tagHTTPStatusCode = "http.status_code"

	tagW3CTraceState          = "w3c.tracestate"
	tagInstrumentationName    = "otel.library.name"
	tagInstrumentationVersion = "otel.library.version"
)

// Constants used for signifying batch-level attribute values where not supplied by OTLP data but required
// by other protocols.
const (
	resourceNotSet        = "OTLPResourceNotSet"
	resourceNoServiceName = "OTLPResourceNoServiceName"
)

// OpenTracingSpanKind are possible values for tagSpanKind and match the OpenTracing
// conventions: https://github.com/opentracing/specification/blob/master/semantic_conventions.md
const (
	openTracingSpanKindClient   = "client"
	openTracingSpanKindServer   = "server"
	openTracingSpanKindConsumer = "consumer"
	openTracingSpanKindProducer = "producer"
	openTracingSpanKindInternal = "internal"
)
