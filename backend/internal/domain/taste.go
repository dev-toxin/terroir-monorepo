package domain

import (
	"slices"
	"strings"
	"time"
)

// Axis names a taste-profile dimension the survey can change.
type Axis string

const (
	AxisAcidity      Axis = "acidity"   // preferred level 1..5
	AxisTannin       Axis = "tannin"    // preferred level 1..5
	AxisBody         Axis = "body"      // preferred level 1..5
	AxisSweetness    Axis = "sweetness" // preferred level 1..5 (1 = bone dry)
	AxisFruitDark    Axis = "fruit_dark"
	AxisFruitRed     Axis = "fruit_red"
	AxisOak          Axis = "oak"
	AxisSpice        Axis = "spice_tolerance" // food heat 0..3
	AxisLikedGrapes  Axis = "liked_grapes"
	AxisRejected     Axis = "rejected_grapes"
	AxisRegions      Axis = "regions"
	AxisBudget       Axis = "budget"
	AxisColorsPrefer Axis = "colors"
)

// TasteProfile is a per-user structured taste profile. Scalar axes are 1..5, 0 = unknown.
type TasteProfile struct {
	UserID           string    `json:"user_id"`
	Acidity          int       `json:"acidity"`
	Tannin           int       `json:"tannin"`
	Body             int       `json:"body"`
	Sweetness        int       `json:"sweetness"`
	FruitDark        int       `json:"fruit_dark"`
	FruitRed         int       `json:"fruit_red"`
	Oak              int       `json:"oak"`
	SpiceTolerance   int       `json:"spice_tolerance"` // 0..3
	LikedGrapes      []string  `json:"liked_grapes"`
	RejectedGrapes   []string  `json:"rejected_grapes"`
	Regions          []string  `json:"regions"`
	Colors           []Color   `json:"colors"`
	BudgetMinMinor   int64     `json:"budget_min_minor"`
	BudgetMaxMinor   int64     `json:"budget_max_minor"`
	BudgetCurrency   string    `json:"budget_currency"`
	Notes            string    `json:"notes"`
	Version          int       `json:"version"`
	UpdatedAt        time.Time `json:"updated_at"`
	AnsweredQuestion int       `json:"answered_questions"`
}

// Likes reports whether the grape is in the liked list.
func (p *TasteProfile) Likes(grape string) bool { return containsGrape(p.LikedGrapes, grape) }

// Rejects reports whether the grape is in the rejected list.
func (p *TasteProfile) Rejects(grape string) bool { return containsGrape(p.RejectedGrapes, grape) }

func containsGrape(list []string, grape string) bool {
	want := CanonicalGrape(grape)
	for _, g := range list {
		if CanonicalGrape(g) == want {
			return true
		}
	}
	return false
}

// AnswerType of a survey question.
type AnswerType string

const (
	AnswerScale  AnswerType = "scale"  // value "1".."5" sets Axis directly
	AnswerChoice AnswerType = "choice" // one option; option effects applied
	AnswerMulti  AnswerType = "multi"  // several options; each adds to list Axis
)

// Effect is a deterministic change applied to a profile.
type Effect struct {
	Axis  Axis   `json:"axis"`
	Set   int    `json:"set,omitempty"`   // for scalar axes
	Add   string `json:"add,omitempty"`   // for list axes
	Min   int64  `json:"min,omitempty"`   // for budget (minor units)
	Max   int64  `json:"max,omitempty"`   // for budget (minor units)
	Value string `json:"value,omitempty"` // for currency
}

// Option of a choice/multi question.
type Option struct {
	Value   string   `json:"value"`
	LabelRU string   `json:"label_ru"`
	LabelEN string   `json:"label_en"`
	Effects []Effect `json:"-"`
}

// Question of the taste survey.
type Question struct {
	ID      string     `json:"id"`
	Order   int        `json:"order"`
	TextRU  string     `json:"text_ru"`
	TextEN  string     `json:"text_en"`
	Type    AnswerType `json:"type"`
	Axis    Axis       `json:"axis"`
	Options []Option   `json:"options,omitempty"`
}

