package cache

import "fmt"

// Fixed cache keys for singleton resources.
const (
	KeySystemStatus   = "system:status"
	KeyOrgsList       = "orgs:list"
	KeyAdminUsersGlob = "admin:users:*"
)

// KeyUser returns the cache key for a single user by ID.
func KeyUser(id uint) string {
	return fmt.Sprintf("user:%d", id)
}

// KeyOrg returns the cache key for a single organisation by ID.
func KeyOrg(id uint) string {
	return fmt.Sprintf("org:%d", id)
}

// KeyOrgMembers returns the cache key for the member list of an organisation.
func KeyOrgMembers(id uint) string {
	return fmt.Sprintf("org:%d:members", id)
}

// KeyAdminUsers returns the cache key for a paginated admin user-search result.
func KeyAdminUsers(email string, page, size int) string {
	return fmt.Sprintf("admin:users:%s:%d:%d", email, page, size)
}
