// Package summarize turns a call transcript into a French meeting summary
// through Bulle's /api/ai/complete, acting for the room owner with a token
// Registre exchanged for audience bulle. Echo holds no model key of its own.
package summarize

import (
	"context"
	stderrors "errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/FacileStudio/porte/exchange"
	"github.com/FacileStudio/porte/forfait"
	"github.com/FacileStudio/tronc/bulle"
	troncerrors "github.com/FacileStudio/tronc/errors"
)

const (
	requestTimeout = 90 * time.Second
	notesTokens    = 2048
	summaryTokens  = 4096
)

func target() exchange.Target {
	return exchange.Target{Audience: "bulle", Scopes: []string{"bulle.read", "bulle.write"}}
}

// Tokens mints a Bulle token for one Echo user. *exchange.Exchanger fits it.
type Tokens interface {
	Token(ctx context.Context, userID int64, target exchange.Target) (string, error)
	Forget(userID int64, target exchange.Target)
}

// Summarizer asks Bulle for meeting summaries. It is safe for concurrent use.
type Summarizer struct {
	client *bulle.Client
	logger *slog.Logger
}

type ownerKey struct{}

// New builds a Summarizer that calls Bulle at bulleURL with tokens minted for
// the owner each Summarize call names. Redirects are refused, so a token
// never follows a Location header off Bulle's origin.
func New(bulleURL string, tokens Tokens, logger *slog.Logger) (*Summarizer, error) {
	if logger == nil {
		logger = slog.Default()
	}
	client, err := bulle.New(bulle.Config{
		URL: bulleURL,
		TokenSource: func(ctx context.Context) (string, error) {
			owner, ok := ctx.Value(ownerKey{}).(int64)
			if !ok || owner == 0 {
				return "", exchange.ErrNoUser
			}
			return tokens.Token(ctx, owner, target())
		},
		Forget: func(ctx context.Context) {
			if owner, ok := ctx.Value(ownerKey{}).(int64); ok {
				tokens.Forget(owner, target())
			}
		},
		HTTPClient: &http.Client{
			Timeout: requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return &Summarizer{client: client, logger: logger}, nil
}

// Summarize writes the summary of transcript as ownerID, and names the model
// Bulle used. A nil Summarizer answers unavailable instead of panicking. A
// transcript longer than one Bulle input is noted part by part in parallel,
// then the notes are summarized in one more call.
func (s *Summarizer) Summarize(ctx context.Context, ownerID int64, transcript string) (string, string, error) {
	if s == nil {
		return "", "", troncerrors.Unavailable("AI summaries are not configured on this deployment")
	}
	ctx = context.WithValue(ctx, ownerKey{}, ownerID)
	parts, truncated := split(transcript)
	if len(parts) == 1 {
		return s.complete(ctx, bulle.CompleteRequest{Prompt: summaryPrompt(truncated), Input: parts[0], MaxTokens: summaryTokens})
	}
	notes, err := s.notes(ctx, parts)
	if err != nil {
		return "", "", err
	}
	return s.complete(ctx, bulle.CompleteRequest{Prompt: combinePrompt(truncated), Input: joinNotes(notes), MaxTokens: summaryTokens})
}

func (s *Summarizer) notes(ctx context.Context, parts []string) ([]string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	notes := make([]string, len(parts))
	var first error
	var once sync.Once
	var wg sync.WaitGroup
	for i, part := range parts {
		wg.Go(func() {
			request := bulle.CompleteRequest{Prompt: notesPrompt(i+1, len(parts)), Input: part, MaxTokens: notesTokens}
			note, _, err := s.complete(ctx, request)
			if err != nil {
				once.Do(func() { first = err; cancel() })
				return
			}
			notes[i] = note
		})
	}
	wg.Wait()
	return notes, first
}

func (s *Summarizer) complete(ctx context.Context, request bulle.CompleteRequest) (string, string, error) {
	answer, err := s.client.Complete(ctx, request)
	if err != nil {
		return "", "", s.failure(ctx, err)
	}
	text := strings.TrimSpace(answer.Text)
	if text == "" {
		return "", "", troncerrors.Unavailable("Bulle returned an empty summary; try again")
	}
	return text, answer.Model, nil
}

// failure maps a Bulle or Registre failure onto the errors Echo already
// answers with: Bulle's 402 becomes the same plan refusal RequireAI gives,
// and its 429 says whether the daily cap or the short burst limit refused.
// A call cancelled because a sibling part failed is not logged again.
func (s *Summarizer) failure(ctx context.Context, err error) error {
	var plan *bulle.PlanRequired
	if stderrors.As(err, &plan) {
		key := plan.Key
		if key == "" {
			key = plans.KeyAIAccess
		}
		return &forfait.LimitError{Key: key, UpgradeURL: plan.UpgradeURL}
	}
	var known *troncerrors.Error
	if stderrors.As(err, &known) {
		return err
	}
	var status *bulle.StatusError
	if stderrors.As(err, &status) && status.Status == http.StatusTooManyRequests {
		return tooMany(status)
	}
	if ctx.Err() == nil {
		s.logger.Warn("summary: bulle call failed", slog.Any("error", err))
	}
	return troncerrors.Unavailable("Bulle could not write the summary right now; try again later")
}

// tooMany is Bulle's 429 in Echo's words. The daily cap answers a
// rate_limited envelope naming the day; the per-user burst limiter answers
// plain text, which leaves no code.
func tooMany(status *bulle.StatusError) error {
	if status.Code == "rate_limited" && strings.Contains(status.Message, "day") {
		return troncerrors.RateLimited("the daily AI limit is reached; try again tomorrow")
	}
	return troncerrors.RateLimited("too many AI requests in a row; try again in a minute")
}
