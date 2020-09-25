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
	"encoding/json"

	otlpcommon "github.com/signalfx/sapm-proto/gen/otlp/common/v1"
)

// attributeMapToString converts an OTLP AttributeMap to a standard go map
func attributeMapToString(attrMap *otlpcommon.KeyValueList) string {
	rawMap := attributeMapToMap(attrMap)
	js, _ := json.Marshal(rawMap)
	return string(js)
}

func attributeArrayToString(attrArray *otlpcommon.ArrayValue) string {
	rawSlice := attributeArrayToArray(attrArray)
	js, _ := json.Marshal(rawSlice)
	return string(js)
}

func attributeMapToMap(attrMap *otlpcommon.KeyValueList) map[string]interface{} {
	if attrMap == nil {
		return nil
	}
	rawMap := make(map[string]interface{})
	for _, attr := range attrMap.GetValues() {
		switch v := attr.GetValue().GetValue().(type) {
		case *otlpcommon.AnyValue_StringValue:
			rawMap[attr.Key] = v.StringValue
		case *otlpcommon.AnyValue_BoolValue:
			rawMap[attr.Key] = v.BoolValue
		case *otlpcommon.AnyValue_IntValue:
			rawMap[attr.Key] = v.IntValue
		case *otlpcommon.AnyValue_DoubleValue:
			rawMap[attr.Key] = v.DoubleValue
		case *otlpcommon.AnyValue_KvlistValue:
			rawMap[attr.Key] = attributeMapToMap(v.KvlistValue)
		case *otlpcommon.AnyValue_ArrayValue:
			rawMap[attr.Key] = attributeArrayToArray(v.ArrayValue)
		default:
			rawMap[attr.Key] = nil
		}
	}
	return rawMap
}

func attributeArrayToArray(attrArray *otlpcommon.ArrayValue) []interface{} {
	if attrArray == nil {
		return nil
	}
	rawSlice := make([]interface{}, 0, len(attrArray.Values))
	for _, attr := range attrArray.GetValues() {
		switch v := attr.GetValue().(type) {
		case *otlpcommon.AnyValue_StringValue:
			rawSlice = append(rawSlice, v.StringValue)
		case *otlpcommon.AnyValue_BoolValue:
			rawSlice = append(rawSlice, v.BoolValue)
		case *otlpcommon.AnyValue_IntValue:
			rawSlice = append(rawSlice, v.IntValue)
		case *otlpcommon.AnyValue_DoubleValue:
			rawSlice = append(rawSlice, v.DoubleValue)
		case *otlpcommon.AnyValue_KvlistValue:
			rawSlice = append(rawSlice, "<Invalid array value>")
		case *otlpcommon.AnyValue_ArrayValue:
			rawSlice = append(rawSlice, "<Invalid array value>")
		default:
			rawSlice = append(rawSlice, nil)
		}
	}

	return rawSlice
}
