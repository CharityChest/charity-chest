package model

// System-level roles stored on users.role.
// RoleRoot is set exclusively via direct DB write; no API endpoint creates it.
const (
	RoleRoot   = "root"
	RoleSystem = "system"
)

// Org-level roles stored on org_members.role.
const (
	OrgRoleOwner       = "owner"
	OrgRoleAdmin       = "admin"
	OrgRoleOperational = "operational"
)

// CanAssignOrgRole reports whether an actor with actorOrgRole may assign
// targetOrgRole to another user within the same organisation.
//
// Hierarchy:
//
//	owner  → admin, operational
//	admin  → operational
//	operational / unknown → nothing
//
// Root and system users bypass this check entirely (handled in the middleware/handler).
func CanAssignOrgRole(actorOrgRole, targetOrgRole string) bool {
	switch actorOrgRole {
	case OrgRoleOwner:
		return targetOrgRole == OrgRoleAdmin || targetOrgRole == OrgRoleOperational
	case OrgRoleAdmin:
		return targetOrgRole == OrgRoleOperational
	default:
		return false
	}
}

// ValidOrgRole reports whether s is a recognised org-level role.
func ValidOrgRole(s string) bool {
	return s == OrgRoleOwner || s == OrgRoleAdmin || s == OrgRoleOperational
}
