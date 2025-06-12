package sse

import (
	"fmt"
	"net/http"
)

type Event struct {
	Type string
	Data string
}

func Send(w http.ResponseWriter, e Event) {
	if e.Type != "" {
		fmt.Fprintf(w, "event: %s\n", e.Type)
	}
	fmt.Fprintf(w, "data: %s\n\n", e.Data)
	w.(http.Flusher).Flush()
}
