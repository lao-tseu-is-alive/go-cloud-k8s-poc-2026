package core

import goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"

var granteeKindToProto = map[GranteeKind]goelandv1.GranteeKind{
	GranteeUser:         goelandv1.GranteeKind_GRANTEE_KIND_USER,
	GranteeGroup:        goelandv1.GranteeKind_GRANTEE_KIND_GROUP,
	GranteeOrgUnit:      goelandv1.GranteeKind_GRANTEE_KIND_ORG_UNIT,
	GranteeCreatorUnits: goelandv1.GranteeKind_GRANTEE_KIND_CREATOR_UNITS,
}

// GranteeKindToProto converts a grantee kind (UNSPECIFIED when unknown).
func GranteeKindToProto(k GranteeKind) goelandv1.GranteeKind {
	return granteeKindToProto[k]
}

// GranteeKindFromProto converts a proto grantee kind ("" when unspecified).
func GranteeKindFromProto(k goelandv1.GranteeKind) GranteeKind {
	for domain, proto := range granteeKindToProto {
		if proto == k {
			return domain
		}
	}
	return ""
}

// LevelToProto converts an access level to the Permission enum (NONE for 0).
func LevelToProto(l Level) goelandv1.Permission {
	return goelandv1.Permission(int32(l) + 1)
}

// LevelFromProto converts a Permission (READ to FULL_CONTROL) to a level; other values give LevelNone.
func LevelFromProto(p goelandv1.Permission) Level {
	if p < goelandv1.Permission_PERMISSION_READ || p > goelandv1.Permission_PERMISSION_FULL_CONTROL {
		return LevelNone
	}
	return Level(int32(p) - 1)
}
