package llmclient

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamChat_DeltasForwarded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	c := NewChatClient(srv.URL, "sk-test", "gpt-4o-mini")
	var sb strings.Builder
	err := c.StreamChat([]ChatMessage{{Role: "user", Content: "hi"}}, func(delta string) error {
		sb.WriteString(delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if sb.String() != "Hello" {
		t.Errorf("got %q", sb.String())
	}
}

func TestStreamChat_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "invalid api key")
	}))
	defer srv.Close()

	c := NewChatClient(srv.URL, "sk-test", "gpt-4o-mini")
	err := c.StreamChat([]ChatMessage{{Role: "user", Content: "hi"}}, func(string) error { return nil })
	if err == nil {
		t.Fatal("expected error for 401")
	}
}