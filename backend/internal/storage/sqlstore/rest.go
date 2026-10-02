package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
)

// ---------- taste ----------

// GetProfile returns the stored profile, or an empty one (version 0) if none.
func (s *Store) GetProfile(ctx context.Context, userID string) (domain.TasteProfile, error) {
	var data string
	err := s.db.QueryRowContext(ctx, s.q(`SELECT data FROM taste_profiles WHERE user_id = ?`), userID).Scan(&data)
	if err == sql.ErrNoRows {
		return domain.TasteProfile{UserID: userID, LikedGrapes: []string{}, RejectedGrapes: []string{}, Regions: []string{}, Colors: []domain.Color{}}, nil
	}
	if err != nil {
		return domain.TasteProfile{}, err
	}
	var p domain.TasteProfile
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		return p, err
	}
	p.UserID = userID
	return p, nil
}

// SaveProfileWithAnswers upserts the profile and answers in one transaction.
func (s *Store) SaveProfileWithAnswers(ctx context.Context, p domain.TasteProfile, answers []domain.SurveyAnswer) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, s.q(`INSERT INTO taste_profiles (user_id, data, version, updated_at) VALUES (?, ?, ?, ?)
ON CONFLICT (user_id) DO UPDATE SET data = excluded.data, version = excluded.version, updated_at = excluded.updated_at`),
			p.UserID, string(data), p.Version, s.ts(p.UpdatedAt))
		if err != nil {
			return err
		}
		for _, a := range answers {
			vals, _ := json.Marshal(a.Values)
			_, err := tx.ExecContext(ctx, s.q(`INSERT INTO survey_answers (user_id, question_id, answer_values, answered_at) VALUES (?, ?, ?, ?)
ON CONFLICT (user_id, question_id) DO UPDATE SET answer_values = excluded.answer_values, answered_at = excluded.answered_at`),
				p.UserID, a.QuestionID, string(vals), s.ts(a.AnsweredAt))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// AnsweredQuestions returns the set of answered question ids.
func (s *Store) AnsweredQuestions(ctx context.Context, userID string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT question_id FROM survey_answers WHERE user_id = ?`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// ---------- recipes ----------

const recipeCols = `id, cellar_id, title, cuisine, main_ingredient, cooking_method, sauce, spiciness, richness, acidity, time_minutes, notes, tags, created_at, updated_at`

func scanRecipe(row interface{ Scan(...any) error }) (domain.Recipe, error) {
	var r domain.Recipe
	var ing, method, sauce string
	var tags jsonList
	var created, updated dbTime
	err := row.Scan(&r.ID, &r.CellarID, &r.Title, &r.Cuisine, &ing, &method, &sauce, &r.Spiciness, &r.Richness, &r.Acidity,
		&r.TimeMinutes, &r.Notes, &tags, &created, &updated)
	if err != nil {
		return r, notFound(err)
	}
	r.MainIngredient, r.CookingMethod, r.Sauce = domain.MainIngredient(ing), domain.CookingMethod(method), domain.Sauce(sauce)
	r.Tags = []string(tags)
	r.CreatedAt, r.UpdatedAt = created.T, updated.T
	return r, nil
}

// CreateRecipe inserts a recipe.
func (s *Store) CreateRecipe(ctx context.Context, r domain.Recipe) error {
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO recipes (`+recipeCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		r.ID, r.CellarID, r.Title, r.Cuisine, string(r.MainIngredient), string(r.CookingMethod), string(r.Sauce), r.Spiciness,
		r.Richness, r.Acidity, r.TimeMinutes, r.Notes, jsonList(r.Tags), s.ts(r.CreatedAt), s.ts(r.UpdatedAt))
	return err
}

// GetRecipe returns a recipe.
func (s *Store) GetRecipe(ctx context.Context, cellarID, id string) (domain.Recipe, error) {
	return scanRecipe(s.db.QueryRowContext(ctx, s.q(`SELECT `+recipeCols+` FROM recipes WHERE cellar_id = ? AND id = ?`), cellarID, id))
}

