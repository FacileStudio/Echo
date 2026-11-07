package capture

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
	"gorm.io/gorm"
)

// Notify finalizes the uploads of a call when LiveKit reports its room
// finished, so a recording whose tab closed before the end still lands.
func (s *Service) Notify(ctx context.Context, event *livekit.WebhookEvent) {
	if event.GetEvent() != "room_finished" || s.dir == "" {
		return
	}
	err := s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var calls []schemas.Call
		if err := tx.Where("livekit_room_sid = ? AND livekit_room_sid <> ''", event.GetRoom().GetSid()).Find(&calls).Error; err != nil {
			return err
		}
		for _, call := range calls {
			if err := finalizeCall(tx, s.dir, call.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Warn("finalizing recordings at the end of a call failed", slog.Any("error", err))
	}
}

// Run finalizes uploads that stopped receiving chunks, the case where the tab
// closed and the call carried on, until ctx ends.
func (s *Service) Run(ctx context.Context) {
	if s.dir == "" {
		return
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.FinalizeStale(ctx, time.Now().Add(-staleAfter)); err != nil {
				slog.Warn("finalizing stale recordings failed", slog.Any("error", err))
			}
		}
	}
}

// FinalizeStale finalizes every upload that has not changed since cutoff.
func (s *Service) FinalizeStale(ctx context.Context, cutoff time.Time) error {
	return s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []schemas.Recording
		err := tx.Where("status = ? AND updated_at < ?", schemas.RecordingUploading, cutoff).Find(&rows).Error
		if err != nil {
			return err
		}
		return finalizeRows(tx, s.dir, rows)
	})
}

func finalizeCall(tx *gorm.DB, dir string, callID uuid.UUID) error {
	var rows []schemas.Recording
	err := tx.Where("call_id = ? AND status = ?", callID, schemas.RecordingUploading).Find(&rows).Error
	if err != nil {
		return err
	}
	return finalizeRows(tx, dir, rows)
}

func finalizeRows(tx *gorm.DB, dir string, rows []schemas.Recording) error {
	for _, rec := range rows {
		if rec.Status != schemas.RecordingUploading {
			continue
		}
		if rec.Size == 0 {
			if err := tx.Delete(&schemas.Recording{}, "id = ?", rec.ID).Error; err != nil {
				return err
			}
			remove(dir, rec.Filename)
			continue
		}
		err := tx.Model(&schemas.Recording{}).Where("id = ?", rec.ID).Update("status", schemas.RecordingComplete).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func remove(dir, name string) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return
	}
	defer root.Close()
	if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("could not remove an empty recording", slog.Any("error", err))
	}
}
