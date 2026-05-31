package web

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
)

// MustParsePage parses layout.html + <name>.html + all partials into one template set.
// Panics on parse error (called at startup).
func MustParsePage(name string) *template.Template {
	t := template.Must(
		template.New("layout.html").
			Funcs(FuncMap).
			ParseFS(TemplatesFS,
				"layout.html",
				name+".html",
				"partials/*.html",
			),
	)
	return t
}

// MustParseStandalone parses a single template file with no layout (e.g. login).
func MustParseStandalone(name string) *template.Template {
	return template.Must(
		template.New(name + ".html").
			Funcs(FuncMap).
			ParseFS(TemplatesFS, name+".html"),
	)
}

// RenderPartial renders a named template from t into a string. Used for SSE fragments.
func RenderPartial(t *template.Template, name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("render partial %q: %w", name, err)
	}
	return buf.String(), nil
}

// Render executes the root template (layout.html for pages, standalone for login)
// and writes the result to w. On error it writes a 500.
func Render(w http.ResponseWriter, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

