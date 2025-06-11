package templates

import (
	"html/template"
	"io"
)

type Templates struct {
	templates *template.Template
}

func (t *Templates) Render(w io.Writer, name string, data any) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func New(glob string) *Templates {
	return &Templates{
		templates: template.Must(template.ParseGlob(glob)),
	}
}
