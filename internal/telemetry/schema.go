package telemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/posthog/posthog-go"
)

var (
	safeKeyPattern         = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
	safeEnumPattern        = regexp.MustCompile(`^[a-z0-9_./@:-]{1,64}$`)
	safeFingerprintPattern = regexp.MustCompile(`^[a-f0-9]{8,64}$`)
	safeVersionPattern     = regexp.MustCompile(`^v?[0-9][0-9a-zA-Z.+-]{0,31}$`)
)

var allowedPropertiesByEvent = map[string]map[string]struct{}{
	EventCommandStarted: {
		"event_schema_version":   {},
		"persona":                {},
		"invocation_mode":        {},
		"version":                {},
		"platform":               {},
		"os":                     {},
		"arch":                   {},
		"command_name":           {},
		"command_path":           {},
		"command_schema_version": {},
		"global_flags":           {},
		"has_branch_override":    {},
		"option_flags":           {},
		"option_counts":          {},
		"option_enums":           {},
	},
	EventCommandCompleted: {
		"event_schema_version":   {},
		"persona":                {},
		"invocation_mode":        {},
		"version":                {},
		"platform":               {},
		"os":                     {},
		"arch":                   {},
		"command_name":           {},
		"command_path":           {},
		"command_schema_version": {},
		"global_flags":           {},
		"has_branch_override":    {},
		"option_flags":           {},
		"option_counts":          {},
		"option_enums":           {},
		"duration_ms":            {},
		"exit_code":              {},
	},
	EventCommandError: {
		"event_schema_version":   {},
		"persona":                {},
		"invocation_mode":        {},
		"version":                {},
		"platform":               {},
		"os":                     {},
		"arch":                   {},
		"command_name":           {},
		"command_path":           {},
		"command_schema_version": {},
		"global_flags":           {},
		"has_branch_override":    {},
		"option_flags":           {},
		"option_counts":          {},
		"option_enums":           {},
		"duration_ms":            {},
		"exit_code":              {},
		"error_family":           {},
		"error_fingerprint":      {},
	},
	EventCommandCrashed: {
		"event_schema_version": {},
		"persona":              {},
		"invocation_mode":      {},
		"version":              {},
		"platform":             {},
		"os":                   {},
		"arch":                 {},
		"command_name":         {},
		"command_path":         {},
		"duration_ms":          {},
		"exit_code":            {},
		"panic_type":           {},
		"crash_fingerprint":    {},
		"top_frame":            {},
	},
	EventIntegrationCall: {
		"event_schema_version": {},
		"persona":              {},
		"invocation_mode":      {},
		"version":              {},
		"platform":             {},
		"os":                   {},
		"arch":                 {},
		"command_name":         {},
		"command_path":         {},
		"service":              {},
		"operation":            {},
		"duration_ms":          {},
		"success":              {},
		"retry_count":          {},
		"cache_hit":            {},
		"status_code_bucket":   {},
	},
	EventUpdateBannerShown: {
		"event_schema_version": {},
		"persona":              {},
		"invocation_mode":      {},
		"version":              {},
		"platform":             {},
		"os":                   {},
		"arch":                 {},
		"current_version":      {},
		"latest_version":       {},
		"trigger":              {},
	},
}

var requiredPropertiesByEvent = map[string][]string{
	EventCommandStarted: {
		"event_schema_version", "persona", "invocation_mode", "version", "platform",
		"command_name", "command_schema_version", "global_flags", "option_flags", "option_counts", "option_enums",
	},
	EventCommandCompleted: {
		"event_schema_version", "persona", "invocation_mode", "version", "platform",
		"command_name", "command_schema_version", "duration_ms", "exit_code",
	},
	EventCommandError: {
		"event_schema_version", "persona", "invocation_mode", "version", "platform",
		"command_name", "command_schema_version", "duration_ms", "exit_code", "error_family", "error_fingerprint",
	},
	EventCommandCrashed: {
		"event_schema_version", "persona", "invocation_mode", "version", "platform",
		"command_name", "duration_ms", "exit_code", "panic_type", "crash_fingerprint",
	},
	EventIntegrationCall: {
		"event_schema_version", "persona", "invocation_mode", "version", "platform",
		"service", "operation", "duration_ms", "success", "retry_count", "cache_hit", "status_code_bucket",
	},
	EventUpdateBannerShown: {
		"event_schema_version", "persona", "invocation_mode", "version", "platform",
		"current_version", "latest_version", "trigger",
	},
}

func sanitizeAndValidateProperties(event string, props posthog.Properties) posthog.Properties {
	allowed, ok := allowedPropertiesByEvent[event]
	if !ok {
		return nil
	}

	sanitized := posthog.NewProperties()
	for key, value := range props {
		if _, allowedKey := allowed[key]; !allowedKey {
			continue
		}
		sanitized[key] = value
	}

	for _, required := range requiredPropertiesByEvent[event] {
		if _, present := sanitized[required]; !present {
			return nil
		}
	}

	if event == EventCommandStarted {
		if _, exists := sanitized["duration_ms"]; exists {
			return nil
		}
		if _, exists := sanitized["exit_code"]; exists {
			return nil
		}
	}

	return sanitized
}

func sanitizeBoolMap(source map[string]bool, maxKeys int) map[string]bool {
	result := make(map[string]bool)
	if maxKeys <= 0 {
		return result
	}
	count := 0
	for key, value := range source {
		safeKey := normalizeKey(key)
		if safeKey == "" {
			continue
		}
		result[safeKey] = value
		count++
		if count >= maxKeys {
			break
		}
	}
	return result
}

func sanitizeIntMap(source map[string]int, maxKeys int) map[string]int {
	result := make(map[string]int)
	if maxKeys <= 0 {
		return result
	}
	count := 0
	for key, value := range source {
		safeKey := normalizeKey(key)
		if safeKey == "" {
			continue
		}
		result[safeKey] = maxInt(value, 0)
		count++
		if count >= maxKeys {
			break
		}
	}
	return result
}

func sanitizeEnumMap(source map[string]string, maxKeys int) map[string]string {
	result := make(map[string]string)
	if maxKeys <= 0 {
		return result
	}
	count := 0
	for key, value := range source {
		safeKey := normalizeKey(key)
		safeValue := normalizeEnum(value)
		if safeKey == "" || safeValue == "" {
			continue
		}
		result[safeKey] = safeValue
		count++
		if count >= maxKeys {
			break
		}
	}
	return result
}

func normalizeEnum(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return ""
	}
	if len(normalized) > 64 {
		normalized = normalized[:64]
	}
	if !safeEnumPattern.MatchString(normalized) {
		return ""
	}
	return normalized
}

func normalizeKey(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return ""
	}
	if len(normalized) > 64 {
		normalized = normalized[:64]
	}
	if !safeKeyPattern.MatchString(normalized) {
		return ""
	}
	return normalized
}

func nonEmptyOrDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return strings.TrimSpace(fallback)
	}
	return value
}

func sanitizeSchemaVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown@1.0.0"
	}
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func sanitizeFingerprint(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	if safeFingerprintPattern.MatchString(value) {
		return value
	}
	return fingerprint(value)
}

func sanitizeTopFrame(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	if len(value) > 120 {
		value = value[:120]
	}
	return value
}

func sanitizeVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > 32 {
		value = value[:32]
	}
	if safeVersionPattern.MatchString(value) {
		return value
	}
	return ""
}

func fingerprint(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:8])
}

func maxInt(value int, minValue int) int {
	if value < minValue {
		return minValue
	}
	return value
}
