package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchLatestReleaseTag_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/tagoro9/fotingo/releases/latest", r.URL.Path)
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		assert.Equal(t, "fotingo/version-checker", r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
	}))
	t.Cleanup(server.Close)

	origBaseURL := latestReleaseAPIBaseURL
	latestReleaseAPIBaseURL = server.URL
	t.Cleanup(func() {
		latestReleaseAPIBaseURL = origBaseURL
	})

	tag, err := FetchLatestReleaseTag(context.Background(), server.Client(), "tagoro9", "fotingo")
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", tag)
}

func TestFetchLatestReleaseTag_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	origBaseURL := latestReleaseAPIBaseURL
	latestReleaseAPIBaseURL = server.URL
	t.Cleanup(func() {
		latestReleaseAPIBaseURL = origBaseURL
	})

	_, err := FetchLatestReleaseTag(context.Background(), server.Client(), "tagoro9", "fotingo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "latest release request failed")
}
