package commandruntime

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kyokomi/emoji/v2"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/io"
)

// FormatMessageWithEmoji expands aliases and normalizes spacing between emoji prefixes and text.
func FormatMessageWithEmoji(message io.Message) string {
	base := strings.TrimSpace(emoji.Sprint(message.Message))
	emojiPrefix := strings.TrimSpace(emoji.Sprint(message.Emoji))
	if emojiPrefix == "" {
		return NormalizeVisualPrefixSpacing(base)
	}

	if base == "" {
		return emojiPrefix
	}

	return emojiPrefix + " " + base
}

// ActiveDisplayMessage returns the spinner-line text for active status entries.
func ActiveDisplayMessage(message io.Message, normalized string) string {
	if strings.TrimSpace(message.Emoji) != "" {
		return strings.TrimSpace(emoji.Sprint(message.Message))
	}

	if _, content, hasPrefix := SplitVisualPrefix(normalized); hasPrefix && content != "" {
		return content
	}

	return normalized
}

// SplitVisualPrefix splits a non-alphanumeric prefix token from the message body.
func SplitVisualPrefix(message string) (string, string, bool) {
	first, rest, found := strings.Cut(strings.TrimSpace(message), " ")
	if !found || !IsVisualPrefixToken(first) {
		return "", strings.TrimSpace(message), false
	}

	return first, strings.TrimLeft(rest, " "), true
}

// NormalizeVisualPrefixSpacing normalizes spacing after visual prefix tokens.
func NormalizeVisualPrefixSpacing(message string) string {
	prefix, content, hasPrefix := SplitVisualPrefix(message)
	if !hasPrefix {
		return strings.TrimSpace(message)
	}

	if content == "" {
		return prefix
	}

	return prefix + " " + content
}

// IsVisualPrefixToken reports whether token should be treated as a visual (emoji/symbol) prefix.
func IsVisualPrefixToken(token string) bool {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return false
	}

	firstRune, _ := utf8.DecodeRuneInString(trimmed)
	if firstRune == utf8.RuneError || unicode.IsLetter(firstRune) || unicode.IsNumber(firstRune) {
		return false
	}

	if firstRune == '[' || firstRune == '(' || firstRune == '{' {
		return false
	}

	return unicode.Is(unicode.So, firstRune) ||
		unicode.Is(unicode.Sk, firstRune) ||
		unicode.Is(unicode.Sm, firstRune) ||
		unicode.Is(unicode.Sc, firstRune)
}

// EventEmoji resolves the displayed emoji for a status event.
func EventEmoji(event StatusEvent) string {
	if strings.TrimSpace(string(event.Emoji)) != "" {
		return string(event.Emoji)
	}

	if event.Level == OutputLevelVerbose || event.Level == OutputLevelDebug {
		return string(DefaultEmojiForLevel(event.Level))
	}

	return ""
}

// IsErrorMessage reports whether message content should be rendered with error severity.
func IsErrorMessage(message string) bool {
	lower := strings.ToLower(message)
	return strings.HasPrefix(message, "💥") || strings.Contains(lower, "error")
}

// IsWarningMessage reports whether message content should be rendered with warning severity.
func IsWarningMessage(message string) bool {
	lower := strings.ToLower(message)
	return strings.HasPrefix(message, "⚠") || strings.Contains(lower, "warning")
}

// FormatCommandError returns user-facing command error text based on debug mode.
func FormatCommandError(err error, debug bool) string {
	if err == nil {
		return ""
	}
	if debug {
		return fmt.Sprintf("%v", err)
	}
	return fmt.Sprintf("%s (rerun with --debug for more details)", SummarizeCommandError(err))
}

// SummarizeCommandError returns the top-level summary for user-facing errors.
func SummarizeCommandError(err error) string {
	if err == nil {
		return ""
	}

	var exitErr *fterrors.ExitCodeError
	if errors.As(err, &exitErr) {
		return exitErr.Message
	}

	return err.Error()
}
