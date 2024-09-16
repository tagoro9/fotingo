package commandruntime

import "regexp"

var (
	debugSecretValuePattern = regexp.MustCompile(`(?i)(token|password|secret|authorization|api[_-]?key)\s*[:=]\s*([^\s,;]+)`)
	debugBearerPattern      = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]+`)
)

// RedactSensitiveDebug redacts common credential patterns from debug messages.
func RedactSensitiveDebug(message string) string {
	redacted := debugSecretValuePattern.ReplaceAllString(message, `$1=<redacted>`)
	redacted = debugBearerPattern.ReplaceAllString(redacted, "bearer <redacted>")
	return redacted
}