// ListRecipes lists recipes of a cellar by title.
func (s *Store) ListRecipes(ctx context.Context, cellarID string) ([]domain.Recipe, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT `+recipeCols+` FROM recipes WHERE cellar_id = ? ORDER BY title, id`), cellarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Recipe{}
	for rows.Next() {
		r, err := scanRecipe(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateRecipe overwrites a recipe.
func (s *Store) UpdateRecipe(ctx context.Context, r domain.Recipe) error {
	res, err := s.db.ExecContext(ctx, s.q(`UPDATE recipes SET title = ?, cuisine = ?, main_ingredient = ?, cooking_method = ?, sauce = ?,
spiciness = ?, richness = ?, acidity = ?, time_minutes = ?, notes = ?, tags = ?, updated_at = ? WHERE cellar_id = ? AND id = ?`),
		r.Title, r.Cuisine, string(r.MainIngredient), string(r.CookingMethod), string(r.Sauce), r.Spiciness, r.Richness, r.Acidity,
		r.TimeMinutes, r.Notes, jsonList(r.Tags), s.ts(r.UpdatedAt), r.CellarID, r.ID)
	return affected(res, err)
}

// DeleteRecipe deletes a recipe (its pairing feedback cascades; consumption keeps a dangling id on purpose).
func (s *Store) DeleteRecipe(ctx context.Context, cellarID, id string) error {
	res, err := s.db.ExecContext(ctx, s.q(`DELETE FROM recipes WHERE cellar_id = ? AND id = ?`), cellarID, id)
	return affected(res, err)
}

// AddPairingFeedback stores a pairing rating.
func (s *Store) AddPairingFeedback(ctx context.Context, f domain.PairingFeedback) error {
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO pairing_feedback (id, cellar_id, recipe_id, wine_id, user_id, rating, note, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`), f.ID, f.CellarID, f.RecipeID, f.WineID, f.UserID, f.Rating, f.Note, s.ts(f.CreatedAt))
	if isFKViolation(err) {
		return domain.Invalid("recipe_id", "recipe or wine not found")
	}
	return err
}

// ListPairingFeedback lists all pairing ratings of a cellar.
func (s *Store) ListPairingFeedback(ctx context.Context, cellarID string) ([]domain.PairingFeedback, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id, cellar_id, recipe_id, wine_id, user_id, rating, note, created_at FROM pairing_feedback
WHERE cellar_id = ? ORDER BY created_at, id`), cellarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.PairingFeedback{}
	for rows.Next() {
		var f domain.PairingFeedback
		var at dbTime
		if err := rows.Scan(&f.ID, &f.CellarID, &f.RecipeID, &f.WineID, &f.UserID, &f.Rating, &f.Note, &at); err != nil {
			return nil, err
		}
		f.CreatedAt = at.T
		out = append(out, f)
	}
	return out, rows.Err()
}

// ---------- assistant ----------

// AddMessage stores a chat message.
func (s *Store) AddMessage(ctx context.Context, m domain.ChatMessage) error {
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO chat_messages (id, conversation_id, user_id, role, content, created_at) VALUES (?, ?, ?, ?, ?, ?)`),
		m.ID, m.ConversationID, m.UserID, m.Role, m.Content, s.ts(m.CreatedAt))
	return err
}

// ListMessages returns the last `limit` messages of a conversation in chronological order.
func (s *Store) ListMessages(ctx context.Context, conversationID string, limit int) ([]domain.ChatMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id, conversation_id, user_id, role, content, created_at FROM chat_messages
WHERE conversation_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`), conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ChatMessage
	for rows.Next() {
		var m domain.ChatMessage
		var at dbTime
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.UserID, &m.Role, &m.Content, &at); err != nil {
			return nil, err
		}
		m.CreatedAt = at.T
		out = append(out, m)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

// AddFact stores a memory fact.
func (s *Store) AddFact(ctx context.Context, f domain.MemoryFact) error {
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO memory_facts (id, user_id, fact, source, created_at) VALUES (?, ?, ?, ?, ?)`),
		f.ID, f.UserID, f.Fact, f.Source, s.ts(f.CreatedAt))
	return err
}

// ListFacts returns the newest facts first.
func (s *Store) ListFacts(ctx context.Context, userID string, limit int) ([]domain.MemoryFact, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT id, user_id, fact, source, created_at FROM memory_facts WHERE user_id = ?
ORDER BY created_at DESC, id DESC LIMIT ?`), userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.MemoryFact{}
	for rows.Next() {
		var f domain.MemoryFact
		var at dbTime
		if err := rows.Scan(&f.ID, &f.UserID, &f.Fact, &f.Source, &at); err != nil {
			return nil, err
		}
		f.CreatedAt = at.T
		out = append(out, f)
	}
	return out, rows.Err()
}

// AddUsage records an LLM call.
func (s *Store) AddUsage(ctx context.Context, u domain.LLMUsage) error {
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO llm_usage (id, user_id, usage_period, provider, model, prompt_tokens, completion_tokens, cost_micros, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`), u.ID, u.UserID, u.CreatedAt.UTC().Format("2006-01"), u.Provider, u.Model, u.PromptTokens,
		u.CompletionTokens, u.CostMicros, s.ts(u.CreatedAt))
	return err
}

// UsageSince sums usage from t (inclusive).
func (s *Store) UsageSince(ctx context.Context, t time.Time) (int64, int64, error) {
	var tokens, cost sql.NullInt64
	err := s.db.QueryRowContext(ctx, s.q(`SELECT SUM(prompt_tokens + completion_tokens), SUM(cost_micros) FROM llm_usage WHERE created_at >= ?`),
		s.ts(t)).Scan(&tokens, &cost)
	return tokens.Int64, cost.Int64, err
}
