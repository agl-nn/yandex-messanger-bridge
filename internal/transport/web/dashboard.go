// Путь: internal/transport/web/dashboard.go
package web

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"yandex-messenger-bridge/internal/web/templates/pages"
)

// Dashboard отображает главную страницу с дашбордом
func (h *Handler) Dashboard(c echo.Context) error {
	userID := getUserIDFromContext(c)

	integrations, err := h.repo.FindByUserID(c.Request().Context(), userID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load data")
	}

	activeCount := 0
	for _, i := range integrations {
		if i.IsActive {
			activeCount++
		}
	}

	stats := map[string]interface{}{
		"total_integrations":  len(integrations),
		"active_integrations": activeCount,
		"today_deliveries":    0,
	}

	return pages.Dashboard(stats, integrations).Render(c.Request().Context(), c.Response().Writer)
}
