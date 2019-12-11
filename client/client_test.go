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

package client

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	jaegerpb "github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signalfx/sapm-proto/sapmprotocol"
)

var defaultEndpointOption = WithEndpoint("http://local")

func assertRequestEqualBatch(t *testing.T, r *http.Request, b *jaegerpb.Batch) {
	psr, err := sapmprotocol.ParseTraceV2Request(r)
	assert.NoError(t, err)

	// No super batching happens in current version
	require.Len(t, psr.Batches, 1)

	got := psr.Batches[0]
	if !reflect.DeepEqual(got, b) {
		t.Errorf("got:\n%+v\nwant:\n%+v\n", got, b)
	}
}

func TestDefaults(t *testing.T) {
	c, err := New(defaultEndpointOption)
	require.NoError(t, err)

	hc := c.httpClient

	assert.Equal(t, defaultHTTPTimeout, hc.Timeout)
	assert.Equal(t, defaultNumWorkers, uint(len(c.workers)))
	w := <-c.workers
	assert.Equal(t, defaultMaxRetries, w.maxRetries)
}

func TestClient(t *testing.T) {
	transport := &mockTransport{}
	c, err := New(defaultEndpointOption, WithHTTPClient(newMockHTTPClient(transport)))
	require.NoError(t, err)

	batches := []*jaegerpb.Batch{}

	for i := 0; i < 10; i++ {
		batch := &jaegerpb.Batch{
			Process: &jaegerpb.Process{ServiceName: "test_service_" + strconv.Itoa(i)},
			Spans:   []*jaegerpb.Span{{}},
		}
		batches = append(batches, batch)
	}

	for _, batch := range batches {
		err = c.Export(context.Background(), batch)
		require.NoError(t, err)
	}
	requests := transport.requests()
	assert.Len(t, requests, len(batches))

	for i, want := range batches {
		assertRequestEqualBatch(t, requests[i].r, want)
	}
}

func TestFailure(t *testing.T) {
	transport := &mockTransport{statusCode: 500}
	c, err := New(
		defaultEndpointOption,
		WithHTTPClient(newMockHTTPClient(transport)),
		WithMaxRetries(0),
	)
	require.NoError(t, err)

	batch := &jaegerpb.Batch{
		Process: &jaegerpb.Process{ServiceName: "test_service"},
		Spans:   []*jaegerpb.Span{{}},
	}

	err = c.Export(context.Background(), batch)
	require.NotNil(t, err)
	assert.Equal(t, err.Error(), "error exporting spans. server responded with status 500")

	requests := transport.requests()
	require.Len(t, requests, 1)
	assertRequestEqualBatch(t, requests[0].r, batch)

	transport.reset(200)
	transport.err = errors.New("transport error")

	err = c.Export(context.Background(), batch)
	require.NotNil(t, err)
	assert.Equal(t, err.Error(), "Post http://local: transport error")

	requests = transport.requests()
	require.Len(t, requests, 1)
	assertRequestEqualBatch(t, requests[0].r, batch)
}

func TestRetries(t *testing.T) {
	transport := &mockTransport{statusCode: 500}
	c, err := New(
		defaultEndpointOption,
		WithHTTPClient(newMockHTTPClient(transport)),
		WithMaxRetries(0),
	)
	require.NoError(t, err)

	batch := &jaegerpb.Batch{
		Process: &jaegerpb.Process{ServiceName: "test_service"},
		Spans:   []*jaegerpb.Span{{}},
	}

	err = c.Export(context.Background(), batch)
	require.NotNil(t, err)
	assert.Equal(t, err.Error(), "error exporting spans. server responded with status 500")
	serr := err.(*ErrHTTPSend)
	assert.False(t, serr.Permanent)

	requests := transport.requests()
	require.Len(t, requests, 1)
	assertRequestEqualBatch(t, requests[0].r, batch)
}

