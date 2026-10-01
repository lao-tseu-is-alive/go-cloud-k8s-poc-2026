package core

import (
	"testing"

	"github.com/google/uuid"
)

func lvl(l Level) *int16 { v := int16(l); return &v }

// TestAccessPrecedenceRule checks the most-specific-wins rule of the effective
// level, including a lower specific grant overriding a higher general one, and
// a confidential subject where roles and the baseline give nothing.
func TestAccessPrecedenceRule(t *testing.T) {
	cases := []struct {
		name   string
		row    accessRow
		level  Level
		source AccessSource
	}{
		{"baseline", accessRow{}, LevelRead, SourceBaseline},
		{"role over baseline", accessRow{RoleLevel: lvl(LevelManage)}, LevelManage, SourceRole},
		{"unit over role, even lower", accessRow{UnitLevel: lvl(LevelRead), RoleLevel: lvl(LevelFullControl)}, LevelRead, SourceOrgUnit},
		{"group over unit", accessRow{GroupLevel: lvl(LevelContribute), UnitLevel: lvl(LevelFullControl)}, LevelContribute, SourceGroup},
		{"personal over everything", accessRow{Personal: lvl(LevelRead), GroupLevel: lvl(LevelManage), UnitLevel: lvl(LevelFullControl)}, LevelRead, SourcePersonal},
		{"confidential: no baseline", accessRow{Confidentiality: ConfidentialLevel}, LevelNone, SourceBaseline},
		{"confidential: no role", accessRow{Confidentiality: 3, RoleLevel: lvl(LevelFullControl)}, LevelNone, SourceBaseline},
		{"confidential: explicit grants count", accessRow{Confidentiality: 5, UnitLevel: lvl(LevelRead)}, LevelRead, SourceOrgUnit},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := c.row.resolve(uuid.New())
			if a.Level != c.level || a.Source != c.source {
				t.Fatalf("got %s from %s, want %s from %s", a.Level, a.Source, c.level, c.source)
			}
		})
	}
}
