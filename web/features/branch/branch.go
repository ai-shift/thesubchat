package branch

import (
	"log/slog"
	"net/http"
	"shellshift/internal/db"
	"shellshift/internal/templates"
)

type BranchHandler struct {
	t *templates.Templates
	q *db.Queries
}

func InitMux(q *db.Queries) *http.ServeMux {
	h := BranchHandler{
		t: templates.New("web/features/branch/views/*.html"),
		q: q,
	}

	m := http.NewServeMux()
	m.HandleFunc("GET /", h.getBranch)
	return m
}

type BranchView struct {
}

func (h BranchHandler) getBranch(w http.ResponseWriter, r *http.Request) {
	err := h.t.Render(w, "branch", BranchView{})

	if err != nil {
		slog.Error("failed to render graph", "with", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
