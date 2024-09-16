package i18n

import (
	"fmt"
	"sync"
)

const (
	DefaultLocale = "en"
)

// ConfigProvider is the minimal config contract needed to resolve locale.
type ConfigProvider interface {
	GetString(key string) string
}

// Localizer resolves message keys for a locale.
type Localizer struct {
	locale string
}

var (
	globalMu        sync.RWMutex
	globalLocalizer = New(DefaultLocale)
)

// New creates a localizer for the provided locale with fallback to English.
func New(locale string) *Localizer {
	resolved := locale
	if resolved == "" {
		resolved = DefaultLocale
	}
	if _, ok := catalogs[resolved]; !ok {
		resolved = DefaultLocale
	}
	return &Localizer{locale: resolved}
}

// NewFromConfig creates a localizer from config key "locale".
func NewFromConfig(cfg ConfigProvider) *Localizer {
	if cfg == nil {
		return New(DefaultLocale)
	}
	return New(cfg.GetString("locale"))
}

// Locale returns the effective locale for this localizer.
func (l *Localizer) Locale() string {
	if l == nil {
		return DefaultLocale
	}
	return l.locale
}

// T resolves a translated message key.
func (l *Localizer) T(key Key, args ...any) string {
	if l == nil {
		l = New(DefaultLocale)
	}
	catalog := catalogs[l.locale]
	template, ok := catalog[key]
	if !ok {
		return MissingKeyFallback(key)
	}
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}

// MissingKeyFallback returns deterministic fallback text for missing keys.
func MissingKeyFallback(key Key) string {
	return fmt.Sprintf("[[missing:%s]]", key)
}

// SetGlobalLocale updates the process-wide locale used by T.
func SetGlobalLocale(locale string) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLocalizer = New(locale)
}

// T resolves a message using the global localizer.
func T(key Key, args ...any) string {
	globalMu.RLock()
	l := globalLocalizer
	globalMu.RUnlock()
	return l.T(key, args...)
}
