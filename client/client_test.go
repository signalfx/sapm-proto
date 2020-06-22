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

func assertRequestEqualBatches(t *testing.T, r *http.Request, b []*jaegerpb.Batch) {
	psr, err := sapmprotocol.ParseTraceV2Request(r)
	assert.NoError(t, err)
	assert.EqualValues(t, b, psr.Batches)
}

func TestDefaults(t *testing.T) {
	c, err := New(defaultEndpointOption)
	require.NoError(t, err)

	hc := c.httpClient

	assert.Equal(t, defaultHTTPTimeout, hc.Timeout)
	assert.Equal(t, defaultNumWorkers, uint(len(c.workers)))
}

func TestClient(t *testing.T) {
	transport := &mockTransport{}
	c, err := New(defaultEndpointOption, WithHTTPClient(newMockHTTPClient(transport)), WithAccessToken("ClientToken"))
	require.NoError(t, err)

	requestsBatches := [][]*jaegerpb.Batch{}

	for i := 0; i < 10; i++ {
		batches := []*jaegerpb.Batch{
			{
				Process: &jaegerpb.Process{ServiceName: "test_service_" + strconv.Itoa(i)},
				Spans:   []*jaegerpb.Span{{}},
			},
		}
		requestsBatches = append(requestsBatches, batches)
	}

	for _, batches := range requestsBatches {
		err := c.Export(context.Background(), batches)
		require.Nil(t, err)
	}

	requests := transport.requests()
	assert.Len(t, requests, len(requestsBatches))

	for i, want := range requestsBatches {
		assertRequestEqualBatches(t, requests[i].r, want)
	}

	for _, request := range requests {
		assert.Equal(t, request.r.Header.Get(headerAccessToken), "ClientToken")
	}
}

func TestClientExportWithAccessToken(t *testing.T) {
	transport := &mockTransport{}
	c, err := New(defaultEndpointOption, WithHTTPClient(newMockHTTPClient(transport)))
	require.NoError(t, err)

	requestsBatches := [][]*jaegerpb.Batch{}

	for i := 0; i < 10; i++ {
		batches := []*jaegerpb.Batch{
			{
				Process: &jaegerpb.Process{ServiceName: "test_service_" + strconv.Itoa(i)},
				Spans:   []*jaegerpb.Span{{}},
			},
		}
		requestsBatches = append(requestsBatches, batches)
	}

	for _, batches := range requestsBatches {
		err := c.ExportWithAccessToken(context.Background(), batches, "Preferential")
		require.Nil(t, err)
	}

	requests := transport.requests()
	assert.Len(t, requests, len(requestsBatches))

	for i, want := range requestsBatches {
		assertRequestEqualBatches(t, requests[i].r, want)
	}

	for _, request := range requests {
		assert.Equal(t, request.r.Header.Get(headerAccessToken), "Preferential")
	}
}

func TestClientExportWithEmptyAccessToken(t *testing.T) {
	transport := &mockTransport{}
	c, err := New(defaultEndpointOption, WithHTTPClient(newMockHTTPClient(transport)), WithAccessToken("ClientToken"))
	require.NoError(t, err)

	requestsBatches := [][]*jaegerpb.Batch{}

	for i := 0; i < 10; i++ {
		batches := []*jaegerpb.Batch{
			{
				Process: &jaegerpb.Process{ServiceName: "test_service_" + strconv.Itoa(i)},
				Spans:   []*jaegerpb.Span{{}},
			},
		}
		requestsBatches = append(requestsBatches, batches)
	}

	for _, batches := range requestsBatches {
		err := c.ExportWithAccessToken(context.Background(), batches, "")
		require.Nil(t, err)
	}

	requests := transport.requests()
	assert.Len(t, requests, len(requestsBatches))

	for i, want := range requestsBatches {
		assertRequestEqualBatches(t, requests[i].r, want)
	}

	for _, request := range requests {
		assert.Equal(t, request.r.Header.Get(headerAccessToken), "ClientToken")
	}
}

func TestFailure(t *testing.T) {
	transport := &mockTransport{statusCode: 500}
	c, err := New(
		defaultEndpointOption,
		WithHTTPClient(newMockHTTPClient(transport)),
	)
	require.NoError(t, err)

	batches := []*jaegerpb.Batch{
		{
			Process: &jaegerpb.Process{ServiceName: "test_service"},
			Spans:   []*jaegerpb.Span{{}},
		},
	}

	err = c.Export(context.Background(), batches)
	require.NotNil(t, err)
	assert.Equal(t, "error exporting spans. server responded with status 500", err.Error())

	requests := transport.requests()
	require.Len(t, requests, 1)
	assertRequestEqualBatches(t, requests[0].r, batches)

	transport.reset(200)
	transport.err = errors.New("transport error")

	err = c.Export(context.Background(), batches)
	require.NotNil(t, err)
	assert.Equal(t, "Post \"http://local\": transport error", err.Error())

	requests = transport.requests()
	require.Len(t, requests, 1)
	assertRequestEqualBatches(t, requests[0].r, batches)
}

