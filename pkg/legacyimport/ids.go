package legacyimport

import (
	"strconv"

	"github.com/google/uuid"
)

// namespace is the UUIDv5 namespace of the imported ids; it never changes, so
// two runs give the same ids.
var namespace = uuid.MustParse("76d7f700-049f-42b8-9f61-92fdfe23f89e")

// SourceSystem names the legacy system in provenance rows and import batches.
const SourceSystem = "goeland"

// OperatorID is recorded where the import itself acts (grants, memberships).
const OperatorID = "import:goeland"

// Key kinds of the deterministic ids; subject kinds use their SubjectKind name.
const (
	keyCaseType = "CASE_TYPE"
	keyAddress  = "ADDRESS"
)

// ID is the deterministic id of legacy row legacyID of kind (UUIDv5 of
// "<kind>:<legacyID>").
func ID(kind string, legacyID int64) uuid.UUID {
	return uuid.NewSHA1(namespace, []byte(kind+":"+strconv.FormatInt(legacyID, 10)))
}