func TestBadRequest(t *testing.T) {
	transport := &mockTransport{}

	c, err := New(
		defaultEndpointOption,
		WithHTTPClient(newMockHTTPClient(transport)),
	)
	require.NoError(t, err)

	batch := &jaegerpb.Batch{
		Process: &jaegerpb.Process{ServiceName: "test_service"},
		Spans:   []*jaegerpb.Span{{}},
	}

	for _, code := range []int{400, 401} {
		transport.reset(code)
		err = c.Export(context.Background(), batch)
		require.NotNil(t, err)
		require.IsType(t, &ErrHTTPSend{}, err)
		serr := err.(*ErrHTTPSend)
		assert.True(t, serr.Permanent)
		assert.Equal(t, err.Error(), "dropping request: server responded with: "+strconv.Itoa(code))

		requests := transport.requests()
		require.Len(t, requests, 1)
		assertRequestEqualBatch(t, requests[0].r, batch)

	}
}

func TestWorkers(t *testing.T) {
	workerDelay := time.Millisecond * 200
	transport := &mockTransport{delay: workerDelay}

	// tell client to use a single worker
	// add delay to transport
	c, err := New(
		defaultEndpointOption,
		WithHTTPClient(newMockHTTPClient(transport)),
		WithWorkers(1),
	)
	require.NoError(t, err)

	numRequests := 4
	wg := sync.WaitGroup{}
	wg.Add(numRequests)

	batches := make([]*jaegerpb.Batch, numRequests)
	for i := 0; i < numRequests; i++ {
		batches[i] = &jaegerpb.Batch{
			Process: &jaegerpb.Process{ServiceName: "test_service"},
			Spans:   []*jaegerpb.Span{{}},
		}
	}

	then := time.Now()
	for _, batch := range batches {
		go func(b *jaegerpb.Batch) {
			err := c.Export(context.Background(), b)
			require.NoError(t, err)
			wg.Done()
		}(batch)
	}
	wg.Wait()

	requests := transport.requests()
	require.Len(t, requests, 4)

	// ensure each batch took at least (workerDelay * batch's queue position) to complete
	for i, b := range batches {
		r := requests[i]
		delay := r.receivedAt.Sub(then)
		assert.GreaterOrEqual(t, int(delay), int(workerDelay*time.Duration(i)))
		assertRequestEqualBatch(t, r.r, b)
	}

	// reset transport to remove delay and empty recorded requests
	transport.reset(200)
	c, err = New(
		defaultEndpointOption,
		WithHTTPClient(newMockHTTPClient(transport)),
		WithWorkers(4),
	)
	require.NoError(t, err)

	wg = sync.WaitGroup{}
	wg.Add(numRequests)

	then = time.Now()
	for _, batch := range batches {
		go func(b *jaegerpb.Batch) {
			err := c.Export(context.Background(), b)
			require.NoError(t, err)
			wg.Done()
		}(batch)
	}
	wg.Wait()

	requests = transport.requests()
	require.Len(t, requests, 4)

	// ensure all four requests completed within 100ms
	hundredMs := time.Millisecond * time.Duration(100)
	for i, b := range batches {
		r := requests[i]
		delay := r.receivedAt.Sub(then)
		assert.LessOrEqual(t, int(delay), int(hundredMs))
		assertRequestEqualBatch(t, r.r, b)
	}
}

func TestClientStop(t *testing.T) {
	transport := &mockTransport{
		statusCode: 429,
		headers: map[string]string{
			"Retry-After": "1",
		},
	}
	c, err := New(
		defaultEndpointOption,
		WithHTTPClient(newMockHTTPClient(transport)),
		WithMaxRetries(1),
	)
	require.NoError(t, err)

	// should take more than 1 second
	batch := &jaegerpb.Batch{
		Process: &jaegerpb.Process{ServiceName: "test_service"},
		Spans:   []*jaegerpb.Span{{}},
	}
	then := time.Now()
	err = c.Export(context.Background(), batch)
	assert.NotNil(t, err)
	assert.GreaterOrEqual(t, int(time.Since(then).Seconds()), 1)

	// if client is stopped, it should ignore rate-limiting retry and return immediately
	then = time.Now()
	go func() {
		err = c.Export(context.Background(), batch)
		assert.NotNil(t, err)
	}()
	c.Stop()
	assert.Less(t, int(time.Since(then).Milliseconds()), 100)
}
