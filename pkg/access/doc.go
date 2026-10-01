// Package access is the Goéland access domain (spec v2 §32, roadmap GLD-048):
// the grants on subjects and the security groups.
//
// A grant gives a level (READ < CONTRIBUTE < MANAGE < FULL_CONTROL) on one
// subject to an internal user, a security group or an org unit; a change or a
// revocation keeps the row as history and is audited on the subject, and a
// subject never loses its last FULL_CONTROL grant. Groups are GROUP subjects;
// membership is a USER_MEMBER_OF_GROUP relationship, ended on removal. The
// effective level itself is computed by core.EffectiveAccessTx, which every
// domain uses inside its transactions (core.EnsureAccessTx).
package access
