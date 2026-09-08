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