// Package orgunit implements the Goéland POC organizational units (spec v2
// §5.7, §31): the internal units of the administration (directions, services,
// offices...), never ACTORs.
//
// A unit is a first-class subject (org_unit.id == subject_ref.id of kind
// ORG_UNIT) in one tree. It has no natural unique code: its abbreviation is
// often shared with its parent, live siblings never share a label, and an
// optional external reference keeps its id in a source system. The tree is a
// parent link that never forms a cycle (checked by the service and a database
// trigger, and tree mutations are serialized). Cases,
// tasks and circulations target units through typed relationships
// (CASE_HAS_ORG_UNIT_LEADER, _MANAGER, _PARTICIPANT) and
// record_metadata.owner_org_id names the owning unit of any subject.
//
// A unit is never deleted: it is dissolved with a reason (only once it has no
// live sub-unit), stays visible as history and takes no new child or
// relationship. Reading requires goeland:read; every mutation, including the
// administration of the org_unit_type catalogue, requires goeland:admin.
package orgunit
