// Package checker provides concurrent HTTP availability checks for a list
// of URLs. It supports per-request timeouts, a global rate limit, and
// graceful context cancellation.
package checker

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Result holds the outcome of a single URL check.
type Result struct {
	URL        string        `json:"url"`
	StatusCode int           `json:"status_code"`
	Err        error         `json:"-"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
	Up         bool          `json:"up"`
}

// Options configures the Checker.
type Options struct {
	// Concurrency is the maximum number of simultaneous in-flight requests.
	Concurrency int
	// Timeout is the per-request timeout.
	Timeout time.Duration
	// RateLimit is the maximum number of requests started per second.
	// Zero means unlimited.
	RateLimit float64
	// Client is the HTTP client to use. If nil, a default client is created.
	Client *http.Client
}

// Checker runs availability checks against a list of URLs.
type Checker struct {
	opts   Options
	client *http.Client
}

// New creates a Checker with the given options, applying sensible defaults.
func New(opts Options) *Checker {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 10
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{
			Timeout: opts.Timeout,
			// Do not follow redirects indefinitely.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		}
	}
	return &Checker{opts: opts, client: client}
}

// Check runs availability checks for all URLs concurrently, respecting the
// configured concurrency and rate limits, and returns one Result per URL in
// the same order as the input. It honors ctx cancellation.
func (c *Checker) Check(ctx context.Context, urls []string) []Result {
	results := make([]Result, len(urls))
	if len(urls) == 0 {
		return results
	}

	// Rate limiter: simple token bucket via time.Ticker.
	var rateCh chan time.Time
	if c.opts.RateLimit > 0 {
		interval := time.Duration(float64(time.Second) / c.opts.RateLimit)
		if interval <= 0 {
			interval = time.Nanosecond
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		rateCh = ticker.C
	}

	// Semaphore for bounded concurrency.
	sem := make(chan struct{}, c.opts.Concurrency)
	var wg sync.WaitGroup

	for i, url := range urls {
		// Honor context cancellation before scheduling more work.
		select {
		case <-ctx.Done():
			// Fill remaining results with cancellation error.
			for j := i; j < len(urls); j++ {
				results[j] = Result{
					URL:   urls[j],
					Err:   ctx.Err(),
					Error: ctx.Err().Error(),
					Up:    false,
				}
			}
			wg.Wait()
			return results
		default:
		}

		// Rate limit wait (before acquiring the semaphore so we don't
		// hold a slot while waiting).
		if rateCh != nil {
			select {
			case <-rateCh:
			case <-ctx.Done():
				for j := i; j < len(urls); j++ {
					results[j] = Result{
						URL:   urls[j],
						Err:   ctx.Err(),
						Error: ctx.Err().Error(),
						Up:    false,
					}
				}
				wg.Wait()
				return results
			}
		}

		// Acquire a semaphore slot (respecting ctx).
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			for j := i; j < len(urls); j++ {
				results[j] = Result{
					URL:   urls[j],
					Err:   ctx.Err(),
					Error: ctx.Err().Error(),
					Up:    false,
				}
			}
			wg.Wait()
			return results
		}

		wg.Add(1)
		go func(idx int, u string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = c.checkOne(ctx, u)
		}(i, url)
	}

	wg.Wait()
	return results
}

func (c *Checker) checkOne(ctx context.Context, url string) Result {
	start := time.Now()
	res := Result{URL: url}

	reqCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		res.Duration = time.Since(start)
		res.Err = err
		res.Error = err.Error()
		res.Up = false
		return res
	}
	// Some sites reject requests without a User-Agent.
	req.Header.Set("User-Agent", "go-uptime-checker/1.0")

	resp, err := c.client.Do(req)
	res.Duration = time.Since(start)
	if err != nil {
		res.Err = err
		res.Error = err.Error()
		res.Up = false
		return res
	}
	defer resp.Body.Close()

	res.StatusCode = resp.StatusCode
	res.Up = resp.StatusCode >= 200 && resp.StatusCode < 400
	if !res.Up {
		res.Error = fmt.Sprintf("unexpected status: %d", resp.StatusCode)
	}
	return res
}