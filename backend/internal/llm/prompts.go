package llm

import (
	_ "embed"
)

//go:embed prompts/assistant-ru.md
var assistantRU string

//go:embed prompts/assistant-en.md
var assistantEN string

// SystemPrompt returns the assistant system prompt for a locale ("ru" default).
func SystemPrompt(locale string) string {
	if locale == "en" {
		return assistantEN
	}
	return assistantRU
}
