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

// Start begins recording the room. Only the owner may record.
func (rec *Recording) Start(ctx context.Context, slug string, callerID int64) (media.EgressInfo, error) {
	if err := rec.rooms.RequireOwner(ctx, slug, callerID); err != nil {
		return media.EgressInfo{}, err
	}
	rec.mu.Lock()
	if _, busy := rec.active[slug]; busy {
		rec.mu.Unlock()
		return media.EgressInfo{}, troncerrors.Conflict("room is already being recorded")
	}
	rec.mu.Unlock()

	info, err := rec.media.StartRecording(ctx, slug)
	if err != nil {
		return media.EgressInfo{}, err
	}
	rec.mu.Lock()
	if rec.stopped[info.EgressID] {
		rec.mu.Unlock()
		return media.EgressInfo{}, nil
	}
	rec.active[slug] = info.EgressID
	rec.mu.Unlock()
	return info, nil
}

// Stop ends the room's active recording.
func (rec *Recording) Stop(ctx context.Context, slug string, callerID int64) (media.EgressInfo, error) {
	if err := rec.rooms.RequireOwner(ctx, slug, callerID); err != nil {
		return media.EgressInfo{}, err
	}
	rec.mu.Lock()
	egressID, ok := rec.active[slug]
	delete(rec.active, slug)
	rec.stopped[egressID] = true
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

// EgressIDFor exposes the running egress id of a room, if any.
func (rec *Recording) EgressIDFor(slug string) (string, bool) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	id, ok := rec.active[slug]
	return id, ok
}
