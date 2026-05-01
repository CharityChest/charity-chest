package cache_test

import (
	"testing"

	"charity-chest/internal/cache"
)

func TestKeyUser(t *testing.T) {
	if got := cache.KeyUser(7); got != "user:7" {
		t.Errorf("KeyUser(7) = %q, want user:7", got)
	}
}

func TestKeyOrg(t *testing.T) {
	if got := cache.KeyOrg(3); got != "org:3" {
		t.Errorf("KeyOrg(3) = %q, want org:3", got)
	}
}

func TestKeyOrgMembers(t *testing.T) {
	if got := cache.KeyOrgMembers(5); got != "org:5:members" {
		t.Errorf("KeyOrgMembers(5) = %q, want org:5:members", got)
	}
}

func TestKeyAdminUsers(t *testing.T) {
	if got := cache.KeyAdminUsers("alice", 2, 50); got != "admin:users:alice:2:50" {
		t.Errorf("KeyAdminUsers = %q, want admin:users:alice:2:50", got)
	}
}

func TestKeyAdminUsers_EmptyEmail(t *testing.T) {
	if got := cache.KeyAdminUsers("", 1, 20); got != "admin:users::1:20" {
		t.Errorf("KeyAdminUsers empty email = %q, want admin:users::1:20", got)
	}
}

func TestKeyConstants(t *testing.T) {
	if cache.KeySystemStatus != "system:status" {
		t.Errorf("KeySystemStatus = %q, want system:status", cache.KeySystemStatus)
	}
	if cache.KeyOrgsList != "orgs:list" {
		t.Errorf("KeyOrgsList = %q, want orgs:list", cache.KeyOrgsList)
	}
	if cache.KeyAdminUsersGlob != "admin:users:*" {
		t.Errorf("KeyAdminUsersGlob = %q, want admin:users:*", cache.KeyAdminUsersGlob)
	}
}
