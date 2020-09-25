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
	"testing"

	"github.com/stretchr/testify/assert"

	otlpcommon "github.com/signalfx/sapm-proto/gen/otlp/common/v1"
)

func TestKeyValueListToJSONString(t *testing.T) {
	valueMap := &otlpcommon.KeyValueList{
		Values: []*otlpcommon.KeyValue{
			{
				Key: "strKey",
				Value: &otlpcommon.AnyValue{
					Value: &otlpcommon.AnyValue_StringValue{
						StringValue: "strVal",
					},
				},
			},
			{
				Key: "intKey",
				Value: &otlpcommon.AnyValue{
					Value: &otlpcommon.AnyValue_IntValue{
						IntValue: 7,
					},
				},
			},
			{
				Key: "floatKey",
				Value: &otlpcommon.AnyValue{
					Value: &otlpcommon.AnyValue_DoubleValue{
						DoubleValue: 18.6,
					},
				},
			},
			{
				Key: "boolKey",
				Value: &otlpcommon.AnyValue{
					Value: &otlpcommon.AnyValue_BoolValue{
						BoolValue: false,
					},
				},
			},
			{
				Key: "nullKey",
			},
			{
				Key: "mapKey",
				Value: &otlpcommon.AnyValue{
					Value: &otlpcommon.AnyValue_KvlistValue{
						KvlistValue: &otlpcommon.KeyValueList{
							Values: []*otlpcommon.KeyValue{
								{
									Key: "keyOne",
									Value: &otlpcommon.AnyValue{
										Value: &otlpcommon.AnyValue_StringValue{
											StringValue: "valOne",
										},
									},
								},
								{
									Key: "keyTwo",
									Value: &otlpcommon.AnyValue{
										Value: &otlpcommon.AnyValue_StringValue{
											StringValue: "valTwo",
										},
									},
								},
							},
						},
					},
				},
			},
			{
				Key: "arrKey",
				Value: &otlpcommon.AnyValue{
					Value: &otlpcommon.AnyValue_ArrayValue{
						ArrayValue: &otlpcommon.ArrayValue{
							Values: []*otlpcommon.AnyValue{
								{
									Value: &otlpcommon.AnyValue_StringValue{
										StringValue: "strOne",
									},
								},
								{
									Value: &otlpcommon.AnyValue_StringValue{
										StringValue: "strTwo",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	expected := `{"arrKey":["strOne","strTwo"],"boolKey":false,"floatKey":18.6,"intKey":7,"mapKey":{"keyOne":"valOne","keyTwo":"valTwo"},"nullKey":null,"strKey":"strVal"}`

	strVal := keyValueListToJSONString(valueMap)
	assert.EqualValues(t, expected, strVal)
}

func TestKeyValueListToJSONString_Nil(t *testing.T) {
	strVal := keyValueListToJSONString(nil)
	assert.EqualValues(t, "null", strVal)
}

func TestArrayValueToJSONString(t *testing.T) {
	valueMap := &otlpcommon.ArrayValue{
		Values: []*otlpcommon.AnyValue{
			{
				Value: &otlpcommon.AnyValue_StringValue{
					StringValue: "strVal",
				},
			},
			{
				Value: &otlpcommon.AnyValue_IntValue{
					IntValue: 7,
				},
			},
			{
				Value: &otlpcommon.AnyValue_DoubleValue{
					DoubleValue: 18.6,
				},
			},
			{
				Value: &otlpcommon.AnyValue_BoolValue{
					BoolValue: false,
				},
			},
			{},
			{
				Value: &otlpcommon.AnyValue_KvlistValue{
					KvlistValue: &otlpcommon.KeyValueList{
						Values: []*otlpcommon.KeyValue{
							{
								Key: "keyOne",
								Value: &otlpcommon.AnyValue{
									Value: &otlpcommon.AnyValue_StringValue{
										StringValue: "valOne",
									},
								},
							},
							{
								Key: "keyTwo",
								Value: &otlpcommon.AnyValue{
									Value: &otlpcommon.AnyValue_StringValue{
										StringValue: "valTwo",
									},
								},
							},
						},
					},
				},
			},
			{
				Value: &otlpcommon.AnyValue_ArrayValue{
					ArrayValue: &otlpcommon.ArrayValue{
						Values: []*otlpcommon.AnyValue{
							{
								Value: &otlpcommon.AnyValue_StringValue{
									StringValue: "strOne",
								},
							},
							{
								Value: &otlpcommon.AnyValue_StringValue{
									StringValue: "strTwo",
								},
							},
						},
					},
				},
			},
		},
	}
	expected := `["strVal",7,18.6,false,null,"\u003cInvalid array value\u003e","\u003cInvalid array value\u003e"]`

	strVal := arrayValueToJSONString(valueMap)
	assert.EqualValues(t, expected, strVal)
}

func TestArrayValueToJSONString_Nil(t *testing.T) {
	strVal := arrayValueToJSONString(nil)
	assert.EqualValues(t, "null", strVal)
}
