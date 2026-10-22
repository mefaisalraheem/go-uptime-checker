package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCheck_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Options{Concurrency: 4, Timeout: 2 * time.Second})
	results := c.Check(context.Background(), []string{srv.URL})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if !r.Up {
		t.Fatalf("expected up, got %+v", r)
	}
	if r.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", r.StatusCode)
	}
	if r.Err != nil {
		t.Fatalf("unexpected err: %v", r.Err)
	}
}

func TestCheck_FailureStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(Options{Concurrency: 2, Timeout: 2 * time.Second})
	results := c.Check(context.Background(), []string{srv.URL})
	if results[0].Up {
		t.Fatalf("expected down, got %+v", results[0])
	}
	if results[0].StatusCode != 500 {
		t.Fatalf("expected 500, got %d", results[0].StatusCode)
	}
}

func TestCheck_Timeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer func() { close(block); srv.Close() }()

	c := New(Options{Concurrency: 2, Timeout: 100 * time.Millisecond})
	results := c.Check(context.Background(), []string{srv.URL})
	if results[0].Up {
		t.Fatalf("expected down due to timeout, got %+v", results[0])
	}
	if results[0].Err == nil {
		t.Fatalf("expected error")
	}
}

func TestCheck_ConcurrencyLimit(t *testing.T) {
	var inFlight, maxInFlight int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&maxInFlight)
			if cur <= old || atomic.CompareAndSwapInt32(&maxInFlight, old, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	urls := make([]string, 10)
	for i := range urls {
		urls[i] = srv.URL
	}

	c := New(Options{Concurrency: 3, Timeout: 2 * time.Second})
	c.Check(context.Background(), urls)

	if got := atomic.LoadInt32(&maxInFlight); got > 3 {
		t.Fatalf("concurrency limit exceeded: %d", got)
	}
}

func TestCheck_ContextCancellation(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer func() { close(block); srv.Close() }()

	urls := make([]string, 5)
	for i := range urls {
		urls[i] = srv.URL
	}

	c := New(Options{Concurrency: 5, Timeout: 5 * time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	results := c.Check(ctx, urls)
	elapsed := time.Since(start)
	if elapsed > 2*time.Second {
		t.Fatalf("cancellation took too long: %v", elapsed)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Up {
			t.Fatalf("expected down due to cancellation, got %+v", r)
		}
	}
}

func TestCheck_Empty(t *testing.T) {
	c := New(Options{})
	results := c.Check(context.Background(), nil)
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %d", len(results))
	}
}

func TestCheck_PreservesOrder(t *testing.T) {
	// Two servers with different latencies; ensure output order matches input.
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer fast.Close()
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	urls := []string{slow.URL, fast.URL}
	c := New(Options{Concurrency: 2, Timeout: time.Second})
	results := c.Check(context.Background(), urls)
	if results[0].URL != urls[0] || results[1].URL != urls[1] {
		t.Fatalf("order not preserved: %+v", results)
	}
}

func TestNew_Defaults(t *testing.T) {
	c := New(Options{})
	if c.opts.Concurrency <= 0 {
		t.Fatalf("expected default concurrency")
	}
	if c.opts.Timeout <= 0 {
		t.Fatalf("expected default timeout")
	}
}