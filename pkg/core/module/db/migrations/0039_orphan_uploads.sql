-- migrate:up

-- Goéland POC — collection of orphan uploads (GLD-045).
--
-- An upload stores bytes and registers a content_blob before CreateDocument or
-- AddDocumentVersion attaches it to a version; bytes never attached stayed
-- forever. The server now destroys the bytes of a blob that no version
-- references once it has waited GOELAND_ORPHAN_UPLOAD_TTL_HOURS since its last
-- upload, and marks the blob disposed_at (the row stays: an identical upload
-- later stores the bytes again, as after an elimination, migration 0037).
--
-- uploaded_at is the last time bytes were handed over for this content: a new
-- blob, or an identical upload reusing it (the deduplicated upload gives the
-- client the existing blob id, which then needs a fresh delay before its
-- CreateDocument). NULL for blobs registered before this migration and for the
-- imported blobs, whose age is their created_at.

ALTER TABLE content_blob ADD COLUMN uploaded_at TIMESTAMPTZ;

-- The collector's candidates: blobs whose bytes Goéland holds. The imported
-- blobs (empty storage_ref, bytes left in the legacy store) are outside it.
CREATE INDEX idx_content_blob_held_age ON content_blob ((coalesce(uploaded_at, created_at)))
    WHERE disposed_at IS NULL AND storage_ref <> '';

-- migrate:down

DROP INDEX IF EXISTS idx_content_blob_held_age;
ALTER TABLE content_blob DROP COLUMN IF EXISTS uploaded_at;
