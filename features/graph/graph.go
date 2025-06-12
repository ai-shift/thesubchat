// Provides structurized user's chats representation
package graph

import (
	"net/http"
	"shellshift/internal/db"
	"shellshift/internal/templates"
)

type GraphHandler struct {
	t *templates.Templates
	q *db.Queries
}

func InitMux(q *db.Queries) *http.ServeMux {
	h := GraphHandler{
		t: templates.New("features/graph/views/*.html"),
		q: q,
	}

	m := http.NewServeMux()
	m.HandleFunc("GET /", h.getGraph)
	return m
}

func (h GraphHandler) getGraph(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}
