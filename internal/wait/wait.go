// Package wait provides bounded retry loops for TCP and HTTP probes.
// Used by sabokit up to wait for SSH reachability, Let's Encrypt cert,
// and Authentik blueprint indexing without shelling to curl/nc.
package wait

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Options controls the retry policy.
type Options struct {
	Attempts int           // total attempts (>= 1)
	Interval time.Duration // delay between attempts
	Timeout  time.Duration // per-attempt timeout
}

// DefaultTCP is the reachability probe up.sh used (port 22, up to 5min).
func DefaultTCP() Options {
	return Options{Attempts: 60, Interval: 5 * time.Second, Timeout: 5 * time.Second}
}

// DefaultHTTP is the LE cert probe up.sh used (~4min total).
func DefaultHTTP() Options {
	return Options{Attempts: 24, Interval: 10 * time.Second, Timeout: 10 * time.Second}
}

// TCP probes addr until DialTimeout succeeds or attempts are exhausted.
func TCP(addr string, opts Options) error {
	if opts.Attempts < 1 {
		opts.Attempts = 1
	}
	var lastErr error
	for i := 0; i < opts.Attempts; i++ {
		c, err := net.DialTimeout("tcp", addr, opts.Timeout)
		if err == nil {
			c.Close()
			return nil
		}
		lastErr = err
		if i+1 < opts.Attempts {
			time.Sleep(opts.Interval)
		}
	}
	return fmt.Errorf("tcp %s not reachable after %d attempts: %w", addr, opts.Attempts, lastErr)
}

// HTTP probes url with GET until the response satisfies the predicate or
// attempts are exhausted. TLS verification is disabled by default since
// the LE cert may not be valid yet during the probe loop.
//
// onAttempt (optional) is called after each failed attempt with the
// attempt number (1-indexed) and the err/response status — useful for
// printing a hint like "forcing traefik restart" mid-loop.
func HTTP(url string, predicate func(*http.Response) bool, opts Options, onAttempt func(attempt int, err error)) error {
	if opts.Attempts < 1 {
		opts.Attempts = 1
	}
	client := &http.Client{
		Timeout: opts.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	for i := 0; i < opts.Attempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		var ok bool
		if err == nil {
			ok = predicate(resp)
			resp.Body.Close()
		}
		cancel()
		if ok {
			return nil
		}
		if onAttempt != nil {
			onAttempt(i+1, err)
		}
		if i+1 < opts.Attempts {
			time.Sleep(opts.Interval)
		}
	}
	return fmt.Errorf("http %s never satisfied predicate after %d attempts", url, opts.Attempts)
}

// HTTPStatus is a shorthand HTTP probe that checks for a specific status.
func HTTPStatus(url string, status int, opts Options, onAttempt func(int, error)) error {
	return HTTP(url, func(r *http.Response) bool { return r.StatusCode == status }, opts, onAttempt)
}
