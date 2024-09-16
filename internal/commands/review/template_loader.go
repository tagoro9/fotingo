package review

import (
	"errors"
	"os"
	"path/filepath"
)

// TemplateSearchOrder defines repository locations checked for pull request templates.
var TemplateSearchOrder = []string{
	filepath.Join(".github", "PULL_REQUEST_TEMPLATE", "fotingo.md"),
	filepath.Join("docs", "PULL_REQUEST_TEMPLATE", "fotingo.md"),
	filepath.Join("PULL_REQUEST_TEMPLATE", "fotingo.md"),
	filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"),
	filepath.Join(".github", "pull_request_template.md"),
	filepath.Join("docs", "PULL_REQUEST_TEMPLATE.md"),
	filepath.Join("docs", "pull_request_template.md"),
	"PULL_REQUEST_TEMPLATE.md",
	"pull_request_template.md",
}

// ResolveTemplate resolves the repository template, falling back to defaultTemplate.
func ResolveTemplate(defaultTemplate string) string {
	repositoryRoot, err := FindRepositoryRoot()
	if err != nil {
		return defaultTemplate
	}

	templateContent, found := LoadRepositoryTemplate(repositoryRoot, TemplateSearchOrder)
	if !found {
		return defaultTemplate
	}

	return templateContent
}

// LoadRepositoryTemplate loads the first template file found in searchOrder.
func LoadRepositoryTemplate(repositoryRoot string, searchOrder []string) (string, bool) {
	for _, relativePath := range searchOrder {
		templatePath := filepath.Join(repositoryRoot, relativePath)
		templateContent, err := os.ReadFile(templatePath)
		if err == nil {
			return string(templateContent), true
		}

		if errors.Is(err, os.ErrNotExist) {
			continue
		}
	}

	return "", false
}

// FindRepositoryRoot walks parent directories until a git repository root is found.
func FindRepositoryRoot() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		gitPath := filepath.Join(currentDir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return currentDir, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return "", os.ErrNotExist
		}
		currentDir = parentDir
	}
}
