package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJSONRetriesThenParsesFencedReply(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"<think>{not this}</think>\n`+"```json\\n{\\\"p_bad\\\": 0.3}\\n```"+`"}}]}`)
	}))
	defer srv.Close()

	c := New("k", "m")
	c.BaseURL = srv.URL
	c.Backoff = []time.Duration{0, 0}

	var out struct {
		PBad float64 `json:"p_bad"`
	}
	if err := c.JSON(context.Background(), "s", "u", &out); err != nil {
		t.Fatal(err)
	}
	if out.PBad != 0.3 || calls != 2 {
		t.Fatalf("p_bad=%v calls=%d", out.PBad, calls)
	}
}

func TestJSONDoesNotRetryClientErrors(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New("k", "m")
	c.BaseURL = srv.URL
	c.Backoff = []time.Duration{0, 0}
	if err := c.JSON(context.Background(), "s", "u", &struct{}{}); err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestJSONNoKey(t *testing.T) {
	if err := New("", "m").JSON(context.Background(), "s", "u", &struct{}{}); !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("err = %v", err)
	}
}
