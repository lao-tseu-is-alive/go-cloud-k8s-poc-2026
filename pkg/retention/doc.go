// Package retention keeps the retention of closed cases (roadmap GLD-032 step
// A, spec v2 §35-36): the retention_policy catalogue, the retention computed
// when a case is closed (a snapshot of its type's default policy, ended when
// the case is reopened), the legal holds that block any disposal and the list
// of due cases for the records manager.
//
// The case is the unit; its documents follow it. Nothing here disposes of
// anything: the disposition bordereaux are step B. casefile calls
// StartCaseRetentionTx and EndCaseRetentionTx inside its transition
// transaction; retention never imports casefile. Every change is audited on
// the CASE subject.
package retention
