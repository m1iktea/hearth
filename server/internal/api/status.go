package api

import (
	"net/http"

	"github.com/m1iktea/hearth/server/internal/store"
)

type statusHandler struct {
	snaps *store.SnapshotStore
}

func (h *statusHandler) all(w http.ResponseWriter, r *http.Request) {
	writeOK(w, h.snaps.All())
}

func (h *statusHandler) bySource(w http.ResponseWriter, r *http.Request) {
	source := r.PathValue("source")
	snap, ok := h.snaps.Get(source)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown source: "+source)
		return
	}
	writeOK(w, snap)
}
