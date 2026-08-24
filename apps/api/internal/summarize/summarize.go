// Package summarize turns a call transcript into a French meeting summary
// through the Anthropic Messages API — a Go port of the old ai-proxy.ts.
package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/FacileStudio/tronc/env"
)

const (
	apiURL          = "https://api.anthropic.com/v1/messages"
	apiVersion      = "2023-06-01"
	defaultModel    = "claude-sonnet-5"
	maxTokens       = 4096
	requestTimeout  = 2 * time.Minute
	maxResponseSize = 1 << 20
)

// Summarizer calls Claude over the Messages API.
type Summarizer struct {
	apiKey string
	model  string
	client *http.Client
}

// NewFromEnv builds a Summarizer from ANTHROPIC_API_KEY, or returns nil when
// the key is unset. A nil Summarizer is a supported state: the deployment
// simply has no AI summaries, and every caller must handle it.
func NewFromEnv() *Summarizer {
	key := env.String("ANTHROPIC_API_KEY", "")
	if key == "" {
		return nil
	}
	return &Summarizer{
		apiKey: key,
		model:  env.String("ANTHROPIC_MODEL", defaultModel),
		client: &http.Client{Timeout: requestTimeout},
	}
}

type messagesRequest struct {
	Model     string         `json:"model"`
	MaxTokens int            `json:"max_tokens"`
	Messages  []messageInput `json:"messages"`
}

type messageInput struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *apiError `json:"error,omitempty"`
}

// apiError is Anthropic's error envelope: {"error":{"type":..,"message":..}}.
type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Model reports the configured Claude model id. Summarize sends exactly this
// value, so the two can never disagree.
func (s *Summarizer) Model() string {
	if s.model == "" {
		return defaultModel
	}
	return s.model
}

const promptTemplate = "Voici la transcription brute d'une réunion. Rédige en français un résumé " +
	"structuré : décisions prises, actions à mener avec leur responsable si " +
	"identifiable, questions restées ouvertes. Sois factuel : n'invente rien " +
	"qui ne soit pas dans la transcription.\n\nTranscription :\n%s"

// Summarize turns a raw transcript into a French meeting summary.
func (s *Summarizer) Summarize(ctx context.Context, transcript string) (string, error) {
	body, err := json.Marshal(messagesRequest{
		Model:     s.Model(),
		MaxTokens: maxTokens,
		Messages:  []messageInput{{Role: "user", Content: fmt.Sprintf(promptTemplate, transcript)}},
	})
	if err != nil {
		return "", err
	}
	raw, status, err := s.post(ctx, body)
	if err != nil {
		return "", err
	}

	var decoded messagesResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("anthropic response (%d): %w", status, err)
	}
	if status >= http.StatusBadRequest || decoded.Error != nil {
		return "", fmt.Errorf("anthropic: %s", errorMessage(decoded.Error, status))
	}
	for _, block := range decoded.Content {
		if block.Type == "text" && block.Text != "" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("anthropic returned no text (model %s)", s.Model())
}

// errorMessage keeps a malformed error envelope from panicking: an API that
// fails is exactly the API most likely to answer in an unexpected shape.
func errorMessage(failure *apiError, status int) string {
	if failure == nil || failure.Message == "" {
		return fmt.Sprintf("status %d", status)
	}
	return failure.Message
}

func (s *Summarizer) post(ctx context.Context, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("x-api-key", s.apiKey)
	req.Header.Set("anthropic-version", apiVersion)
	req.Header.Set("Content-Type", "application/json")

	res, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponseSize))
	if err != nil {
		return nil, res.StatusCode, err
	}
	return raw, res.StatusCode, nil
}
