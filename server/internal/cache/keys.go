package cache

import "fmt"

const (
	KeySystemStatus   = "system:status"
	KeyOrgsList       = "orgs:list"
	KeyAdminUsersGlob = "admin:users:*"
)

func KeyUser(id uint) string {
	return fmt.Sprintf("user:%d", id)
}

func KeyOrg(id uint) string {
	return fmt.Sprintf("org:%d", id)
}

func KeyOrgMembers(id uint) string {
	return fmt.Sprintf("org:%d:members", id)
}

func KeyAdminUsers(email string, page, size int) string {
	return fmt.Sprintf("admin:users:%s:%d:%d", email, page, size)
}
