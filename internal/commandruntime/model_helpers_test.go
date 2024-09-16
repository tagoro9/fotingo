package commandruntime

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	fterrors "github.com/tagoro9/fotingo/internal/errors"
	"github.com/tagoro9/fotingo/internal/io"
)

func TestFormatMessageWithEmojiAndActiveDisplay(t *testing.T) {
	withEmoji := FormatMessageWithEmoji(io.Message{Emoji: ":rocket:", Message: "launching"})
	assert.Equal(t, "🚀 launching", withEmoji)

	withoutEmoji := FormatMessageWithEmoji(io.Message{Message: "⚙  loading"})
	assert.Equal(t, "⚙ loading", withoutEmoji)

	active := ActiveDisplayMessage(io.Message{Message: "⚙ loading"}, withoutEmoji)
	assert.Equal(t, "loading", active)
}

func TestSplitAndNormalizeVisualPrefix(t *testing.T) {
	prefix, content, ok := SplitVisualPrefix("⚙   loading")
	assert.True(t, ok)
	assert.Equal(t, "⚙", prefix)
	assert.Equal(t, "loading", content)
	assert.Equal(t, "⚙ loading", NormalizeVisualPrefixSpacing("⚙   loading"))
}

func TestIsVisualPrefixToken(t *testing.T) {
	assert.True(t, IsVisualPrefixToken("⚙"))
	assert.False(t, IsVisualPrefixToken("abc"))
	assert.False(t, IsVisualPrefixToken("[warn]"))
}

func TestEventEmoji(t *testing.T) {
	assert.Equal(t, "⚠", EventEmoji(StatusEvent{Emoji: LogEmojiWarning}))
	assert.Equal(t, string(DefaultEmojiForLevel(OutputLevelVerbose)), EventEmoji(StatusEvent{Level: OutputLevelVerbose}))
	assert.Equal(t, "", EventEmoji(StatusEvent{Level: OutputLevelInfo}))
}

func TestMessageSeverityDetection(t *testing.T) {
	assert.True(t, IsErrorMessage("💥 boom"))
	assert.True(t, IsWarningMessage("⚠ careful"))
	assert.False(t, IsErrorMessage("all good"))
	assert.False(t, IsWarningMessage("all good"))
}

func TestFormatAndSummarizeCommandError(t *testing.T) {
	plain := errors.New("broken")
	assert.Equal(t, "broken", SummarizeCommandError(plain))
	assert.Contains(t, FormatCommandError(plain, false), "rerun with --debug")
	assert.Equal(t, "broken", FormatCommandError(plain, true))

	exitErr := fterrors.WrapGitHubError("friendly summary", errors.New("low-level details"))
	assert.Equal(t, "friendly summary", SummarizeCommandError(exitErr))
}
