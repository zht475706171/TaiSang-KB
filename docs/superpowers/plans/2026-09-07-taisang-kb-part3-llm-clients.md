# TaiSang-KB Implementation Plan (Part 3: LLM/Embedding/Rerank clients)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** OpenAI-compatible HTTP clients for chat completions (streaming), embeddings (batched), and rerank.

**Architecture:** Three independent client structs in `internal/infrastructure/llmclient/`, all hitting OpenAI-compatible endpoints. No SDK dependency — raw `net/http` + `encoding/json` for full control and minimal deps. Streaming uses `bufio.Scanner` over the SSE response body.

**Tech Stack:** Go stdlib (`net/http`, `encoding/json`, `bufio`).

**Spec ref:** §5 (embeddingClient, llmClient, reranker), §6.2 (streaming protocol), §7.1 (retry/timeout).

**Prerequisite:** Part 1 complete.

---

### Task 1: Embedding client — batch + retry

**Files:**
- Create: `internal/infrastructure/llmclient/embedding.go`
- Create: `internal/infrastructure/llmclient/embedding_test.go`

- [ ] **Step 1: Write the failing test (mock server)**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/infrastructure/llmclient/ -v
```
Expected: FAIL (no embedding.go).

- [ ] **Step 3: Write implementation**

```go
package llmclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type EmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type EmbedResponse struct {
	Data []EmbedItem `json:"data"`
}

type EmbedItem struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type EmbeddingClient struct {
	baseURL    string
	apiKey     string
	model      string
	batchSize  int
	httpClient *http.Client
}

func NewEmbeddingClient(baseURL, apiKey, model string, batchSize int) *EmbeddingClient {
	if batchSize <= 0 {
		batchSize = 64
	}
	return &EmbeddingClient{
		baseURL:   baseURL,
		apiKey:    apiKey,
		model:     model,
		batchSize: batchSize,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// EmbedBatch embeds all inputs, splitting into sub-batches of batchSize.
// Returns one vector per input (same order). On retry exhaustion for a sub-batch,
// returns error with the failed index range; partial successes are not returned.
func (c *EmbeddingClient) EmbedBatch(inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	out := make([][]float32, 0, len(inputs))
	for i := 0; i < len(inputs); i += c.batchSize {
		end := i + c.batchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		batch := inputs[i:end]
		vecs, err := c.embedBatchWithRetry(batch)
		if err != nil {
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}
		out = append(out, vecs...)
	}
	return out, nil
}

func (c *EmbeddingClient) embedBatchWithRetry(batch []string) ([][]float32, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * time.Second) // 2s, 4s
		}
		vecs, err := c.embedBatchOnce(batch)
		if err == nil {
			return vecs, nil
		}
		lastErr = err
		// Don't retry on 4xx (client error).
		if he, ok := err.(HTTPError); ok && he.StatusCode >= 400 && he.StatusCode < 500 {
			return nil, err
		}
	}
	return nil, lastErr
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("http %d: %s", e.StatusCode, e.Body)
}

func (c *EmbeddingClient) embedBatchOnce(batch []string) ([][]float32, error) {
	body, _ := json.Marshal(EmbedRequest{Model: c.model, Input: batch})
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, HTTPError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	var er EmbedResponse
	if err := json.Unmarshal(respBody, &er); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	// Sort by index to guarantee order.
	out := make([][]float32, len(batch))
	for _, item := range er.Data {
		if item.Index >= 0 && item.Index < len(out) {
			out[item.Index] = item.Embedding
		}
	}
	for i, v := range out {
		if v == nil {
			return nil, fmt.Errorf("missing embedding at index %d", i)
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/infrastructure/llmclient/ -v
```
Expected: 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/llmclient/
git commit -m "feat(llmclient): embedding client with batching and retry"
```

---

### Task 2: LLM streaming chat client

**Files:**
- Create: `internal/infrastructure/llmclient/chat.go`
- Create: `internal/infrastructure/llmclient/chat_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/infrastructure/llmclient/ -run TestStreamChat -v
```
Expected: FAIL.

- [ ] **Step 3: Write implementation**

```go
package llmclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type ChatClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewChatClient(baseURL, apiKey, model string) *ChatClient {
	return &ChatClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// StreamChat sends messages and invokes onDelta for each streamed content delta.
// Returns nil when stream completes with [DONE].
func (c *ChatClient) StreamChat(messages []ChatMessage, onDelta func(delta string) error) error {
	body, _ := json.Marshal(chatRequest{Model: c.model, Messages: messages, Stream: true})
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return HTTPError{StatusCode: resp.StatusCode, Body: string(b)}
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			return nil
		}
		var chunk chatStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue // skip malformed
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content != "" {
				if err := onDelta(ch.Delta.Content); err != nil {
					return err
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/infrastructure/llmclient/ -run TestStreamChat -v
```
Expected: 2 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/llmclient/
git commit -m "feat(llmclient): streaming chat client (SSE)"
```

---

### Task 3: Rerank client

**Files:**
- Create: `internal/infrastructure/llmclient/rerank.go`
- Create: `internal/infrastructure/llmclient/rerank_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/infrastructure/llmclient/ -run TestRerank -v
```
Expected: FAIL.

- [ ] **Step 3: Write implementation**

```go
package llmclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

type RerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

type RerankResponse struct {
	Results []RerankItem `json:"results"`
}

type RerankItem struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
	Score          float64 `json:"score"` // some APIs use Score, some RelevanceScore
}

type RerankClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewRerankClient(baseURL, apiKey, model string) *RerankClient {
	return &RerankClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Rerank returns documents indices sorted by relevance score descending.
func (c *RerankClient) Rerank(query string, documents []string) ([]RerankItem, error) {
	if len(documents) == 0 {
		return nil, nil
	}
	body, _ := json.Marshal(RerankRequest{
		Model:     c.model,
		Query:     query,
		Documents: documents,
		TopN:      len(documents),
	})
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/rerank", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, HTTPError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	var rr RerankResponse
	if err := json.Unmarshal(respBody, &rr); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	// Normalize score field.
	for i := range rr.Results {
		if rr.Results[i].RelevanceScore == 0 && rr.Results[i].Score != 0 {
			rr.Results[i].RelevanceScore = rr.Results[i].Score
		}
	}
	sort.SliceStable(rr.Results, func(i, j int) bool {
		return rr.Results[i].RelevanceScore > rr.Results[j].RelevanceScore
	})
	return rr.Results, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/infrastructure/llmclient/ -run TestRerank -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/llmclient/
git commit -m "feat(llmclient): rerank client with score normalization"
```

---

### Task 4: Self-review and checkpoint

- [ ] **Step 1: Run all llmclient tests**

```bash
go test ./internal/infrastructure/llmclient/ -v
```
Expected: all PASS.

- [ ] **Step 2: Run whole repo**

```bash
go test ./... -count=1 -short
go vet ./...
```
Expected: PASS / no issues.

- [ ] **Step 3: Commit checkpoint**

```bash
git commit --allow-empty -m "checkpoint: Part 3 llm clients complete"
```

---

## End of Part 3

**What's done:**
- Embedding client: batched + retry (3 attempts, exp backoff, skip retry on 4xx)
- Chat client: SSE streaming with delta callback
- Rerank client: score normalization + sort

**Next:** Part 4 — services + handlers + async parse worker.