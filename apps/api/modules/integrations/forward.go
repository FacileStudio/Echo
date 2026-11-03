package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
	"gorm.io/gorm"
)

// SignatureHeader carries the hex HMAC-SHA256 of the raw body under the
// receiving app's integration secret.
const SignatureHeader = "X-Echo-Signature"

const deliveryAttempts = 3

// forwardedEvents are the LiveKit events an app hears about. The rest are
// Echo's own bookkeeping.
var forwardedEvents = map[string]bool{
	"room_finished":      true,
	"participant_joined": true,
	"participant_left":   true,
}

// Forwarder relays LiveKit events for integration rooms to the owning app's
// callback_url, signed with that app's secret.
type Forwarder struct {
	orm    *gorm.DB
	keys   Keys
	client *http.Client
	log    *slog.Logger
	retry  time.Duration
}

// NewForwarder builds a Forwarder over the database and the apps' secrets.
func NewForwarder(orm *gorm.DB, keys Keys, log *slog.Logger) *Forwarder {
	return &Forwarder{orm: orm, keys: keys, client: &http.Client{Timeout: 10 * time.Second}, log: log, retry: 2 * time.Second}
}

// Notify forwards one verified LiveKit event. The room lookup runs on the
// webhook request; the delivery runs in the background so a slow app never
// delays LiveKit's own webhook.
func (f *Forwarder) Notify(ctx context.Context, event *livekit.WebhookEvent) {
	out, err := f.prepare(ctx, event)
	if err != nil {
		f.log.Warn("integration callback not prepared", slog.Any("error", err))
		return
	}
	if out != nil {
		go f.deliver(context.WithoutCancel(ctx), out)
	}
}

type delivery struct {
	app  string
	url  string
	body []byte
}

func (f *Forwarder) prepare(ctx context.Context, event *livekit.WebhookEvent) (*delivery, error) {
	if !forwardedEvents[event.GetEvent()] || event.GetRoom().GetName() == "" {
		return nil, nil
	}
	if who := event.GetParticipant(); who != nil {
		if _, minted := mintedIdentity(who.GetIdentity()); !minted {
			return nil, nil
		}
	}
	var room schemas.Room
	err := f.orm.WithContext(ctx).
		Where("slug = ? AND integration_app <> '' AND callback_url <> ''", event.GetRoom().GetName()).
		First(&room).Error
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(callbackEvent(&room, event))
	if err != nil {
		return nil, err
	}
	return &delivery{app: room.IntegrationApp, url: room.CallbackURL, body: body}, nil
}

func callbackEvent(room *schemas.Room, event *livekit.WebhookEvent) CallbackEvent {
	at := time.Now().UTC()
	if event.GetCreatedAt() > 0 {
		at = time.Unix(event.GetCreatedAt(), 0).UTC()
	}
	payload := CallbackEvent{
		ID:         uuid.NewString(),
		Event:      event.GetEvent(),
		RoomID:     room.Slug,
		ExternalID: room.ExternalID,
		At:         at.Format(time.RFC3339),
	}
	if identity, minted := mintedIdentity(event.GetParticipant().GetIdentity()); minted {
		payload.Participant = &Participant{Identity: identity, Name: event.GetParticipant().GetName()}
	}
	return payload
}

// deliver posts the event, retrying with a growing pause, and reports
// whether the app accepted it.
func (f *Forwarder) deliver(ctx context.Context, out *delivery) bool {
	signature := f.keys.Sign(out.app, out.body)
	var err error
	for attempt := 1; attempt <= deliveryAttempts; attempt++ {
		if err = f.post(ctx, out.url, signature, out.body); err == nil {
			return true
		}
		if attempt < deliveryAttempts {
			time.Sleep(time.Duration(attempt) * f.retry)
		}
	}
	f.log.Warn("integration callback failed", slog.String("app", out.app), slog.Any("error", err))
	return false
}

func (f *Forwarder) post(ctx context.Context, url, signature string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, signature)
	res, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("callback answered %d", res.StatusCode)
	}
	return nil
}
