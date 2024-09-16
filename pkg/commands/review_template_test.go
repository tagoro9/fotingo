package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveReviewTemplate_PrefersFotingoTemplate(t *testing.T) {
	repoDir := setupTemplateRepo(t)
	writeTemplateFile(t, repoDir, filepath.Join(".github", "pull_request_template.md"), "standard template")
	writeTemplateFile(t, repoDir, filepath.Join(".github", "PULL_REQUEST_TEMPLATE", "fotingo.md"), "fotingo template")

	withWorkingDirectory(t, repoDir, func() {
		assert.Equal(t, "fotingo template", resolveReviewTemplate())
	})
}

func TestResolveReviewTemplate_UsesGitHubStandardLocations(t *testing.T) {
	repoDir := setupTemplateRepo(t)
	writeTemplateFile(t, repoDir, filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"), "github root template")

	withWorkingDirectory(t, repoDir, func() {
		assert.Equal(t, "github root template", resolveReviewTemplate())
	})
}

func TestResolveReviewTemplate_FindsTemplateFromNestedDirectory(t *testing.T) {
	repoDir := setupTemplateRepo(t)
	writeTemplateFile(t, repoDir, filepath.Join("docs", "pull_request_template.md"), "docs template")
	nestedDir := filepath.Join(repoDir, "pkg", "commands")
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))

	withWorkingDirectory(t, nestedDir, func() {
		assert.Equal(t, "docs template", resolveReviewTemplate())
	})
}

func TestResolveReviewTemplate_FallsBackToDefaultTemplate(t *testing.T) {
	repoDir := setupTemplateRepo(t)

	withWorkingDirectory(t, repoDir, func() {
		assert.Equal(t, defaultPRTemplate, resolveReviewTemplate())
	})
}

func setupTemplateRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(repoDir, ".git"), 0o755))

	return repoDir
}

func writeTemplateFile(t *testing.T, repoDir, relativePath, content string) {
	t.Helper()

	fullPath := filepath.Join(repoDir, relativePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o600))
}

func withWorkingDirectory(t *testing.T, directory string, fn func()) {
	t.Helper()

	originalDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(directory))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalDir))
	})

	fn()
}
