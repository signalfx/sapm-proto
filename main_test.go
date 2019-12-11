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

package main

import (
	"log"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/jaegertracing/jaeger/model"

	splunk_sapm "github.com/signalfx/sapm-proto/gen"
)

func TestCreateRequest(t *testing.T) {
	// Just a test to ensure generated .pb.go files compile and can be marshaled.
	// (This is simply to verify that code generation worked, not a comprehensive test).
	m := &splunk_sapm.PostSpansRequest{
		Batches: []*model.Batch{
			{
				Process: &model.Process{
					ServiceName: "test_service",
				},
			},
		},
	}
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("Cannot encode: %v", err.Error())
	}
	log.Printf("encoded %v bytes\n", len(b))

	m2 := &splunk_sapm.PostSpansRequest{}
	err = proto.Unmarshal(b, m2)
	if err != nil {
		t.Fatalf("Cannot decode: %v", err.Error())
	}
}
