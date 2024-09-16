package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	badger "github.com/dgraph-io/badger/v4"
)

const (
	defaultFolderName = ".config/fotingo"
	defaultDBName     = "cache.db"
)

// Store is a persistent cache interface for internal consumers.
type Store interface {
	Get(key string, destination any) (bool, error)
	SetWithTTL(key string, value any, ttl time.Duration) error
	Delete(key string) error
	List(prefix string) ([]Entry, error)
	Clear() error
	Close() error
}

// Entry represents a cache entry with metadata.
type Entry struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value,omitempty"`
	ExpiresAt *time.Time      `json:"expiresAt,omitempty"`
	SizeBytes int             `json:"sizeBytes"`
}

type options struct {
	path   string
	logger badger.Logger
}

// Option configures cache construction.
type Option func(*options)

// WithPath sets the database path.
func WithPath(path string) Option {
	return func(opts *options) {
		opts.path = path
	}
}

// WithLogger overrides the default badger logger.
func WithLogger(logger badger.Logger) Option {
	return func(opts *options) {
		opts.logger = logger
	}
}

// BadgerStore is a badger-backed persistent cache.
type BadgerStore struct {
	db *badger.DB
}

// DefaultPath returns the default cache path.
func DefaultPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}

	return filepath.Join(homeDir, defaultFolderName, defaultDBName), nil
}

// NewDefault constructs a cache store using the default path.
func NewDefault() (*BadgerStore, error) {
	return New()
}

// New constructs a badger-backed cache store.
func New(opts ...Option) (*BadgerStore, error) {
	resolvedPath, err := DefaultPath()
	if err != nil {
		return nil, err
	}

	settings := options{
		path:   resolvedPath,
		logger: nil,
	}
	for _, opt := range opts {
		opt(&settings)
	}

	if settings.path == "" {
		return nil, errors.New("cache path cannot be empty")
	}

	if err := os.MkdirAll(settings.path, 0700); err != nil {
		return nil, fmt.Errorf("failed to create cache directory %q: %w", settings.path, err)
	}

	badgerOptions := badger.DefaultOptions(settings.path).
		WithDir(settings.path).
		WithValueDir(settings.path).
		WithLogger(settings.logger)

	db, err := badger.Open(badgerOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to open cache database at %q: %w", settings.path, err)
	}

	return &BadgerStore{db: db}, nil
}

// Get returns true when the key exists and destination was populated.
func (s *BadgerStore) Get(key string, destination any) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("cache store is not initialized")
	}
	if destination == nil {
		return false, errors.New("destination cannot be nil")
	}

	var payload []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		payload, err = item.ValueCopy(nil)
		return err
	})
	if errors.Is(err, badger.ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read cache key %q: %w", key, err)
	}

	if err := json.Unmarshal(payload, destination); err != nil {
		return false, fmt.Errorf("failed to decode cache value for key %q: %w", key, err)
	}

	return true, nil
}

// SetWithTTL stores a value with an optional TTL.
func (s *BadgerStore) SetWithTTL(key string, value any, ttl time.Duration) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is not initialized")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to encode cache value for key %q: %w", key, err)
	}

	entry := badger.NewEntry([]byte(key), payload)
	if ttl > 0 {
		entry = entry.WithTTL(ttl)
	}

	if err := s.db.Update(func(txn *badger.Txn) error {
		return txn.SetEntry(entry)
	}); err != nil {
		return fmt.Errorf("failed to store cache key %q: %w", key, err)
	}

	return nil
}

// Delete removes a key from cache.
func (s *BadgerStore) Delete(key string) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is not initialized")
	}
	if err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	}); err != nil {
		return fmt.Errorf("failed to delete cache key %q: %w", key, err)
	}

	return nil
}

// List returns cache entries matching the provided prefix.
// When prefix is empty, all entries are returned.
func (s *BadgerStore) List(prefix string) ([]Entry, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("cache store is not initialized")
	}
	entries := make([]Entry, 0)

	if err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true

		iterator := txn.NewIterator(opts)
		defer iterator.Close()

		prefixBytes := []byte(prefix)
		for iterator.Seek(prefixBytes); iterator.ValidForPrefix(prefixBytes); iterator.Next() {
			item := iterator.Item()
			key := string(item.Key())

			value, err := item.ValueCopy(nil)
			if err != nil {
				return fmt.Errorf("failed to copy value for key %q: %w", key, err)
			}

			entry := Entry{
				Key:       key,
				Value:     json.RawMessage(value),
				SizeBytes: len(value),
			}

			if expiresAtUnix := item.ExpiresAt(); expiresAtUnix > 0 {
				expiresAt := time.Unix(int64(expiresAtUnix), 0).UTC()
				entry.ExpiresAt = &expiresAt
			}

			entries = append(entries, entry)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list cache entries: %w", err)
	}

	return entries, nil
}

// Clear removes all entries from the cache.
func (s *BadgerStore) Clear() error {
	if s == nil || s.db == nil {
		return errors.New("cache store is not initialized")
	}
	if err := s.db.DropAll(); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}
	return nil
}

// Close closes the badger database.
func (s *BadgerStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// GetOrLoad loads data from cache and falls back to loader on miss.
func GetOrLoad[T any](store Store, key string, ttl time.Duration, loader func() (T, error)) (T, error) {
	var zero T

	if store != nil {
		var cached T
		hit, err := store.Get(key, &cached)
		if err != nil {
			return zero, err
		}
		if hit {
			return cached, nil
		}
	}

	loaded, err := loader()
	if err != nil {
		return zero, err
	}

	if store != nil {
		if err := store.SetWithTTL(key, loaded, ttl); err != nil {
			return zero, err
		}
	}

	return loaded, nil
}
