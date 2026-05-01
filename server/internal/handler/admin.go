package handler

import (
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"charity-chest/internal/cache"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// AdminHandler handles root-only administration endpoints.
type AdminHandler struct {
	db    *gorm.DB
	cache *cache.Cache
}

// NewAdminHandler creates an AdminHandler backed by the given database.
func NewAdminHandler(db *gorm.DB, c *cache.Cache) *AdminHandler {
	return &AdminHandler{db: db, cache: c}
}

type orgSummary struct {
	ID   uint             `json:"id"`
	Name string           `json:"name"`
	Role model.MemberRole `json:"role"`
}

type userWithOrgs struct {
	ID            uint                      `json:"id"`
	Email         string                    `json:"email"`
	Name          string                    `json:"name"`
	Role          *model.AdministrativeRole `json:"role,omitempty"`
	MFAEnabled    bool                      `json:"mfa_enabled"`
	CreatedAt     time.Time                 `json:"created_at"`
	Organizations []orgSummary              `json:"organizations"`
}

type orgMemberRow struct {
	UserID  uint
	OrgID   uint
	OrgName string
	Role    model.MemberRole
}

type cachedSearchResult struct {
	Data []userWithOrgs `json:"data"`
	Meta PaginationMeta `json:"meta"`
}

// SearchUsers handles GET /v1/api/admin/users
// Query params: email (optional partial match), page (default 1), size (default 20, max 100).
func (h *AdminHandler) SearchUsers(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.QueryParam("size"))
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	email := c.QueryParam("email")
	ctx := c.Request().Context()
	key := cache.KeyAdminUsers(email, page, size)

	var cached cachedSearchResult
	if hit, err := h.cache.Get(ctx, key, &cached); err != nil {
		log.Printf("cache: get %s: %v", key, err)
	} else if hit {
		return dataWithMetaJSON(c, http.StatusOK, cached.Data, cached.Meta)
	}

	q := h.db.Model(&model.User{})
	if email != "" {
		q = q.Where("email LIKE ?", "%"+email+"%")
	}

	var total int64
	q.Count(&total)

	totalPages := max(1, int(math.Ceil(float64(total)/float64(size))))

	var users []model.User
	offset := (page - 1) * size
	userQ := h.db.Offset(offset).Limit(size)
	if email != "" {
		userQ = userQ.Where("email LIKE ?", "%"+email+"%")
	}
	userQ.Find(&users)

	orgsMap := make(map[uint][]orgSummary)
	if len(users) > 0 {
		ids := make([]uint, len(users))
		for i, u := range users {
			ids[i] = u.ID
		}
		var rows []orgMemberRow
		h.db.Table("org_members").
			Select("org_members.user_id, org_members.org_id, organizations.name as org_name, org_members.role").
			Joins("JOIN organizations ON organizations.id = org_members.org_id").
			Where("org_members.user_id IN ?", ids).
			Scan(&rows)
		for _, r := range rows {
			orgsMap[r.UserID] = append(orgsMap[r.UserID], orgSummary{ID: r.OrgID, Name: r.OrgName, Role: r.Role})
		}
	}

	result := make([]userWithOrgs, len(users))
	for i, u := range users {
		orgs := orgsMap[u.ID]
		if orgs == nil {
			orgs = []orgSummary{}
		}
		result[i] = userWithOrgs{
			ID:            u.ID,
			Email:         u.Email,
			Name:          u.Name,
			Role:          u.Role,
			MFAEnabled:    u.MFAEnabled,
			CreatedAt:     u.CreatedAt,
			Organizations: orgs,
		}
	}

	meta := PaginationMeta{Page: page, Size: size, Total: total, TotalPages: totalPages}

	if err := h.cache.Set(ctx, key, cachedSearchResult{Data: result, Meta: meta}); err != nil {
		log.Printf("cache: set %s: %v", key, err)
	}

	return dataWithMetaJSON(c, http.StatusOK, result, meta)
}
