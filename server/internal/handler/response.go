package handler

import "github.com/labstack/echo/v4"

// dataJSON writes a success response wrapped in {"data": v}.
func dataJSON(c echo.Context, code int, v any) error {
	return c.JSON(code, struct {
		Data any `json:"data"`
	}{Data: v})
}
