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
	// OpenAI-compatible APIs return embeddings in the same order as the input;
	// the Index field is redundant. Map by slice order to be robust against
	// servers that omit Index.
	if len(er.Data) != len(batch) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(batch), len(er.Data))
	}
	out := make([][]float32, len(batch))
	for i, item := range er.Data {
		out[i] = item.Embedding
	}
	for i, v := range out {
		if v == nil {
			return nil, fmt.Errorf("missing embedding at index %d", i)
		}
	}
	return out, nil
}