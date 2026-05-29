package handler

import (
	"errors"
	"log"
	"net/http"

	"charity-chest/services/operational/backend/internal/adminclient"
	"charity-chest/services/operational/backend/internal/i18n"
	"charity-chest/services/operational/backend/internal/middleware"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// MeHandler serves GET /v1/api/me — operational JWT in, admin user out.
type MeHandler struct {
	admin AdminAPI
}

func NewMeHandler(admin AdminAPI) *MeHandler {
	return &MeHandler{admin: admin}
}

// Me returns the current user's slim DTO. The JWT middleware already
// validated the bearer token and stashed the UUID, so this handler just
// forwards to admin and decodes the envelope.
func (h *MeHandler) Me(c echo.Context) error {
	loc := middleware.LocaleFrom(c)
	userUUID, ok := c.Get(middleware.UserUUIDContextKey).(uuid.UUID)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, i18n.T(loc, i18n.KeyInvalidToken))
	}

	ctx := adminclient.ContextWithLocale(c.Request().Context(), loc)
	user, err := h.admin.GetUser(ctx, userUUID)
	if err != nil {
		switch {
		case errors.Is(err, adminclient.ErrUserNotFound):
			return echo.NewHTTPError(http.StatusNotFound, i18n.T(loc, i18n.KeyUserNotFound))
		case errors.Is(err, adminclient.ErrAdminUnavailable):
			return echo.NewHTTPError(http.StatusBadGateway, i18n.T(loc, i18n.KeyAdminUnavailable))
		default:
			log.Printf("me: admin error: %v", err)
			return echo.NewHTTPError(http.StatusBadGateway, i18n.T(loc, i18n.KeyAdminUnavailable))
		}
	}
	return dataJSON(c, http.StatusOK, user)
}
