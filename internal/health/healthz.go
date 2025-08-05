// File: internal/health/healthz.go
package health

import (
	"fmt"
	"net/http"
)

// Handler responds with a basic 200 OK health check
func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}
