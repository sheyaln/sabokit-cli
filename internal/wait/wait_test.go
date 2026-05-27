package wait

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTCPSucceeds(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	addr := l.Addr().String()
	go func() {
		c, _ := l.Accept()
		if c != nil {
			c.Close()
		}
	}()
	if err := TCP(addr, Options{Attempts: 3, Interval: 50 * time.Millisecond, Timeout: time.Second}); err != nil {
		t.Errorf("TCP probe should succeed: %v", err)
	}
}

func TestTCPFailsFast(t *testing.T) {
	// dialing a never-listened-on port should fail across attempts
	err := TCP("127.0.0.1:1", Options{Attempts: 2, Interval: 10 * time.Millisecond, Timeout: 100 * time.Millisecond})
	if err == nil {
		t.Error("expected error")
	}
}

func TestHTTPStatusSucceeds(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	if err := HTTPStatus(srv.URL, 200, Options{Attempts: 5, Interval: 20 * time.Millisecond, Timeout: time.Second}, nil); err != nil {
		t.Errorf("HTTPStatus should eventually succeed: %v", err)
	}
	if calls < 3 {
		t.Errorf("expected at least 3 attempts, got %d", calls)
	}
}

func TestHTTPOnAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	attempts := 0
	_ = HTTPStatus(srv.URL, 200, Options{Attempts: 3, Interval: 5 * time.Millisecond, Timeout: time.Second}, func(n int, err error) {
		attempts++
		if n != attempts {
			t.Errorf("attempt = %d, want %d", n, attempts)
		}
	})
	if attempts != 3 {
		t.Errorf("expected 3 onAttempt calls, got %d", attempts)
	}
}
