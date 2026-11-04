package recording

import (
	"context"
	"sync"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/modules/rooms"
	troncerrors "github.com/FacileStudio/tronc/errors"
)

// Recording tracks one running RoomComposite egress per room slug. Echo is
// single-node for v1, so an in-memory map is enough; the egress id is also
// returned to the client so stop works even after an API restart.
type Recording struct {
	rooms *rooms.Service
	media *media.Service

	mu      sync.Mutex
	active  map[string]string
	stopped map[string]bool
}

// NewRecording builds a Recording gate over the rooms and media services.
func NewRecording(roomService *rooms.Service, mediaService *media.Service) *Recording {
	return &Recording{rooms: roomService, media: mediaService, active: map[string]string{}, stopped: map[string]bool{}}
}

var errNotRecording = troncerrors.Conflict("no recording is running for this room")

// Start begins recording the room. Only the owner may record, and only when
// the owner's plan includes recording.
func (rec *Recording) Start(ctx context.Context, slug string, callerID int64) (media.EgressInfo, error) {
	if err := rec.rooms.RequireRecording(ctx, slug, callerID); err != nil {
		return media.EgressInfo{}, err
	}
	if err := rec.reserve(slug); err != nil {
		return media.EgressInfo{}, err
	}
	info, err := rec.media.StartRecording(ctx, slug)
	if err != nil {
		rec.release(slug)
		return media.EgressInfo{}, err
	}
	return rec.settle(slug, info), nil
}

// reserve marks a start in flight for the room, refusing a second one.
func (rec *Recording) reserve(slug string) error {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if _, busy := rec.active[slug]; busy {
		return troncerrors.Conflict("room is already being recorded")
	}
	rec.active[slug] = ""
	return nil
}

// release drops a reservation whose start failed.
func (rec *Recording) release(slug string) {
	rec.mu.Lock()
	delete(rec.active, slug)
	rec.mu.Unlock()
}

// settle records the started egress, unless a stop raced the start and
// already asked for it to end.
func (rec *Recording) settle(slug string, info media.EgressInfo) media.EgressInfo {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.stopped[info.EgressID] {
		delete(rec.active, slug)
		return media.EgressInfo{}
	}
	rec.active[slug] = info.EgressID
	return info
}

// Stop ends the room's active recording.
func (rec *Recording) Stop(ctx context.Context, slug string, callerID int64) (media.EgressInfo, error) {
	if err := rec.rooms.RequireOwner(ctx, slug, callerID); err != nil {
		return media.EgressInfo{}, err
	}
	rec.mu.Lock()
	egressID, ok := rec.active[slug]
	delete(rec.active, slug)
	if ok {
		rec.stopped[egressID] = true
	}
	rec.mu.Unlock()
	if !ok {
		return media.EgressInfo{}, errNotRecording
	}
	info, err := rec.media.StopRecording(ctx, egressID)
	if err != nil {
		return media.EgressInfo{}, err
	}
	return info, nil
}

// EgressIDFor exposes the running egress id of a room, if any. An id of ""
// means a start is still in flight.
func (rec *Recording) EgressIDFor(slug string) (string, bool) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	id, ok := rec.active[slug]
	return id, ok
}
