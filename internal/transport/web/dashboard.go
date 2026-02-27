package web

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"yandex-messenger-bridge/internal/web/templates/pages"
)

// Dashboard отображает главную страницу с дашбордом
func (h *Handler) Dashboard(c echo.Context) error {
	userID := getUserIDFromContext(c)

	// Получаем статистику
	integrations, err := h.repo.FindByUserID(c.Request().Context(), userID)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load data")
	}

	// Считаем статистику
	activeCount := 0
	for _, i := range integrations {
		if i.IsActive {
			activeCount++
		}
	}

	stats := map[string]interface{}{
		"total_integrations":  len(integrations),
		"active_integrations": activeCount,
		"recent_integrations": integrations,
	}

	return pages.Dashboard(stats).Render(c.Response().Writer)
}
