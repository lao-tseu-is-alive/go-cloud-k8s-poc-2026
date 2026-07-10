package actor

import (
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// DomainCategoryToProto converts an organization category to its proto representation.
func DomainCategoryToProto(c *OrganizationCategory) *goelandv1.OrganizationCategory {
	if c == nil {
		return nil
	}
	return &goelandv1.OrganizationCategory{
		Id:       c.ID.String(),
		Code:     c.Code,
		Label:    c.Label,
		IsActive: c.IsActive,
	}
}

// DomainContactToProto converts a contact to its proto representation.
func DomainContactToProto(c *Contact) *goelandv1.ActorContact {
	if c == nil {
		return nil
	}
	return &goelandv1.ActorContact{
		ContactType: goelandv1.ContactType(c.ContactType),
		Value:       c.Value,
		IsPrimary:   c.IsPrimary,
		Label:       c.Label,
	}
}

// DomainToProto converts a domain Actor (with hydrated associations) to its proto representation.
func DomainToProto(act *Actor) *goelandv1.Actor {
	if act == nil {
		return nil
	}
	out := &goelandv1.Actor{
		SubjectRef:      core.DomainSubjectRefToProto(act.Subject),
		ActorKind:       goelandv1.ActorKind(act.ActorKind),
		DisplayName:     act.DisplayName,
		NameForSearch:   act.NameForSearch,
		IsActive:        act.IsActive,
		PublicationCode: act.PublicationCode,
		CreatedAt:       core.TimestampOrNil(act.CreatedAt),
		CreatedBy:       act.CreatedBy,
		UpdatedAt:       core.TimestampOrNil(act.UpdatedAt),
		RecordMetadata:  core.DomainRecordMetadataToProto(act.RecordMetadata),
	}
	switch act.ActorKind {
	case KindOrganization:
		categoryCode := ""
		if act.Category != nil {
			categoryCode = act.Category.Code
		}
		out.Specialization = &goelandv1.Actor_Organization{
			Organization: &goelandv1.OrganizationDetails{
				LegalName:    act.LegalName,
				CategoryCode: categoryCode,
				Complement:   act.OrgComplement,
			},
		}
	case KindPerson:
		out.Specialization = &goelandv1.Actor_Person{
			Person: &goelandv1.PersonDetails{
				IsChRegister:  act.IsCHRegister,
				ChRegisterRef: act.CHRegisterRef,
			},
		}
	}
	for _, c := range act.Contacts {
		out.Contacts = append(out.Contacts, DomainContactToProto(c))
	}
	return out
}

// DomainsToProto maps a slice of actors.
func DomainsToProto(actors []*Actor) []*goelandv1.Actor {
	out := make([]*goelandv1.Actor, 0, len(actors))
	for _, a := range actors {
		out = append(out, DomainToProto(a))
	}
	return out
}

// protoContactsToDomain converts wire contacts to the domain input type.
func protoContactsToDomain(in []*goelandv1.ActorContact) []ContactInput {
	out := make([]ContactInput, 0, len(in))
	for _, c := range in {
		if c == nil {
			continue
		}
		out = append(out, ContactInput{
			ContactType: ContactType(c.ContactType),
			Value:       c.Value,
			IsPrimary:   c.IsPrimary,
			Label:       c.Label,
		})
	}
	return out
}
