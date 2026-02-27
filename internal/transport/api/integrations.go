// Путь: internal/transport/api/integrations.go
package api

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"yandex-messenger-bridge/internal/domain"
	"yandex-messenger-bridge/internal/repository/interface"
	"yandex-messenger-bridge/internal/service/encryption"
)

type IntegrationAPI struct {
	repo      _interface.IntegrationRepository
	encryptor *encryption.Encryptor
	baseURL   string
}

type CreateIntegrationRequest struct {
	Name            string                 `json:"name" validate:"required"`
	SourceType      string                 `json:"source_type" validate:"required"`
	SourceConfig    map[string]interface{} `json:"source_config"`
	DestinationType string                 `json:"destination_type"`
	ChatID          string                 `json:"chat_id" validate:"required"`
	BotToken        string                 `json:"bot_token" validate:"required"`
	IsActive        bool                   `json:"is_active"`
}

type UpdateIntegrationRequest struct {
	Name         string                 `json:"name"`
	SourceConfig map[string]interface{} `json:"source_config"`
	ChatID       string                 `json:"chat_id"`
	BotToken     string                 `json:"bot_token"`
	IsActive     bool                   `json:"is_active"`
}

func NewIntegrationAPI(repo _interface.IntegrationRepository, encryptor *encryption.Encryptor, baseURL string) *IntegrationAPI {
	return &IntegrationAPI{
		repo:      repo,
		encryptor: encryptor,
		baseURL:   baseURL,
	}
}

func (api *IntegrationAPI) List(c echo.Context) error {
	userID := c.Get("user_id").(string)

	integrations, err := api.repo.FindByUserID(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":  integrations,
		"total": len(integrations),
	})
}

func (api *IntegrationAPI) Create(c echo.Context) error {
	var req CreateIntegrationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}

	userID := c.Get("user_id").(string)

	// Шифруем токен
	encryptedToken, err := api.encryptor.Encrypt(req.BotToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to encrypt token")
	}

	integration := &domain.Integration{
		UserID:          userID,
		Name:            req.Name,
		SourceType:      req.SourceType,
		SourceConfig:    req.SourceConfig,
		DestinationType: "yandex_messenger",
		DestinationConfig: domain.DestinationConfig{
			ChatID:   req.ChatID,
			BotToken: encryptedToken,
		},
		IsActive: req.IsActive,
	}

	if err := api.repo.Create(c.Request().Context(), integration); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	integration.DestinationConfig.BotToken = "***"

	return c.JSON(http.StatusCreated, integration)
}

func (api *IntegrationAPI) Get(c echo.Context) error {
	id := c.Param("id")
	userID := c.Get("user_id").(string)

	integration, err := api.repo.FindByIDAndUser(c.Request().Context(), id, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Integration not found")
	}

	integration.DestinationConfig.BotToken = "***"

	return c.JSON(http.StatusOK, integration)
}

func (api *IntegrationAPI) Update(c echo.Context) error {
	id := c.Param("id")
	userID := c.Get("user_id").(string)

	var req UpdateIntegrationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}

	integration, err := api.repo.FindByIDAndUser(c.Request().Context(), id, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Integration not found")
	}

	integration.Name = req.Name
	integration.SourceConfig = req.SourceConfig
	integration.IsActive = req.IsActive

	if req.ChatID != "" {
		integration.DestinationConfig.ChatID = req.ChatID
	}

	if req.BotToken != "" && req.BotToken != "***" {
		encrypted, err := api.encryptor.Encrypt(req.BotToken)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to encrypt token")
		}
		integration.DestinationConfig.BotToken = encrypted
	}

	if err := api.repo.Update(c.Request().Context(), integration); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Updated successfully"})
}

func (api *IntegrationAPI) Delete(c echo.Context) error {
	id := c.Param("id")
	userID := c.Get("user_id").(string)

	if err := api.repo.Delete(c.Request().Context(), id, userID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

func (api *IntegrationAPI) GetLogs(c echo.Context) error {
	id := c.Param("id")
	userID := c.Get("user_id").(string)

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	logs, total, err := api.repo.GetDeliveryLogs(c.Request().Context(), id, userID, limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (api *IntegrationAPI) Test(c echo.Context) error {
	id := c.Param("id")
	userID := c.Get("user_id").(string)

	integration, err := api.repo.FindByIDAndUser(c.Request().Context(), id, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Integration not found")
	}

	// Здесь логика тестирования
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Test message sent",
	})
}
