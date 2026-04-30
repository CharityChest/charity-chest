package handler

import "github.com/labstack/echo/v4"

// dataJSON writes a success response wrapped in {"data": v}.
func dataJSON(c echo.Context, code int, v any) error {
	return c.JSON(code, struct {
		Data any `json:"data"`
	}{Data: v})
}

// PaginationMeta carries pagination details for list endpoints.
type PaginationMeta struct {
	Page       int   `json:"page"`
	Size       int   `json:"size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// dataWithMetaJSON writes a paginated response: {"data": data, "metadata": meta}.
func dataWithMetaJSON(c echo.Context, code int, data any, meta PaginationMeta) error {
	return c.JSON(code, struct {
		Data any            `json:"data"`
		Meta PaginationMeta `json:"metadata"`
	}{Data: data, Meta: meta})
}
