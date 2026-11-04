// Package plans is how Echo enforces the plan a room owner pays for.
//
// Echo has no spaces, so every entitlement is read for the personal scope of
// the room owner's Registre subject. porte/forfait holds the values Registre
// pushes; this package resolves which scope a room reads and turns refusals
// into the 402 the client shows.
package plans

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/FacileStudio/Echo/apps/api/schemas"
	"github.com/FacileStudio/porte/forfait"
	"gorm.io/gorm"
)

// The feature keys Echo enforces, as Ardoise names them.
const (
	KeyMaxCallMinutes  = "echo.max_call_minutes"
	KeyMaxParticipants = "echo.max_participants"
	KeyRecording       = "echo.recording"
)

// Features are the free-plan defaults that apply once Registre has pushed
// entitlements and a scope carries no value of its own.
func Features() []forfait.Feature {
	return []forfait.Feature{
		{Key: KeyMaxCallMinutes, Kind: forfait.Limit, Default: json.RawMessage(`40`)},
		{Key: KeyMaxParticipants, Kind: forfait.Limit, Default: json.RawMessage(`4`)},
		{Key: KeyRecording, Kind: forfait.Bool, Default: json.RawMessage(`false`)},
	}
}

// Plans resolves the plan that governs a room. It is safe for concurrent use.
type Plans struct {
	orm    *gorm.DB
	store  *forfait.Store
	issuer string
	logger *slog.Logger
}

// New builds Plans over Echo's database. issuer is the OIDC issuer, which is
// both the porte provider name the owner's subject is stored under and the
// Registre origin upgrade links point at. With no issuer the store never
// receives a snapshot and fails open: a self-hosted Echo without Registre
// stays unlimited.
func New(orm *gorm.DB, issuer string, logger *slog.Logger) *Plans {
	if logger == nil {
		logger = slog.Default()
	}
	store := forfait.NewStore(Features()...)
	store.Issuer = issuer
	store.Logger = logger
	return &Plans{orm: orm, store: store, issuer: issuer, logger: logger}
}

// Policy is the plan one room runs under. Unlimited marks a room no plan
// governs; Scope is the Registre scope read otherwise.
type Policy struct {
	Scope     forfait.Scope
	Unlimited bool
}

// ForRoom resolves the scope of a room.
//
// An integration room has no owner: another suite app created it and pays for
// it through its own product, so it is unlimited. Applying the free plan
// there would cut every existing Prisme call at four people and forty
// minutes. Any other room reads its owner's personal scope. A room with no
// owner, or an owner who never signed in through Registre, reads the scope of
// an empty subject, which Registre never carries: the free defaults. That
// keeps an anonymous guest-created room from being a way around the plan.
func (p *Plans) ForRoom(ctx context.Context, room schemas.Room) (Policy, error) {
	if room.IntegrationApp != "" {
		return Policy{Unlimited: true}, nil
	}
	if room.OwnerID == nil {
		return Policy{}, nil
	}
	subject, err := p.SubjectOf(ctx, *room.OwnerID)
	if err != nil {
		return Policy{}, err
	}
	return Policy{Scope: forfait.Scope{Subject: subject}}, nil
}

// SubjectOf is the Registre subject of an Echo user: the subject porte/oidc
// stored for them under this issuer at their last SSO login, or "" when they
// never signed in through Registre.
func (p *Plans) SubjectOf(ctx context.Context, userID int64) (string, error) {
	if p.issuer == "" {
		return "", nil
	}
	var subjects []string
	err := p.orm.WithContext(ctx).
		Raw(`SELECT subject FROM porte_identities WHERE user_id = ? AND provider = ? ORDER BY created_at DESC NULLS LAST LIMIT 1`,
			userID, p.issuer).
		Scan(&subjects).Error
	if err != nil || len(subjects) == 0 {
		return "", err
	}
	return subjects[0], nil
}

// Limit is a limit feature's ceiling for the policy, -1 meaning unlimited.
func (p *Plans) Limit(policy Policy, key string) int64 {
	if policy.Unlimited {
		return -1
	}
	return p.store.Limit(policy.Scope, key)
}

// Check refuses going from used to used+adding of a limit feature with a
// *forfait.LimitError.
func (p *Plans) Check(policy Policy, key string, used, adding int64) error {
	if policy.Unlimited {
		return nil
	}
	return p.store.Check(policy.Scope, key, used, adding)
}

// Require refuses a bool feature the policy does not enable, with the same
// *forfait.LimitError a limit refusal carries and a limit of 0.
func (p *Plans) Require(policy Policy, key string) error {
	if policy.Unlimited || p.store.Enabled(policy.Scope, key) {
		return nil
	}
	refusal := &forfait.LimitError{Key: key}
	if p.issuer != "" {
		refusal.UpgradeURL = forfait.UpgradeURL(p.issuer, policy.Scope.SpaceID)
	}
	return refusal
}
