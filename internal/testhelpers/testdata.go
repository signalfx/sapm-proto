package testhelpers

import (
	"strconv"
	"time"

	"github.com/jaegertracing/jaeger/model"

	splunksapm "github.com/signalfx/sapm-proto/gen"
)

func CreateSapmData(batchSize int) *splunksapm.PostSpansRequest {
	attrs := []string{
		"service.name", "shoppingcart", "host.name", "spool.example.com", "service.id",
		"adb80442-8437-46b5-a637-ce4a158ba9cf",
	}

	batch := &model.Batch{
		Process: &model.Process{ServiceName: "spring"},
		Spans:   []*model.Span{},
	}
	for i := 0; i < batchSize; i++ {
		span := &model.Span{
			TraceID:       model.NewTraceID(uint64(i*5), uint64(i*10)),
			SpanID:        model.NewSpanID(uint64(i)),
			OperationName: "jonatan" + strconv.Itoa(i),
			Duration:      time.Millisecond * time.Duration(i),
			Tags: model.KeyValues{
				{Key: "span.kind", VStr: "client", VType: model.StringType},
			},
			StartTime: time.Now().UTC().Add(time.Second * time.Duration(i)),
		}
		for j := 0; j < 2; j++ {
			span.Tags = append(
				span.Tags,
				model.KeyValue{
					Key:   attrs[(i+j)%len(attrs)],
					VStr:  attrs[(i+j+1)%len(attrs)],
					VType: model.StringType,
				},
			)
		}

		batch.Spans = append(batch.Spans, span)
	}
	return &splunksapm.PostSpansRequest{Batches: []*model.Batch{batch}}
}
