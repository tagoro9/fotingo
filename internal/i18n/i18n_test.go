package i18n

import "testing"

type fakeConfig struct {
	locale string
}

func (f fakeConfig) GetString(key string) string {
	if key == "locale" {
		return f.locale
	}
	return ""
}

func TestDefaultLocale(t *testing.T) {
	l := New("")
	if got := l.Locale(); got != DefaultLocale {
		t.Fatalf("expected locale %q, got %q", DefaultLocale, got)
	}
}

func TestUnsupportedLocaleFallsBackToEnglish(t *testing.T) {
	l := New("es")
	if got := l.Locale(); got != DefaultLocale {
		t.Fatalf("expected fallback locale %q, got %q", DefaultLocale, got)
	}
	if got := l.T(RootUse); got != "fotingo" {
		t.Fatalf("unexpected translation: %q", got)
	}
}

func TestMissingKeyFallback(t *testing.T) {
	l := New("en")
	missing := Key("does.not.exist")
	if got := l.T(missing); got != "[[missing:does.not.exist]]" {
		t.Fatalf("unexpected missing key fallback: %q", got)
	}
}

func TestNewFromConfig(t *testing.T) {
	l := NewFromConfig(fakeConfig{locale: "en"})
	if got := l.Locale(); got != "en" {
		t.Fatalf("expected locale en, got %q", got)
	}
}