// SurveyAnswer is a user's answer to one question.
type SurveyAnswer struct {
	QuestionID string    `json:"question_id"`
	Values     []string  `json:"values"`
	AnsweredAt time.Time `json:"answered_at"`
}

// ApplyAnswer validates the answer against the question and updates the profile deterministically.
// Re-answering a question overrides scalar axes and adds to lists (lists are de-duplicated).
func ApplyAnswer(p *TasteProfile, q Question, a SurveyAnswer) error {
	if len(a.Values) == 0 {
		return Invalid("values", "answer to %s is empty", q.ID)
	}
	switch q.Type {
	case AnswerScale:
		if len(a.Values) != 1 {
			return Invalid("values", "question %s expects one value", q.ID)
		}
		v, ok := parseScale(a.Values[0])
		if !ok {
			return Invalid("values", "question %s expects 1..5", q.ID)
		}
		applyEffect(p, Effect{Axis: q.Axis, Set: v})
	case AnswerChoice, AnswerMulti:
		if q.Type == AnswerChoice && len(a.Values) != 1 {
			return Invalid("values", "question %s expects one option", q.ID)
		}
		for _, val := range a.Values {
			opt, ok := findOption(q, val)
			if !ok {
				return Invalid("values", "unknown option %q for question %s", val, q.ID)
			}
			for _, e := range opt.Effects {
				applyEffect(p, e)
			}
		}
	default:
		return Invalid("question_id", "unsupported question type")
	}
	p.Version++
	p.AnsweredQuestion++
	return nil
}

func parseScale(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 1 || s[0] < '1' || s[0] > '5' {
		return 0, false
	}
	return int(s[0] - '0'), true
}

func findOption(q Question, v string) (Option, bool) {
	for _, o := range q.Options {
		if o.Value == v {
			return o, true
		}
	}
	return Option{}, false
}

func applyEffect(p *TasteProfile, e Effect) {
	switch e.Axis {
	case AxisAcidity:
		p.Acidity = e.Set
	case AxisTannin:
		p.Tannin = e.Set
	case AxisBody:
		p.Body = e.Set
	case AxisSweetness:
		p.Sweetness = e.Set
	case AxisFruitDark:
		p.FruitDark = e.Set
	case AxisFruitRed:
		p.FruitRed = e.Set
	case AxisOak:
		p.Oak = e.Set
	case AxisSpice:
		p.SpiceTolerance = e.Set
	case AxisLikedGrapes:
		p.LikedGrapes = addUnique(p.LikedGrapes, e.Add)
		p.RejectedGrapes = removeGrape(p.RejectedGrapes, e.Add)
	case AxisRejected:
		p.RejectedGrapes = addUnique(p.RejectedGrapes, e.Add)
		p.LikedGrapes = removeGrape(p.LikedGrapes, e.Add)
	case AxisRegions:
		p.Regions = addUnique(p.Regions, e.Add)
	case AxisColorsPrefer:
		if !slices.Contains(p.Colors, Color(e.Add)) {
			p.Colors = append(p.Colors, Color(e.Add))
		}
	case AxisBudget:
		p.BudgetMinMinor, p.BudgetMaxMinor = e.Min, e.Max
		if e.Value != "" {
			p.BudgetCurrency = e.Value
		}
	}
}

func addUnique(list []string, v string) []string {
	if v == "" || containsGrape(list, v) {
		return list
	}
	return append(list, v)
}

func removeGrape(list []string, v string) []string {
	want := CanonicalGrape(v)
	out := list[:0:0]
	for _, g := range list {
		if CanonicalGrape(g) != want {
			out = append(out, g)
		}
	}
	return out
}

// NextBatch returns up to size unanswered questions in bank order.
func NextBatch(bank []Question, answered map[string]bool, size int) []Question {
	if size <= 0 {
		size = 6
	}
	out := make([]Question, 0, size)
	for _, q := range bank {
		if answered[q.ID] {
			continue
		}
		out = append(out, q)
		if len(out) == size {
			break
		}
	}
	return out
}

// FindQuestion returns the question with id.
func FindQuestion(bank []Question, id string) (Question, bool) {
	for _, q := range bank {
		if q.ID == id {
			return q, true
		}
	}
	return Question{}, false
}
