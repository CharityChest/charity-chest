// Package handler holds Echo handlers for the operational backend.
package handler

import "github.com/labstack/echo/v4"

// dataJSON writes a success response wrapped in {"data": v}.
// Mirrors services/admin/backend's envelope so app code can use one decoder shape.
func dataJSON(c echo.Context, code int, v any) error {
	return c.JSON(code, struct {
		Data any `json:"data"`
	}{Data: v})
}
