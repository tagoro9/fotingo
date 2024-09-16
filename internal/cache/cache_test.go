package cache

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefault_UsesHomeConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	store, err := NewDefault()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	expectedPath := filepath.Join(home, defaultFolderName, defaultDBName)
	assert.DirExists(t, expectedPath)
}

func TestSetGetDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	store, err := New(WithPath(path))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	type testValue struct {
		Name string `json:"name"`
		Size int    `json:"size"`
	}

	key := "test:key"
	value := testValue{Name: "fotingo", Size: 42}

	require.NoError(t, store.SetWithTTL(key, value, 0))

	var loaded testValue
	hit, err := store.Get(key, &loaded)
	require.NoError(t, err)
	assert.True(t, hit)
	assert.Equal(t, value, loaded)

	require.NoError(t, store.Delete(key))

	hit, err = store.Get(key, &loaded)
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestSetWithTTL_Expires(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	store, err := New(WithPath(path))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	require.NoError(t, store.SetWithTTL("ttl:key", map[string]string{"value": "alive"}, 50*time.Millisecond))
	time.Sleep(120 * time.Millisecond)

	var loaded map[string]string
	hit, err := store.Get("ttl:key", &loaded)
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestGetOrLoad_UsesLoaderOnMissAndCaches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	store, err := New(WithPath(path))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	calls := 0
	loader := func() ([]string, error) {
		calls++
		return []string{"a", "b"}, nil
	}

	first, err := GetOrLoad(store, "load:key", time.Minute, loader)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, first)
	assert.Equal(t, 1, calls)

	second, err := GetOrLoad(store, "load:key", time.Minute, loader)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, second)
	assert.Equal(t, 1, calls)
}

func TestGetOrLoad_NilStore(t *testing.T) {
	calls := 0
	loader := func() (string, error) {
		calls++
		return "loaded", nil
	}

	value, err := GetOrLoad[string](nil, "key", time.Minute, loader)
	require.NoError(t, err)
	assert.Equal(t, "loaded", value)
	assert.Equal(t, 1, calls)
}

func TestList_WithPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	store, err := New(WithPath(path))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	require.NoError(t, store.SetWithTTL("github:labels", map[string]string{"value": "a"}, 0))
	require.NoError(t, store.SetWithTTL("github:users", map[string]string{"value": "b"}, 0))
	require.NoError(t, store.SetWithTTL("jira:types", map[string]string{"value": "c"}, 0))

	entries, err := store.List("github:")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "github:labels", entries[0].Key)
	assert.Equal(t, "github:users", entries[1].Key)
}

func TestClear_RemovesAllEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	store, err := New(WithPath(path))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	require.NoError(t, store.SetWithTTL("key:a", map[string]string{"value": "a"}, 0))
	require.NoError(t, store.SetWithTTL("key:b", map[string]string{"value": "b"}, 0))

	require.NoError(t, store.Clear())

	entries, err := store.List("")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestNilStoreOperations_ReturnInitializedErrors(t *testing.T) {
	var store *BadgerStore

	var destination map[string]string
	hit, err := store.Get("key", &destination)
	require.Error(t, err)
	assert.False(t, hit)
	assert.Contains(t, err.Error(), "not initialized")

	err = store.SetWithTTL("key", map[string]string{"value": "a"}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	err = store.Delete("key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	entries, err := store.List("")
	require.Error(t, err)
	assert.Nil(t, entries)
	assert.Contains(t, err.Error(), "not initialized")

	err = store.Clear()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}
