package llmclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedBatch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var req EmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "text-embedding-3-small" {
			t.Errorf("model = %s", req.Model)
		}
		// Return one vector per input.
		resp := EmbedResponse{
			Data: make([]EmbedItem, len(req.Input)),
		}
		for i := range req.Input {
			resp.Data[i].Embedding = []float32{0.1, 0.2, 0.3}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewEmbeddingClient(srv.URL, "sk-test", "text-embedding-3-small", 64)
	vecs, err := c.EmbedBatch([]string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 3 {
		t.Fatalf("got %d vecs", len(vecs))
	}
	if len(vecs[0]) != 3 {
		t.Errorf("vec0 dim = %d", len(vecs[0]))
	}
}

func TestEmbedBatch_RetriesOn5xx(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var req EmbedRequest
		json.NewDecoder(r.Body).Decode(&req)
		resp := EmbedResponse{Data: []EmbedItem{{Embedding: []float32{0.1}}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewEmbeddingClient(srv.URL, "sk-test", "m", 64)
	_, err := c.EmbedBatch([]string{"x"})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestEmbedBatch_PartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "bad model")
	}))
	defer srv.Close()

	c := NewEmbeddingClient(srv.URL, "sk-test", "bad-model", 64)
	_, err := c.EmbedBatch([]string{"x"})
	if err == nil {
		t.Fatal("expected error for 400")
	}
}