// Package capture runs in-browser transcription and recording. Calls are
// end-to-end encrypted, so the server hears nothing: each browser transcribes
// its own microphone and the organizer's browser records the tracks it is
// allowed to. This package keeps the part the server can and must decide:
// whether the organizer's plan allows it, who accepted, which uploads and
// lines to take, and where the recording is stored.
package capture

import (
	"fmt"

	"github.com/FacileStudio/Echo/apps/api/internal/middleware"
	"github.com/FacileStudio/Echo/apps/api/internal/seat"
	"github.com/FacileStudio/Echo/apps/api/modules/rooms"
	"gorm.io/gorm"
)

// Service owns capture state and recording uploads.
type Service struct {
	orm     *gorm.DB
	rooms   *rooms.Service
	keys    *seat.Keys
	limiter *middleware.RateLimiter
	dir     string
}

// NewService builds a Service. dir is the recordings volume; empty disables
// recording on this node.
func NewService(orm *gorm.DB, roomService *rooms.Service, keys *seat.Keys, limiter *middleware.RateLimiter, dir string) *Service {
	return &Service{orm: orm, rooms: roomService, keys: keys, limiter: limiter, dir: dir}
}

func identityOf(userID int64) string { return fmt.Sprintf("user-%d", userID) }
