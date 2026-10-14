// Package caseexport implements the Goéland POC case export (spec v2 §51,
// roadmap GLD-029): a structured copy of a case as a ZIP archive.
//
// An export is a background job. RequestExport checks MANAGE on the case,
// records EXPORT in the sensitive read audit (granted or refused, whatever the
// confidentiality) and queues a case_export row. A Worker of any replica claims
// queued jobs (FOR UPDATE SKIP LOCKED), builds the archive as the requester —
// manifest.json (a goeland.v1.CaseExportManifest: the case, its timeline,
// tasks, circulations, relationships, documents with all their versions, the
// audit, provenance and retention) and each content blob once under
// blobs/<sha256> — and streams it into the blob store. Only what the requester
// may read is exported; the manifest counts what was withheld. Each
// confidential document placed in an archive is recorded as EXPORT too.
//
// Only the requester downloads a built archive, while it still holds MANAGE on
// the case and until the archive expires; the worker then removes the bytes
// and keeps the row as history.
//
// The package composes the read services of the other domains and is imported
// by no domain package.
package caseexport
