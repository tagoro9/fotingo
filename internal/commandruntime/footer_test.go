package commandruntime

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShouldPrintDoneFooter(t *testing.T) {
	assert.True(t, ShouldPrintDoneFooter(false, false, false))
	assert.False(t, ShouldPrintDoneFooter(true, false, false))
	assert.False(t, ShouldPrintDoneFooter(false, true, false))
	assert.False(t, ShouldPrintDoneFooter(false, false, true))
}

func TestHumanizeDuration(t *testing.T) {
	assert.Equal(t, "0ms", HumanizeDuration(-10*time.Millisecond))
	assert.Equal(t, "125ms", HumanizeDuration(125*time.Millisecond))
	assert.Equal(t, "2.5s", HumanizeDuration(2500*time.Millisecond))
	assert.Equal(t, "2m 5s", HumanizeDuration(125*time.Second))
	assert.Equal(t, "1h 5m", HumanizeDuration(65*time.Minute))
}

func TestPrintDoneFooter(t *testing.T) {
	var b bytes.Buffer
	PrintDoneFooter(&b, time.Now().Add(-2500*time.Millisecond))
	assert.Contains(t, b.String(), "🎉 Done in ")
}
