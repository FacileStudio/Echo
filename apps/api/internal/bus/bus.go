// Package bus queues Echo's call lifecycle for the Antenne pool.
//
// Every change is written to antenne_outbox inside the transaction that
// records it, so a call event is published if and only if it committed. The
// drainer in internal/pool sends the rows. A nil *Outbox is publishing turned
// off, which is what Echo runs without the Antenne variables: every method is
// then a no-op and no row is written.
//
// The payload is the full state of the call every time: Agenda draws a strip
// for each signed-in participant from it, so a consumer upserts on facile_id
// and never needs the previous event. Guests never appear in it, and neither
// do calls in integration rooms, which belong to the app that created them.
package bus

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FacileStudio/antenne-client/go/catalog"
	"github.com/FacileStudio/antenne-client/go/envelope"
	"github.com/FacileStudio/antenne-client/go/outbox"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Channels returns the channels Echo publishes on, declared when it registers.
func Channels() []catalog.Channel {
	return []catalog.Channel{catalog.CallStarted, catalog.CallUpdated, catalog.CallEnded}
}

// Outbox enqueues call events. A nil *Outbox does nothing.
type Outbox struct {
	publicURL string
}

// New returns a publishing outbox. publicURL is Echo's origin, which the
// call link is built on; empty leaves the link out.
func New(publicURL string) *Outbox {
	return &Outbox{publicURL: strings.TrimRight(publicURL, "/")}
}

// Started queues call.started for a call just opened in tx.
func (o *Outbox) Started(tx *gorm.DB, callID uuid.UUID) error {
	return o.queue(tx, callID, catalog.CallStarted, envelope.ActionCreated, "started")
}

// Joined queues call.updated when userID just became a participant of the
// call in tx. The key names the person, so each addition is one event and a
// redelivered join is none.
func (o *Outbox) Joined(tx *gorm.DB, callID uuid.UUID, userID int64) error {
	return o.queue(tx, callID, catalog.CallUpdated, envelope.ActionUpdated, "joined:"+strconv.FormatInt(userID, 10))
}

// Ended queues call.ended for a call closed in tx. A call still open is left
// alone.
func (o *Outbox) Ended(tx *gorm.DB, callID uuid.UUID) error {
	return o.queue(tx, callID, catalog.CallEnded, envelope.ActionUpdated, "ended")
}

// queue writes the call's current state to the outbox under the key the
// step names, so a retried step reuses its key and queues nothing more.
func (o *Outbox) queue(tx *gorm.DB, callID uuid.UUID, channel catalog.Channel, action envelope.Action, step string) error {
	if o == nil {
		return nil
	}
	sqlTx, ok := tx.Statement.ConnPool.(*sql.Tx)
	if !ok {
		return fmt.Errorf("bus: a call event must be queued inside its transaction")
	}
	ctx := tx.Statement.Context
	payload, err := o.load(ctx, sqlTx, callID)
	if err != nil || payload == nil {
		return err
	}
	if channel == catalog.CallEnded && payload.EndedAt == nil {
		return nil
	}
	event := wrap(*payload, action, "echo:call:"+payload.FacileID+":"+step)
	err = outbox.Enqueue(ctx, sqlTx, channel, event, event.IdempotencyKey)
	if errors.Is(err, outbox.ErrDuplicate) {
		return nil
	}
	return err
}

// wrap puts call in the suite envelope.
func wrap(call envelope.Call, action envelope.Action, key string) envelope.Event[envelope.Call] {
	return envelope.Event[envelope.Call]{
		Version:        envelope.EventVersion,
		App:            envelope.AppEcho,
		Object:         envelope.ObjectCall,
		Action:         action,
		FacileID:       call.FacileID,
		Payload:        call,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		IdempotencyKey: key,
	}
}
