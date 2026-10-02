package llm

import (
	"context"
	"errors"
)

// Label recognition (photo of a label → draft wine card) — OUT OF MVP SCOPE.
//
// Planned approach: send the image to a multimodal model of the SAME OpenAI-compatible provider
// (image as a data URL in the chat message) and ask for strict JSON matching LabelDraft.
// No separate OCR vendors. The draft is always shown to the user for confirmation; it is
// never saved automatically (source = "llm").

// ErrNotImplemented is returned by stubs of features outside the MVP.
var ErrNotImplemented = errors.New("not implemented")

// LabelDraft is a best-effort wine card extracted from a label photo.
// Pointer fields are nil when the model is not sure.
type LabelDraft struct {
	Name        string   `json:"name"`
	Producer    string   `json:"producer"`
	Country     string   `json:"country"`
	Region      string   `json:"region"`
	Appellation string   `json:"appellation"`
	Grapes      []string `json:"grapes"`
	Vintage     *int     `json:"vintage"`
	Color       string   `json:"color"`
	Sweetness   string   `json:"sweetness"`
	ABV         *float64 `json:"abv"`
	Confidence  float64  `json:"confidence"` // 0..1, overall
	RawText     string   `json:"raw_text"`   // text the model read from the label
}

// LabelRecognizer turns a label photo into a draft wine card.
type LabelRecognizer interface {
	Recognize(ctx context.Context, image []byte, mime string) (LabelDraft, error)
}

// NotImplementedRecognizer is the MVP stub: always returns ErrNotImplemented.
type NotImplementedRecognizer struct{}

// Recognize implements LabelRecognizer.
func (NotImplementedRecognizer) Recognize(context.Context, []byte, string) (LabelDraft, error) {
	return LabelDraft{}, ErrNotImplemented
}
