package main

import (
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/caseexport"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// exportDownloadDeadline bounds the download of one case export archive; the
// server's WriteTimeout (30 s) is too short for a large archive.
const exportDownloadDeadline = 30 * time.Minute

// exportContentHandler streams a built case export archive (GLD-029) to its
// requester, who must still hold MANAGE on the case. Like the document content
// download it is out of proto, behind httpAuthMiddleware.
func exportContentHandler(exports *caseexport.Service, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil || len(r.URL.Query()) > 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid export id or unknown query parameter"})
			return
		}
		user, _ := authadapter.UserFromContext(r.Context())
		archive, err := exports.OpenArchive(r.Context(), id, core.OperatorID(user))
		if err != nil {
			writeExportError(w, log, err)
			return
		}
		defer func() { _ = archive.Object.Close() }()
		if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(exportDownloadDeadline)); err != nil {
			log.Warn("export download: cannot extend the write deadline", "error", err)
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": archive.Filename}))
		w.Header().Set("Content-Length", strconv.FormatInt(archive.Export.SizeBytes, 10))
		if _, err := io.Copy(w, archive.Object); err != nil {
			log.Warn("export download: stream interrupted", "error", err, "export_id", id)
		}
	}
}

// writeExportError maps an archive opening error to an HTTP answer.
func writeExportError(w http.ResponseWriter, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, core.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "export not found"})
	case errors.Is(err, core.ErrPermissionDenied):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "permission denied"})
	case errors.Is(err, core.ErrInvalidState):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	default:
		log.Error("export download: open archive failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the archive"})
	}
}
