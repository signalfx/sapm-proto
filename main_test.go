package main

import (
	"log"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/jaegertracing/jaeger/model"

	splunk_sapm "github.com/signalfx/sapm-proto/gen"
)

func TestCreateRequest(t*testing.T) {
	// Just a test to ensure generated .pb.go files compile and can be marshaled.
	// (This is simply to verify that code generation worked, not a comprehensive test).
	m := &splunk_sapm.PostSpansRequest{
		Batches: []*model.Batch{
			{
				Process:&model.Process{
					ServiceName:"test_service",
				},
			},
		},
	}
	b, err := proto.Marshal(m)
	if err!=nil {
		t.Fatalf("Cannot encode: %v", err.Error())
	}
	log.Printf("encoded %v bytes\n", len(b))
}
