package model_test

import (
	"testing"

	"charity-chest/internal/model"
)

func TestLimitsFor_Free(t *testing.T) {
	l := model.LimitsFor(model.PlanFree)
	if l.MaxOwners != 1 {
		t.Errorf("MaxOwners = %d, want 1", l.MaxOwners)
	}
	if l.MaxAdmins != 0 {
		t.Errorf("MaxAdmins = %d, want 0", l.MaxAdmins)
	}
	if l.MaxOperational != 5 {
		t.Errorf("MaxOperational = %d, want 5", l.MaxOperational)
	}
}

func TestLimitsFor_Pro(t *testing.T) {
	l := model.LimitsFor(model.PlanPro)
	if l.MaxOwners != 1 {
		t.Errorf("MaxOwners = %d, want 1", l.MaxOwners)
	}
	if l.MaxAdmins != 3 {
		t.Errorf("MaxAdmins = %d, want 3", l.MaxAdmins)
	}
	if l.MaxOperational != 15 {
		t.Errorf("MaxOperational = %d, want 15", l.MaxOperational)
	}
}

func TestLimitsFor_Enterprise(t *testing.T) {
	l := model.LimitsFor(model.PlanEnterprise)
	if l.MaxOwners != -1 {
		t.Errorf("MaxOwners = %d, want -1 (unlimited)", l.MaxOwners)
	}
	if l.MaxAdmins != -1 {
		t.Errorf("MaxAdmins = %d, want -1 (unlimited)", l.MaxAdmins)
	}
	if l.MaxOperational != -1 {
		t.Errorf("MaxOperational = %d, want -1 (unlimited)", l.MaxOperational)
	}
}

func TestLimitsFor_Unknown_FallsBackToFree(t *testing.T) {
	l := model.LimitsFor(model.Plan("unknown"))
	free := model.LimitsFor(model.PlanFree)
	if l != free {
		t.Errorf("unknown plan limits = %+v, want free limits %+v", l, free)
	}
}

func TestLimitsFor_Empty_FallsBackToFree(t *testing.T) {
	l := model.LimitsFor(model.Plan(""))
	free := model.LimitsFor(model.PlanFree)
	if l != free {
		t.Errorf("empty plan limits = %+v, want free limits %+v", l, free)
	}
}

func TestPlanLimits_ForRole(t *testing.T) {
	l := model.PlanLimits{MaxOwners: 1, MaxAdmins: 3, MaxOperational: 15}
	cases := []struct {
		role model.MemberRole
		want int
	}{
		{model.OrgRoleOwner, 1},
		{model.OrgRoleAdmin, 3},
		{model.OrgRoleOperational, 15},
		{model.MemberRole("unknown"), 0},
	}
	for _, tc := range cases {
		got := l.ForRole(tc.role)
		if got != tc.want {
			t.Errorf("ForRole(%q) = %d, want %d", tc.role, got, tc.want)
		}
	}
}
