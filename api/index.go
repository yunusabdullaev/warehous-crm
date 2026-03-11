package handler

import (
	"fmt"
	"net/http"
)

// Handler is a basic test handler for Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `{"status": "test", "route": "%s"}`, r.URL.Path)
}
