-- migrate:up

-- Goéland POC — accent-insensitive full-text search for documents.
--
-- The document.search_vector generated column (migration 0003) indexed the raw
-- title/description, so "château" and "chateau" were treated as different words.
-- This migration makes search accent-insensitive by folding accents through the
-- unaccent extension in BOTH the stored vector and the query (see
-- pkg/document/sql.go searchDocumentsSQL, which uses the same wrapper).
--
-- Why a wrapper function: a STORED generated column may only call IMMUTABLE
-- functions, but unaccent() is merely STABLE (it resolves the dictionary at
-- call time). Passing the dictionary explicitly makes the result deterministic,
-- so we wrap it in an IMMUTABLE SQL function that is safe to use in the
-- generated expression and in indexes.

CREATE EXTENSION IF NOT EXISTS unaccent;

-- IMMUTABLE accent-folding wrapper. The explicit 'public.unaccent' dictionary
-- argument is what makes it sound to mark this IMMUTABLE.
-- migrate:statementbegin
CREATE OR REPLACE FUNCTION immutable_unaccent(text)
    RETURNS text
    LANGUAGE sql
    IMMUTABLE PARALLEL SAFE STRICT
AS $$
    SELECT public.unaccent('public.unaccent', $1)
$$;
-- migrate:statementend

-- Rebuild search_vector to fold accents. Dropping the generated column also
-- drops its GIN index (idx_document_search_vector from 0003), so recreate it.
DROP INDEX IF EXISTS idx_document_search_vector;
ALTER TABLE document DROP COLUMN IF EXISTS search_vector;
ALTER TABLE document
    ADD COLUMN search_vector TSVECTOR
    GENERATED ALWAYS AS (
        to_tsvector('simple', immutable_unaccent(coalesce(title, '') || ' ' || coalesce(description, '')))
    ) STORED;
CREATE INDEX idx_document_search_vector ON document USING gin (search_vector);

-- migrate:down

-- Restore the accent-sensitive vector from migration 0003.
DROP INDEX IF EXISTS idx_document_search_vector;
ALTER TABLE document DROP COLUMN IF EXISTS search_vector;
ALTER TABLE document
    ADD COLUMN search_vector TSVECTOR
    GENERATED ALWAYS AS (
        to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(description, ''))
    ) STORED;
CREATE INDEX idx_document_search_vector ON document USING gin (search_vector);
DROP FUNCTION IF EXISTS immutable_unaccent(text);
