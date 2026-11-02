// Package summarize turns a call transcript into a French meeting summary
// through the Anthropic Messages API — a Go port of the old ai-proxy.ts.
package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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

	// maxTranscriptBytes bounds what leaves for a paid third party. Roughly
	// 40 k tokens of French, comfortably inside the context window and inside
	// Anthropic's request size limit whatever the transcript ceiling becomes.
	maxTranscriptBytes = 160 << 10
)

// ErrNotConfigured is returned when a nil Summarizer is asked to work. A nil
// Summarizer is a supported state, so it answers instead of panicking.
var ErrNotConfigured = errors.New("AI summaries are not configured on this deployment")

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
	StopReason string    `json:"stop_reason"`
	Error      *apiError `json:"error,omitempty"`
}

// apiError is Anthropic's error envelope: {"error":{"type":..,"message":..}}.
type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Model reports the configured Claude model id. Summarize sends exactly this
// value, so the two can never disagree.
func (s *Summarizer) Model() string {
	if s == nil || s.model == "" {
		return defaultModel
	}
	return s.model
}

// Summarize turns a raw transcript into a French meeting summary.
func (s *Summarizer) Summarize(ctx context.Context, transcript string) (string, error) {
	if s == nil {
		return "", ErrNotConfigured
	}
	body, err := json.Marshal(messagesRequest{
		Model:     s.Model(),
		MaxTokens: maxTokens,
		Messages:  []messageInput{{Role: "user", Content: buildPrompt(transcript)}},
	})
	if err != nil {
		return "", err
	}
	raw, status, err := s.post(ctx, body)
	if err != nil {
		return "", s.redact(err)
	}

	var decoded messagesResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("anthropic response (%d): %w", status, err)
	}
	if status >= http.StatusBadRequest || decoded.Error != nil {
		return "", fmt.Errorf("anthropic: %s", errorMessage(decoded.Error, status))
	}
	return text(decoded, s.Model())
}

// text pulls the summary out of a successful response, but only when the
// model actually finished: a max_tokens cut or a refusal leaves partial prose
// that would otherwise be stored as though it were the whole summary.
func text(decoded messagesResponse, model string) (string, error) {
	switch decoded.StopReason {
	case "max_tokens":
		return "", errors.New("the call was too long to summarise in one pass")
	case "refusal":
		return "", errors.New("the model declined to summarise this transcript")
	}
	for _, block := range decoded.Content {
		if block.Type == "text" && block.Text != "" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("anthropic returned no text (model %s)", model)
}

// errorMessage keeps a malformed error envelope from panicking: an API that
// fails is exactly the API most likely to answer in an unexpected shape.
func errorMessage(failure *apiError, status int) string {
	if failure == nil || failure.Message == "" {
		return fmt.Sprintf("status %d", status)
	}
	return failure.Message
}

// redact makes it impossible for the key to reach a log line, whatever a
// transport error or a proxy decides to echo back.
func (s *Summarizer) redact(err error) error {
	if err == nil || s == nil || s.apiKey == "" {
		return err
	}
	message := err.Error()
	if !strings.Contains(message, s.apiKey) {
		return err
	}
	return errors.New(strings.ReplaceAll(message, s.apiKey, "[redacted]"))
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
