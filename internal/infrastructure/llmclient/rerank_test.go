package llmclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRerank_ReordersByScore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/rerank" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var req RerankRequest
		json.NewDecoder(r.Body).Decode(&req)
		// Return scores reversed from input order to force reorder.
		resp := RerankResponse{
			Results: []RerankItem{
				{Index: 2, Score: 0.9},
				{Index: 0, Score: 0.5},
				{Index: 1, Score: 0.3},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewRerankClient(srv.URL, "sk-test", "rerank-v1")
	got, err := c.Rerank("q", []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Index != 2 || got[1].Index != 0 || got[2].Index != 1 {
		t.Errorf("order = %v", got)
	}
}