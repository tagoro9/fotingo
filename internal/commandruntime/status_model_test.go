package commandruntime

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	fio "github.com/tagoro9/fotingo/internal/io"
)

func TestStatusModel_StatusOperationUpdateReusesLine(t *testing.T) {
	m := NewStatusModel(StatusModelOptions{})

	updated, _ := m.Update(UpdateStatus(EncodeStatusEvent(
		StatusEventKindStart,
		"open-browser",
		"opening pr",
		OutputLevelInfo,
		LogEmojiBrowser,
	)))
	model := updated.(StatusModel)
	require.Len(t, model.messages, 1)
	assert.Equal(t, "opening pr", model.messages[0].Message)
	assert.Equal(t, fio.MessageTypeStatus, model.messages[0].Type)

	updated, _ = model.Update(UpdateStatus(EncodeStatusEvent(
		StatusEventKindUpdate,
		"open-browser",
		"opening https://github.com/o/r/pull/1",
		OutputLevelInfo,
		LogEmojiBrowser,
	)))
	model = updated.(StatusModel)
	require.Len(t, model.messages, 1)
	assert.Equal(t, "opening https://github.com/o/r/pull/1", model.messages[0].Message)

	updated, _ = model.Update(UpdateStatus(EncodeStatusEvent(
		StatusEventKindSuccess,
		"open-browser",
		"opened",
		OutputLevelInfo,
		LogEmojiBrowser,
	)))
	model = updated.(StatusModel)
	require.Len(t, model.messages, 1)
	assert.Equal(t, "opened", model.messages[0].Message)
	assert.Equal(t, fio.MessageTypeInfo, model.messages[0].Type)
}

func TestStatusModel_View_EmojiAndActiveStatus(t *testing.T) {
	m := NewStatusModel(StatusModelOptions{})
	m.messages = []fio.Message{{Message: ":earth_africa: Opening browser", Type: fio.MessageTypeInfo}}
	m.done = true

	view := m.View()
	assert.Contains(t, viewString(view), "🌍")
	assert.NotContains(t, viewString(view), ":earth_africa:")

	m = NewStatusModel(StatusModelOptions{})
	m.messages = []fio.Message{{Emoji: string(LogEmojiBrowser), Message: "Opening browser", Type: fio.MessageTypeStatus}}
	m.done = false
	view = m.View()
	assert.Contains(t, viewString(view), "Opening browser")
	assert.NotContains(t, viewString(view), "🌍")
}

func TestStatusModel_UpdateCorePaths(t *testing.T) {
	m := NewStatusModel(StatusModelOptions{})

	updated, _ := m.Update(UpdateStatus("processing..."))
	model := updated.(StatusModel)
	require.Len(t, model.messages, 1)
	assert.Equal(t, "processing...", model.messages[0].Message)
	assert.Equal(t, fio.MessageTypeStatus, model.messages[0].Type)

	updated, cmd := model.Update(FinishProcess())
	assert.NotNil(t, cmd)
	assert.True(t, updated.(StatusModel).done)

	updated, cmd = model.Update(ctrlKey('c'))
	assert.NotNil(t, cmd)
	assert.Equal(t, model, updated)
}

func TestStatusModel_UpdateSpinnerTickAndAccumulate(t *testing.T) {
	m := NewStatusModel(StatusModelOptions{})
	updated, cmd := m.Update(spinner.TickMsg{})
	assert.NotNil(t, cmd)
	model := updated.(StatusModel)
	assert.False(t, model.done)

	modelAny, _ := m.Update(UpdateStatus("a"))
	modelAny, _ = modelAny.Update(UpdateStatus("b"))
	modelAny, _ = modelAny.Update(UpdateStatus("c"))
	updatedModel := modelAny.(StatusModel)
	require.Len(t, updatedModel.messages, 3)
	assert.Equal(t, "a", updatedModel.messages[0].Message)
	assert.Equal(t, "b", updatedModel.messages[1].Message)
	assert.Equal(t, "c", updatedModel.messages[2].Message)
}

func TestStatusModel_ViewSuppressOutput(t *testing.T) {
	m := NewStatusModel(StatusModelOptions{
		SuppressOutput: func() bool { return true },
	})
	updated, _ := m.Update(UpdateStatus("hello"))
	assert.Equal(t, "", viewString(updated.(StatusModel).View()))
}

func TestStatusModel_ViewSingleDoneMessage(t *testing.T) {
	m := NewStatusModel(StatusModelOptions{})
	modelAny, _ := m.Update(UpdateStatus("one"))
	model := modelAny.(StatusModel)
	model.demoteActiveMessages()
	model.done = true
	view := model.View()
	assert.True(t, strings.Contains(viewString(view), "one"))
}
