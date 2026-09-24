package thing

import (
	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// TypeToProto converts a thing type to its proto representation.
func TypeToProto(t *ThingType) *goelandv1.ThingType {
	if t == nil {
		return nil
	}
	return &goelandv1.ThingType{
		Id: t.ID.String(), Code: t.Code, Label: t.Label, Description: t.Description,
		Specialization: goelandv1.ThingSpecialization(t.Specialization), IsActive: t.IsActive,
	}
}

// DomainToProto converts a hydrated thing to its proto representation.
func DomainToProto(t *Thing) *goelandv1.Thing {
	if t == nil {
		return nil
	}
	out := &goelandv1.Thing{
		SubjectRef:      core.DomainSubjectRefToProto(t.Subject),
		ThingType:       TypeToProto(t.Type),
		Name:            t.Name,
		Description:     t.Description,
		ExternalRef:     t.ExternalRef,
		GeometryGeojson: deref(t.GeometryGeoJSON),
		AreaM2:          derefFloat(t.AreaM2),
		AnchorE:         derefFloat(t.AnchorE),
		AnchorN:         derefFloat(t.AnchorN),
		Metadata:        core.StructFromMap(t.Metadata),
		CreatedAt:       core.TimestampOrNil(t.CreatedAt),
		CreatedBy:       t.CreatedBy,
		UpdatedAt:       core.TimestampOrNil(t.UpdatedAt),
		RecordMetadata:  core.DomainRecordMetadataToProto(t.RecordMetadata),
	}
	switch {
	case t.Parcel != nil:
		out.Specialization = &goelandv1.Thing_Parcel{Parcel: parcelToProto(t.Parcel)}
	case t.Building != nil:
		out.Specialization = &goelandv1.Thing_Building{Building: buildingToProto(t.Building)}
	}
	return out
}

func parcelToProto(p *Parcel) *goelandv1.ParcelDetails {
	return &goelandv1.ParcelDetails{CommuneOfs: p.CommuneOFS, ParcelNumber: p.ParcelNumber, Egrid: p.EGRID, SurfaceM2: derefFloat(p.SurfaceM2)}
}

func buildingToProto(b *Building) *goelandv1.BuildingDetails {
	out := &goelandv1.BuildingDetails{EcaNumber: b.ECANumber, BuildingStatus: goelandv1.BuildingStatus(b.Status)}
	if b.EGID != nil {
		out.Egid = *b.EGID
	}
	if b.ConstructionYear != nil {
		out.ConstructionYear = int32(*b.ConstructionYear)
	}
	return out
}

// parcelFromProto converts a request parcel block (0 surface means unknown).
func parcelFromProto(p *goelandv1.ParcelDetails) *Parcel {
	if p == nil {
		return nil
	}
	out := &Parcel{CommuneOFS: p.CommuneOfs, ParcelNumber: p.ParcelNumber, EGRID: p.Egrid}
	if p.SurfaceM2 > 0 {
		surface := p.SurfaceM2
		out.SurfaceM2 = &surface
	}
	return out
}

// buildingFromProto converts a request building block (0 means unknown).
func buildingFromProto(b *goelandv1.BuildingDetails) *Building {
	if b == nil {
		return nil
	}
	out := &Building{ECANumber: b.EcaNumber, Status: BuildingStatus(b.BuildingStatus)}
	if b.Egid > 0 {
		egid := b.Egid
		out.EGID = &egid
	}
	if b.ConstructionYear > 0 {
		year := int16(b.ConstructionYear) // bounded to 2100 by the contract
		out.ConstructionYear = &year
	}
	return out
}

func derefFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}
