package model_test

import (
	"testing"

	"charity-chest/internal/model"
)

func TestCanAssignOrgRole(t *testing.T) {
	tests := []struct {
		actor  model.MemberRole
		target model.MemberRole
		want   bool
	}{
		// Owner can assign below
		{model.OrgRoleOwner, model.OrgRoleAdmin, true},
		{model.OrgRoleOwner, model.OrgRoleOperational, true},
		// Owner cannot assign owner (same level)
		{model.OrgRoleOwner, model.OrgRoleOwner, false},

		// Admin can assign operational only
		{model.OrgRoleAdmin, model.OrgRoleOperational, true},
		{model.OrgRoleAdmin, model.OrgRoleAdmin, false},
		{model.OrgRoleAdmin, model.OrgRoleOwner, false},

		// Operational cannot assign anything
		{model.OrgRoleOperational, model.OrgRoleOperational, false},
		{model.OrgRoleOperational, model.OrgRoleAdmin, false},
		{model.OrgRoleOperational, model.OrgRoleOwner, false},

		// Unknown actor cannot assign anything
		{"", model.OrgRoleAdmin, false},
		{"unknown", model.OrgRoleOperational, false},
	}
	for _, tc := range tests {
		got := model.CanAssignOrgRole(tc.actor, tc.target)
		if got != tc.want {
			t.Errorf("CanAssignOrgRole(%q, %q) = %v, want %v", tc.actor, tc.target, got, tc.want)
		}
	}
}

func TestValidOrgRole(t *testing.T) {
	valid := []model.MemberRole{model.OrgRoleOwner, model.OrgRoleAdmin, model.OrgRoleOperational}
	for _, r := range valid {
		if !model.ValidOrgRole(r) {
			t.Errorf("ValidOrgRole(%q) = false, want true", r)
		}
	}
	invalid := []model.MemberRole{"root", "system", "", "superadmin", "OWNER"}
	for _, r := range invalid {
		if model.ValidOrgRole(r) {
			t.Errorf("ValidOrgRole(%q) = true, want false", r)
		}
	}
}
