package jira

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"

	"github.com/tagoro9/fotingo/internal/i18n"
)

const oauthAssetBasePath = "oauth_assets/"

//go:embed oauth_assets/success.html oauth_assets/oauth.css oauth_assets/favicon.svg
var oauthAssetsFS embed.FS

var buildOAuthSuccessPageFn = buildOAuthSuccessPage

type oauthSuccessPageData struct {
	Title   string
	Heading string
	Body    string
	Action  string
}

func buildOAuthSuccessPage() (string, error) {
	templateContent, err := oauthAssetsFS.ReadFile(oauthAssetBasePath + "success.html")
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("success").Parse(string(templateContent))
	if err != nil {
		return "", err
	}

	data := oauthSuccessPageData{
		Title:   i18n.T(i18n.JiraOAuthPageTitle),
		Heading: i18n.T(i18n.JiraOAuthPageHeading),
		Body:    i18n.T(i18n.JiraOAuthPageBody),
		Action:  i18n.T(i18n.JiraOAuthPageAction),
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return "", err
	}

	return rendered.String(), nil
}

func serveOAuthCSS(w http.ResponseWriter, _ *http.Request) {
	serveOAuthAsset(w, "oauth.css", "text/css; charset=utf-8")
}

func serveOAuthFavicon(w http.ResponseWriter, _ *http.Request) {
	serveOAuthAsset(w, "favicon.svg", "image/svg+xml")
}

func serveOAuthAsset(w http.ResponseWriter, asset string, contentType string) {
	content, err := oauthAssetsFS.ReadFile(oauthAssetBasePath + asset)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(content)
}
