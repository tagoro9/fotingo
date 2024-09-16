package commandruntime

import (
	"fmt"
	"io"
	"time"
)

// ShouldPrintDoneFooter reports whether a command should print the completion footer.
func ShouldPrintDoneFooter(
	suppressOutput bool,
	invocationShellCompletion bool,
	shellCompletionRequest bool,
) bool {
	return !suppressOutput && !invocationShellCompletion && !shellCompletionRequest
}

// PrintDoneFooter writes the completion footer to the provided writer.
func PrintDoneFooter(w io.Writer, start time.Time) {
	if w == nil {
		return
	}

	_, _ = fmt.Fprintf(w, "🎉 Done in %s\n", HumanizeDuration(time.Since(start)))
}

// HumanizeDuration formats durations using concise CLI-friendly output.
func HumanizeDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}

	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Round(time.Millisecond)/time.Millisecond)
	}

	if duration < time.Minute {
		seconds := duration.Seconds()
		return fmt.Sprintf("%.1fs", seconds)
	}

	if duration < time.Hour {
		minutes := int(duration / time.Minute)
		seconds := int((duration % time.Minute) / time.Second)
		if seconds == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}

	hours := int(duration / time.Hour)
	minutes := int((duration % time.Hour) / time.Minute)
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
