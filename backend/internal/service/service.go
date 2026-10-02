// Package service implements Terroir use cases on top of storage and pure domain logic.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/storage"
)

// Actor is the authenticated user acting in their (first) cellar.
type Actor struct {
	User     domain.User
	CellarID string
	Role     domain.Role
}

// Service bundles use cases. It is safe for concurrent use.
type Service struct {
	store storage.Store
	clock domain.Clock
	newID func() string
}

// New constructs a Service.
func New(store storage.Store, clock domain.Clock, newID func() string) *Service {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &Service{store: store, clock: clock, newID: newID}
}

// Store exposes the store (for health checks).
func (s *Service) Store() storage.Store { return s.store }

// Now returns the service clock time.
func (s *Service) Now() time.Time { return s.clock.Now() }

// HashToken hashes an API token for storage. Raw tokens are never stored.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Authenticate resolves a raw bearer token to an Actor.
func (s *Service) Authenticate(ctx context.Context, token string) (Actor, error) {
	if token == "" {
		return Actor{}, domain.ErrUnauthorized
	}
	u, err := s.store.UserByToken(ctx, HashToken(token))
	if err != nil {
		if err == domain.ErrNotFound {
			return Actor{}, domain.ErrUnauthorized
		}
		return Actor{}, err
	}
	ms, err := s.store.MembershipsOf(ctx, u.ID)
	if err != nil {
		return Actor{}, err
	}
	if len(ms) == 0 {
		return Actor{}, domain.ErrForbidden
	}
	return Actor{User: u, CellarID: ms[0].CellarID, Role: ms[0].Role}, nil
}

// RegisterTokens stores tokens from config (token -> user id). Unknown users are skipped with an error.
func (s *Service) RegisterTokens(ctx context.Context, tokens map[string]string) error {
	for tok, uid := range tokens {
		if _, err := s.store.GetUser(ctx, uid); err != nil {
			return err
		}
		if err := s.store.AddToken(ctx, uid, HashToken(tok)); err != nil {
			return err
		}
	}
	return nil
}
