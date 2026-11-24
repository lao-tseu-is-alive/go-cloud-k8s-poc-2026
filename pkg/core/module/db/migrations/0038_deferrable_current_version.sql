-- migrate:up

-- Goéland POC — a deferrable current version reference (legacy import performance).
--
-- document.current_version_id points at a document_version that itself points
-- at its document: a bulk load could only write the documents, then their
-- versions, then UPDATE every document to its version. On the 2.37M imported
-- documents that UPDATE rewrote the whole table and every index (6 minutes).
-- Made deferrable, the reference can be written with the document inside a
-- transaction that declares SET CONSTRAINTS ... DEFERRED, and is still checked
-- at its commit. It stays immediate by default: nothing changes for the
-- application.

ALTER TABLE document
    ALTER CONSTRAINT document_current_version_id_fkey DEFERRABLE INITIALLY IMMEDIATE;

-- migrate:down

ALTER TABLE document
    ALTER CONSTRAINT document_current_version_id_fkey NOT DEFERRABLE;
