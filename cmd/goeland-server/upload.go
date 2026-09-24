package main

import (
	"bufio"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document/filestore"
)

// uploadFormField is the multipart form field carrying the file bytes.
const uploadFormField = "file"

// httpAuthMiddleware guards a plain HTTP handler (one that is NOT served through
// the Connect/Vanguard transcoder, and therefore misses the Connect auth
// interceptor) with the same bearer-token verifier used everywhere else, and
// requires scope (e.g. core.ScopeWrite for uploads, which register content). On
// success the authenticated user is stored in the request context.
func httpAuthMiddleware(verifier authadapter.TokenVerifier, log *slog.Logger, scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if verifier == nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token verifier is not configured"})
			return
		}
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "valid bearer token is required"})
			return
		}
		user, err := verifier.VerifyBearerToken(r.Context(), parts[1])
		if err != nil || user == nil || user.AppUserID <= 0 {
			log.Warn("bearer token verification failed", "error", err, "path", r.URL.Path)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bearer token"})
			return
		}
		if !user.HasScope(scope) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "required scope is missing"})
			return
		}
		next.ServeHTTP(w, r.WithContext(authadapter.ContextWithUser(r.Context(), user)))
	})
}

// uploadResponse describes the registered content; the client passes
// ContentBlobID to CreateDocument or AddDocumentVersion. Field names are
// camelCase to match the JSON shape produced by the Vanguard REST transcoder.
type uploadResponse struct {
	// ContentBlobID is the registered content_blob, the trusted content reference.
	ContentBlobID string `json:"contentBlobId"`
	// Reused is true when identical content was already known: the upload was
	// deduplicated and the existing blob returned (accepted existence-oracle
	// risk, IMPLEMENTATION_STATUS §3g).
	Reused bool `json:"reused"`
	// StorageRef is the internal://<name> reference of the stored blob.
	StorageRef string `json:"storageRef"`
	// SHA256 is the server-computed lower-case hex digest of the bytes.
	SHA256 string `json:"sha256"`
	// FileSizeBytes is the server-measured size of the bytes.
	FileSizeBytes int64 `json:"fileSizeBytes"`
	// MimeType is the client-declared type unless absent or generic, else the
	// type implied by the file extension, else one sniffed from the first bytes.
	MimeType string `json:"mimeType"`
	// Filename is the original client filename, informational only.
	Filename string `json:"filename"`
}

// uploadHandler ingests an uploaded file through the document service: the
// bytes are stored, the SHA-256 and size are computed server-side and the
// content is registered as a content_blob, deduplicated globally. It does NOT
// create a document: the client follows up with CreateDocument or
// AddDocumentVersion (which own validation, governance and audit) using the
// returned contentBlobId. The request body size is capped by the caller via
// http.MaxBytesHandler.
func uploadHandler(docs *document.Service, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile(uploadFormField)
		if err != nil {
			if errors.Is(err, http.ErrMissingFile) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing form file field \"" + uploadFormField + "\""})
				return
			}
			// http.MaxBytesReader surfaces oversize bodies here.
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "uploaded file is too large"})
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart upload"})
			return
		}
		defer file.Close()

		// Peek the first bytes for content sniffing without consuming the
		// stream, then hand the buffered reader to the service.
		buffered := bufio.NewReader(file)
		peek, _ := buffered.Peek(512)
		mimeType := detectMimeType(header.Filename, header.Header.Get("Content-Type"), peek)

		user, _ := authadapter.UserFromContext(r.Context())
		res, err := docs.IngestContent(r.Context(), buffered, header.Filename, mimeType, core.OperatorID(user))
		if err != nil {
			log.Error("upload: ingest content failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store uploaded file"})
			return
		}
		log.Info("document content uploaded",
			"content_blob_id", res.Blob.ID,
			"size", res.Blob.FileSizeBytes,
			"mime", res.Blob.MimeType,
			"reused", res.Reused,
			"request_id", core.RequestIDFromContext(r.Context()),
		)
		status := http.StatusCreated
		if res.Reused {
			status = http.StatusOK
		}
		writeJSON(w, status, uploadResponse{
			ContentBlobID: res.Blob.ID.String(),
			Reused:        res.Reused,
			StorageRef:    res.Blob.StorageRef,
			SHA256:        res.Blob.SHA256,
			FileSizeBytes: res.Blob.FileSizeBytes,
			MimeType:      res.Blob.MimeType,
			Filename:      res.Filename,
		})
	}
}

// downloadHandler streams a previously uploaded blob back by its internal://
// storage_ref (passed as ?ref=). It exists for end-to-end verification of the
// upload round-trip; access is gated by httpAuthMiddleware.
func downloadHandler(store *filestore.Store, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Query().Get("ref")
		f, err := store.Open(ref)
		if err != nil {
			if errors.Is(err, filestore.ErrInvalidRef) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "blob not found"})
				return
			}
			log.Error("download: open blob failed", "error", err, "ref", ref)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read file"})
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to stat file"})
			return
		}
		// ServeContent handles Range requests and content-type sniffing by name.
		http.ServeContent(w, r, filepath.Base(f.Name()), info.ModTime(), f)
	}
}

// frontendConfigHandler tells the SPA how to authenticate: "dev" mode expects a
// manually entered static token, "jwt" mode mints tokens from authBaseUrl.
func frontendConfigHandler(config serverConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"authMode":    config.AuthMode,
			"authBaseUrl": config.AuthServerURL,
		})
	}
}

// detectMimeType prefers a meaningful client-provided Content-Type, then the
// file extension, then a content sniff of the leading bytes.
func detectMimeType(filename, declared string, peek []byte) string {
	if declared != "" && declared != "application/octet-stream" {
		return declared
	}
	if ext := filepath.Ext(filename); ext != "" {
		if byExt := mime.TypeByExtension(ext); byExt != "" {
			return byExt
		}
	}
	return http.DetectContentType(peek)
}
