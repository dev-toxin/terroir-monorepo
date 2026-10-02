// Package eval holds the reference ("blind") question set used to compare the Terroir assistant
// with a general-purpose chatbot on the same data. See docs/eval/blind-questions.md.
//
// Each question declares expected PROPERTIES of a good answer (bottles exist and are in stock,
// colour/tannin of the top pick, drinking-window urgency, words that must or must not appear),
// not an exact text — so the same checks can later score real LLM answers.
package eval

import (
	_ "embed"
	"encoding/json"
)

//go:embed testdata/blind_questions.json
var questionsJSON []byte

// Expect lists properties a good answer must have.
type Expect struct {
	MinBottles        int      `json:"min_bottles"`
	InStockOnly       bool     `json:"in_stock_only"`
	TopColors         []string `json:"top_colors"`
	TopMaxTannin      *int     `json:"top_max_tannin"`
	TopMinTannin      *int     `json:"top_min_tannin"`
	TopMinBody        *int     `json:"top_min_body"`
	TopWindows        []string `json:"top_windows"`
	AnswerContainsAny []string `json:"answer_contains_any"`
	AnswerContainsAll []string `json:"answer_contains_all"`
	AnswerNotContains []string `json:"answer_not_contains"`
}

// Question is one reference question.
type Question struct {
	ID       string `json:"id"`
	User     string `json:"user"` // anton | elena (seed users)
	Question string `json:"question"`
	Intent   string `json:"intent"`
	Expect   Expect `json:"expect"`
	Why      string `json:"why"`
}

// Questions returns the reference set.
func Questions() ([]Question, error) {
	var qs []Question
	err := json.Unmarshal(questionsJSON, &qs)
	return qs, err
}
