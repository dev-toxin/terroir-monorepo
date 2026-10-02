package llm

import (
	"context"
	"errors"
	"testing"
)

func TestNotImplementedRecognizer(t *testing.T) {
	var r LabelRecognizer = NotImplementedRecognizer{}
	if _, err := r.Recognize(context.Background(), []byte{1}, "image/jpeg"); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("got %v", err)
	}
}
