package webhook

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"yandex-messenger-bridge/internal/domain"
)

// HandleGitLab обрабатывает вебхуки от GitLab
func (h *Handler) HandleGitLab(w http.ResponseWriter, r *http.Request) {
	integrationID := r.PathValue("id")

	// Устанавливаем таймаут для GitLab (они ждут ответ только 10 секунд)
	ctx, cancel := context.WithTimeout(r.Context(), h.config.GitLabTimeout)
	defer cancel()
	r = r.WithContext(ctx)

	// Загружаем интеграцию
	integration, err := h.getIntegrationByID(ctx, integrationID)
	if err != nil {
		log.Error().Err(err).Str("id", integrationID).Msg("Integration not found")
		// GitLab отключит webhook после 4 ошибок, поэтому всегда отвечаем 200
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	// Читаем тело запроса
	body, err := h.readBody(r)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read body")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Извлекаем конфигурацию GitLab из source_config
	gitlabConfig := &domain.GitLabConfig{}
	if err := mapToStruct(integration.SourceConfig, gitlabConfig); err != nil {
		log.Error().Err(err).Msg("Failed to parse GitLab config")
	}

	// Проверка секретного токена (улучшение #1)
	if gitlabConfig.SecretToken != "" {
		providedToken := r.Header.Get("X-Gitlab-Token")
		if providedToken != gitlabConfig.SecretToken {
			log.Warn().Str("integration", integrationID).Msg("Invalid GitLab token")
			w.WriteHeader(http.StatusOK) // Всегда 200, но не обрабатываем
			return
		}
	}

	// Определяем тип события
	eventType := r.Header.Get("X-Gitlab-Event")

	// Обрабатываем в зависимости от типа
	var message string
	var event interface{}

	switch eventType {
	case "Push Hook":
		var e domain.PushEvent
		if err := json.Unmarshal(body, &e); err == nil {
			event = e
			if h.shouldProcessGitLabPush(&e, gitlabConfig) {
				message = h.formatGitLabPush(&e, gitlabConfig)
			}
		}
	case "Merge Request Hook":
		var e domain.MergeRequestEvent
		if err := json.Unmarshal(body, &e); err == nil {
			event = e
			if h.shouldProcessGitLabMR(&e, gitlabConfig) {
				message = h.formatGitLabMergeRequest(&e, gitlabConfig)
			}
		}
	case "Note Hook":
		var e domain.CommentEvent
		if err := json.Unmarshal(body, &e); err == nil {
			event = e
			if h.shouldProcessGitLabComment(&e, gitlabConfig) {
				message = h.formatGitLabComment(&e, gitlabConfig)
			}
		}
	case "Pipeline Hook":
		var e domain.PipelineEvent
		if err := json.Unmarshal(body, &e); err == nil {
			event = e
			if h.shouldProcessGitLabPipeline(&e, gitlabConfig) {
				message = h.formatGitLabPipeline(&e, gitlabConfig)
			}
		}
	default:
		// Пробуем определить по object_kind
		var base domain.GitLabWebhook
		if err := json.Unmarshal(body, &base); err == nil {
			log.Info().Str("kind", base.ObjectKind).Msg("Unhandled GitLab event")
		}
	}

	// Отправляем в Yandex если есть сообщение
	if message != "" {
		if err := h.sendToYandex(ctx, integration, message); err != nil {
			log.Error().Err(err).Msg("Failed to send to Yandex")
			// Пытаемся отправить повторно (улучшение #2)
			go h.retrySend(integration, message, 0)
		}
	}

	// GitLab ожидает быстрого ответа
	h.logDelivery(integrationID, event, err)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// shouldProcessGitLabPush проверяет, нужно ли обрабатывать push (фильтр по веткам)
func (h *Handler) shouldProcessGitLabPush(event *domain.PushEvent, config *domain.GitLabConfig) bool {
	if len(config.Events) > 0 {
		found := false
		for _, e := range config.Events {
			if e == "push" {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Фильтр по веткам (улучшение #3)
	if config.BranchFilter != "" {
		branch := strings.TrimPrefix(event.Ref, "refs/heads/")
		return matchBranch(branch, config.BranchFilter)
	}

	// Фильтр по проектам
	if len(config.ProjectFilter) > 0 {
		for _, p := range config.ProjectFilter {
			if matchProject(event.Project.PathWithNamespace, p) {
				return true
			}
		}
		return false
	}

	return true
}

// matchBranch проверяет соответствие ветки паттерну (поддерживает *)
func matchBranch(branch, pattern string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		// Простая поддержка wildcard
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(branch, parts[0]) && strings.HasSuffix(branch, parts[1])
		}
	}
	return branch == pattern
}

// retrySend повторяет отправку с экспоненциальной задержкой (улучшение #4)
func (h *Handler) retrySend(integration *domain.Integration, message string, attempt int) {
	if attempt >= h.config.MaxRetries {
		log.Error().Int("attempts", attempt).Msg("Max retries reached")
		return
	}

	// Экспоненциальная задержка: 1s, 2s, 4s
	delay := time.Duration(1<<uint(attempt)) * time.Second
	time.Sleep(delay)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.sendToYandex(ctx, integration, message); err != nil {
		log.Error().Err(err).Int("attempt", attempt+1).Msg("Retry failed")
		h.retrySend(integration, message, attempt+1)
	}
}
