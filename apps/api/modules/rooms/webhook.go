package rooms

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/FacileStudio/Echo/apps/api/internal/media"
	"github.com/FacileStudio/Echo/apps/api/internal/plans"
	"github.com/livekit/protocol/livekit"
)

// Notify keeps the owner's seat free for as long as the call runs. The
// webhook receiver calls it after every recorded event.
//
// The join path alone cannot hold the seat: once a participant connects,
// LiveKit refreshes their token with a long lifetime, so a client can keep
// identities and reconnect them after Echo's pending window lapsed. On every
// participant_joined of a capped room whose owner is absent, everyone above
// limit-1 is disconnected, guests first and the most recently joined first,
// which is the newcomer in the normal case.
func (s *Service) Notify(ctx context.Context, event *livekit.WebhookEvent) {
	if event.GetEvent() != "participant_joined" || event.GetRoom().GetName() == "" {
		return
	}
	room, err := s.BySlug(ctx, event.GetRoom().GetName())
	if err != nil || room == nil || room.OwnerID == nil || room.IntegrationApp != "" {
		return
	}
	policy, err := s.plans.ForRoom(ctx, *room)
	if err != nil {
		slog.Warn("owner seat: plan lookup failed", slog.String("room", room.Slug), slog.Any("error", err))
		return
	}
	ceiling := s.plans.Limit(policy, plans.KeyMaxParticipants)
	if ceiling < 0 {
		return
	}
	call, err := s.latestCall(ctx, room)
	if err != nil {
		return
	}
	owner := fmt.Sprintf("user-%d", *room.OwnerID)
	people := s.present(ctx, room.Slug, call)
	if slices.ContainsFunc(people, func(who media.Connected) bool { return who.Identity == owner }) {
		return
	}
	s.trim(ctx, room.Slug, people, ceiling-1)
}
