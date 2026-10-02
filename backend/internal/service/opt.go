package service

import (
	"bytes"
	"encoding/json"
)

// Opt is a PATCH field that distinguishes "absent" (Set=false) from "explicit null" (Set=true, Null=true).
// encoding/json leaves Opt untouched when the key is missing, and calls UnmarshalJSON for null too.
type Opt[T any] struct {
	Set   bool
	Null  bool
	Value T
}

// UnmarshalJSON implements json.Unmarshaler.
func (o *Opt[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

// ApplyPtr writes the patch into a nullable field.
func (o Opt[T]) ApplyPtr(dst **T) {
	if !o.Set {
		return
	}
	if o.Null {
		*dst = nil
		return
	}
	v := o.Value
	*dst = &v
}

// Apply writes the patch into a non-nullable field (null is ignored).
func (o Opt[T]) Apply(dst *T) {
	if o.Set && !o.Null {
		*dst = o.Value
	}
}