func TestRetries(t *testing.T) {
	transport := &mockTransport{statusCode: 500}
	c, err := New(
		defaultEndpointOption,
		WithHTTPClient(newMockHTTPClient(transport)),
	)
	require.NoError(t, err)

	batches := []*jaegerpb.Batch{
		{
			Process: &jaegerpb.Process{ServiceName: "test_service"},
			Spans:   []*jaegerpb.Span{{}},
		},
	}

	err = c.Export(context.Background(), batches)
	require.NotNil(t, err)
	assert.Equal(t, err.Error(), "error exporting spans. server responded with status 500")
	serr := err.(*ErrSend)
	assert.False(t, serr.Permanent)

	requests := transport.requests()
	require.Len(t, requests, 1)
	assertRequestEqualBatches(t, requests[0].r, batches)
}

func TestBadRequest(t *testing.T) {
	transport := &mockTransport{}

	c, err := New(
		defaultEndpointOption,
		WithHTTPClient(newMockHTTPClient(transport)),
	)
	require.NoError(t, err)

	batches := []*jaegerpb.Batch{
		{
			Process: &jaegerpb.Process{ServiceName: "test_service"},
			Spans:   []*jaegerpb.Span{{}},
		},
	}

	for _, code := range []int{400, 401} {
		transport.reset(code)
		err = c.Export(context.Background(), batches)
		require.NotNil(t, err)
		require.IsType(t, &ErrSend{}, err)
		serr := err.(*ErrSend)
		assert.True(t, serr.Permanent)
		assert.Equal(t, err.Error(), "dropping request: server responded with: "+strconv.Itoa(code))

		requests := transport.requests()
		require.Len(t, requests, 1)
		assertRequestEqualBatches(t, requests[0].r, batches)
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

	requestsBatches := make([][]*jaegerpb.Batch, numRequests)
	for i := 0; i < numRequests; i++ {
		requestsBatches[i] = []*jaegerpb.Batch{
			{
				Process: &jaegerpb.Process{ServiceName: "test_service"},
				Spans:   []*jaegerpb.Span{{}},
			},
		}
	}

	then := time.Now()
	for _, batches := range requestsBatches {
		go func(b []*jaegerpb.Batch) {
			err := c.Export(context.Background(), b)
			assert.Nil(t, err)
			wg.Done()
		}(batches)
	}
	wg.Wait()

	requests := transport.requests()
	require.Len(t, requests, 4)

	// ensure each batch took at least (workerDelay * batch's queue position) to complete
	for i, b := range requestsBatches {
		r := requests[i]
		delay := r.receivedAt.Sub(then)
		assert.GreaterOrEqual(t, int(delay), int(workerDelay*time.Duration(i)))
		assertRequestEqualBatches(t, r.r, b)
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
	for _, batches := range requestsBatches {
		go func(b []*jaegerpb.Batch) {
			err := c.Export(context.Background(), b)
			require.Nil(t, err)
			wg.Done()
		}(batches)
	}
	wg.Wait()

	requests = transport.requests()
	require.Len(t, requests, 4)

	// ensure all four requests completed within 100ms
	timeout := time.Millisecond * time.Duration(100)
	for i, b := range requestsBatches {
		r := requests[i]
		delay := r.receivedAt.Sub(then)
		assert.LessOrEqual(t, int(delay), int(timeout))
		assertRequestEqualBatches(t, r.r, b)
	}
}

func TestClientStop(t *testing.T) {
	transport := &mockTransport{
		statusCode: 429,
		headers: map[string]string{
			"Retry-After": "100",
		},
	}
	c, err := New(
		defaultEndpointOption,
		WithHTTPClient(newMockHTTPClient(transport)),
	)
	require.NoError(t, err)

	// should take more than 1 second
	batches := []*jaegerpb.Batch{
		{
			Process: &jaegerpb.Process{ServiceName: "test_service"},
			Spans:   []*jaegerpb.Span{{}},
		},
	}
	err = c.Export(context.Background(), batches)
	time.Sleep(10 * time.Millisecond)
	assert.NotNil(t, err)

	// if client is stopped, it should ignore pausing and return immediately
	then := time.Now()
	go func() {
		err = c.Export(context.Background(), batches)
		assert.NotNil(t, err)
	}()
	c.Stop()
	assert.True(t, time.Since(then) < time.Duration(101)*time.Millisecond)
}

func TestPauses(t *testing.T) {
	retryDelaySeconds := 2
	transport := &mockTransport{
		statusCode: 429,
		headers: map[string]string{
			"Retry-After": strconv.Itoa(retryDelaySeconds),
		},
	}

	numWorkers := 8
	c, err := New(
		defaultEndpointOption,
		WithHTTPClient(newMockHTTPClient(transport)),
		WithWorkers(uint(numWorkers)),
	)
	require.NoError(t, err)

	batches := []*jaegerpb.Batch{
		{
			Process: &jaegerpb.Process{ServiceName: "test_service"},
			Spans:   []*jaegerpb.Span{{}},
		},
	}

	then := time.Now()
	err = c.Export(context.Background(), batches)
	assert.NotNil(t, err)
	assert.True(t, time.Since(then) < time.Millisecond*time.Duration(100))

	// sleep to let pause goroutine kick in
	wait := time.Millisecond * 50
	time.Sleep(wait)

	wg := sync.WaitGroup{}
	wg.Add(numWorkers)

	elapsed := []time.Duration{}
	for i := 0; i < numWorkers; i++ {
		go func() {
			then := time.Now()
			c.Export(context.Background(), batches)
			elapsed = append(elapsed, time.Since(then)+wait)
			wg.Done()
		}()
	}

	wg.Wait()
	for _, e := range elapsed {
		assert.True(t, e >= time.Second*time.Duration(retryDelaySeconds))
	}
}
