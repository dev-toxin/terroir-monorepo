// Package storage declares repository interfaces. Implementations live in sub-packages
// (sqlstore: SQLite and PostgreSQL over database/sql).
package storage

import (
	"context"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
)

// CellarFilter narrows cellar listings. Zero values mean "no filter".
type CellarFilter struct {
	Color        domain.Color
	Country      string
	Grape        string
	Location     string
	Query        string // substring over name/producer/region/notes
	InStockOnly  bool
	IncludeEmpty bool
}

// Users stores users, tokens and cellar memberships.
type Users interface {
	CreateUser(ctx context.Context, u domain.User) error
	GetUser(ctx context.Context, id string) (domain.User, error)
	AddToken(ctx context.Context, userID, tokenHash string) error
	UserByToken(ctx context.Context, tokenHash string) (domain.User, error)
	CreateCellar(ctx context.Context, c domain.Cellar) error
	AddMember(ctx context.Context, m domain.Membership) error
	MembershipsOf(ctx context.Context, userID string) ([]domain.Membership, error)
	MembersOf(ctx context.Context, cellarID string) ([]domain.User, error)
}

// Wines stores wine cards (scoped by cellar).
type Wines interface {
	CreateWine(ctx context.Context, w domain.Wine) error
	GetWine(ctx context.Context, cellarID, id string) (domain.Wine, error)
	ListWines(ctx context.Context, cellarID string) ([]domain.Wine, error)
	UpdateWine(ctx context.Context, w domain.Wine) error
	// DeleteWine fails with domain.ErrConflict if bottles reference the wine.
	DeleteWine(ctx context.Context, cellarID, id string) error
}

// Cellar stores bottles and consumption history.
type Cellar interface {
	CreateItem(ctx context.Context, it domain.CellarItem) error
	GetItem(ctx context.Context, cellarID, id string) (domain.CellarEntry, error)
	UpdateItem(ctx context.Context, it domain.CellarItem) error
	ListItems(ctx context.Context, cellarID string, f CellarFilter) ([]domain.CellarEntry, error)
	// Drink atomically decrements quantity by ev.Bottles (never below zero) and records the event.
	// Returns domain.ErrOutOfStock if not enough bottles.
	Drink(ctx context.Context, cellarID string, ev domain.ConsumptionEvent) (domain.CellarItem, error)
	ListConsumption(ctx context.Context, cellarID string, userID string, limit int) ([]domain.ConsumptionEvent, error)
}

// Taste stores taste profiles and survey answers.
type Taste interface {
	GetProfile(ctx context.Context, userID string) (domain.TasteProfile, error) // empty profile if none
	// SaveProfileWithAnswer stores the profile and the answer in one transaction.
	SaveProfileWithAnswers(ctx context.Context, p domain.TasteProfile, answers []domain.SurveyAnswer) error
	AnsweredQuestions(ctx context.Context, userID string) (map[string]bool, error)
}

// Recipes stores recipes and pairing feedback.
type Recipes interface {
	CreateRecipe(ctx context.Context, r domain.Recipe) error
	GetRecipe(ctx context.Context, cellarID, id string) (domain.Recipe, error)
	ListRecipes(ctx context.Context, cellarID string) ([]domain.Recipe, error)
	UpdateRecipe(ctx context.Context, r domain.Recipe) error
	DeleteRecipe(ctx context.Context, cellarID, id string) error
	AddPairingFeedback(ctx context.Context, f domain.PairingFeedback) error
	ListPairingFeedback(ctx context.Context, cellarID string) ([]domain.PairingFeedback, error)
}

// Assistant stores chat history, memory facts and LLM usage.
type Assistant interface {
	AddMessage(ctx context.Context, m domain.ChatMessage) error
	ListMessages(ctx context.Context, conversationID string, limit int) ([]domain.ChatMessage, error)
	AddFact(ctx context.Context, f domain.MemoryFact) error
	ListFacts(ctx context.Context, userID string, limit int) ([]domain.MemoryFact, error)
	AddUsage(ctx context.Context, u domain.LLMUsage) error
	// UsageSince sums tokens and cost of all users since t (monthly budget is global for the install).
	UsageSince(ctx context.Context, t time.Time) (tokens int64, costMicros int64, err error)
}

// Store is the full persistence API.
type Store interface {
	Users
	Wines
	Cellar
	Taste
	Recipes
	Assistant
	Ping(ctx context.Context) error
	Kind() string
	Close() error
}
