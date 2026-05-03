package model

// Plan represents the subscription tier of an Organisation.
type Plan string

const (
	PlanFree       Plan = "free"
	PlanPro        Plan = "pro"
	PlanEnterprise Plan = "enterprise"
)

// PlanLimits holds the maximum number of members per org-level role for a plan.
// A value of -1 means unlimited; 0 means the role is not permitted at all.
type PlanLimits struct {
	MaxOwners      int
	MaxAdmins      int
	MaxOperational int
}

// ForRole returns the limit that applies to the given MemberRole.
func (l PlanLimits) ForRole(r MemberRole) int {
	switch r {
	case OrgRoleOwner:
		return l.MaxOwners
	case OrgRoleAdmin:
		return l.MaxAdmins
	case OrgRoleOperational:
		return l.MaxOperational
	default:
		return 0
	}
}

// LimitsFor returns the PlanLimits for the given Plan.
// Unknown or empty plans fall back to free limits.
func LimitsFor(p Plan) PlanLimits {
	switch p {
	case PlanPro:
		return PlanLimits{MaxOwners: 1, MaxAdmins: 3, MaxOperational: 15}
	case PlanEnterprise:
		return PlanLimits{MaxOwners: -1, MaxAdmins: -1, MaxOperational: -1}
	default: // free or unrecognised
		return PlanLimits{MaxOwners: 1, MaxAdmins: 0, MaxOperational: 5}
	}
}
