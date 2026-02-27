package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

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

// HandleGitLab обрабатывает вебхуки от GitLab
func (h *Handler) HandleGitLab(w http.ResponseWriter, r *http.Request) {
	integrationID := r.PathValue("id")

	ctx, cancel := context.WithTimeout(r.Context(), h.config.GitLabTimeout)
	defer cancel()
	r = r.WithContext(ctx)

	integration, err := h.getIntegrationByID(ctx, integrationID)
	if err != nil {
		log.Error().Err(err).Str("id", integrationID).Msg("Integration not found")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	body, err := h.readBody(r)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read body")
		w.WriteHeader(http.StatusOK)
		return
	}

	gitlabConfig := &domain.GitLabConfig{}
	if err := mapToStruct(integration.SourceConfig, gitlabConfig); err != nil {
		log.Error().Err(err).Msg("Failed to parse GitLab config")
	}

	if gitlabConfig.SecretToken != "" {
		providedToken := r.Header.Get("X-Gitlab-Token")
		if providedToken != gitlabConfig.SecretToken {
			log.Warn().Str("integration", integrationID).Msg("Invalid GitLab token")
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	eventType := r.Header.Get("X-Gitlab-Event")

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
	}

	if message != "" {
		if err := h.sendToYandex(ctx, integration, message); err != nil {
			log.Error().Err(err).Msg("Failed to send to Yandex")
			go h.retrySend(integration, message, 0)
		}
	}

	h.logDelivery(integrationID, event, err)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// shouldProcessGitLabPush проверяет, нужно ли обрабатывать push
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

	if config.BranchFilter != "" {
		branch := strings.TrimPrefix(event.Ref, "refs/heads/")
		return h.matchBranch(branch, config.BranchFilter)
	}

	return true
}

// shouldProcessGitLabMR проверяет, нужно ли обрабатывать merge request
func (h *Handler) shouldProcessGitLabMR(event *domain.MergeRequestEvent, config *domain.GitLabConfig) bool {
	if len(config.Events) > 0 {
		for _, e := range config.Events {
			if e == "merge_request" {
				return true
			}
		}
		return false
	}
	return true
}

// shouldProcessGitLabComment проверяет, нужно ли обрабатывать комментарий
func (h *Handler) shouldProcessGitLabComment(event *domain.CommentEvent, config *domain.GitLabConfig) bool {
	if len(config.Events) > 0 {
		for _, e := range config.Events {
			if e == "comment" {
				return true
			}
		}
		return false
	}
	return true
}

// shouldProcessGitLabPipeline проверяет, нужно ли обрабатывать pipeline
func (h *Handler) shouldProcessGitLabPipeline(event *domain.PipelineEvent, config *domain.GitLabConfig) bool {
	if len(config.Events) > 0 {
		for _, e := range config.Events {
			if e == "pipeline" {
				return true
			}
		}
		return false
	}
	return true
}

// formatGitLabPush форматирует push событие
func (h *Handler) formatGitLabPush(event *domain.PushEvent, config *domain.GitLabConfig) string {
	branch := strings.TrimPrefix(event.Ref, "refs/heads/")
	commits := len(event.Commits)

	template := "📦 *{user}* pushed {commits} commit(s) to [{project}]({project_url}) branch `{branch}`\n"
	if config != nil && config.Templates.Push != "" {
		template = config.Templates.Push
	}

	msg := strings.NewReplacer(
		"{user}", event.UserName,
		"{commits}", fmt.Sprintf("%d", commits),
		"{project}", event.Project.Name,
		"{project_url}", event.Project.WebURL,
		"{branch}", branch,
	).Replace(template)

	for i, commit := range event.Commits {
		if i >= 3 {
			msg += fmt.Sprintf("\n  ... и еще %d", commits-3)
			break
		}
		msg += fmt.Sprintf("\n  • [`%s`](%s) %s", commit.ID[:8], commit.URL, commit.Title)
	}

	return msg
}

// formatGitLabMergeRequest форматирует merge request событие
func (h *Handler) formatGitLabMergeRequest(event *domain.MergeRequestEvent, config *domain.GitLabConfig) string {
	var emoji string
	switch event.ObjectAttributes.Action {
	case "open":
		emoji = "🆕"
	case "merge":
		emoji = "✅"
	case "close":
		emoji = "❌"
	default:
		emoji = "🔄"
	}

	template := "{emoji} Merge Request {action} by {user}: [*{title}*]({url}) in {project}\n" +
		"`{source}` → `{target}`"

	if config != nil && config.Templates.MergeRequest != "" {
		template = config.Templates.MergeRequest
	}

	return strings.NewReplacer(
		"{emoji}", emoji,
		"{action}", event.ObjectAttributes.Action,
		"{user}", event.UserName,
		"{title}", event.ObjectAttributes.Title,
		"{url}", event.ObjectAttributes.URL,
		"{project}", event.Project.Name,
		"{source}", event.ObjectAttributes.SourceBranch,
		"{target}", event.ObjectAttributes.TargetBranch,
	).Replace(template)
}

// formatGitLabComment форматирует комментарий
func (h *Handler) formatGitLabComment(event *domain.CommentEvent, config *domain.GitLabConfig) string {
	var target string
	switch event.ObjectAttributes.NoteableType {
	case "Issue":
		if event.Issue != nil {
			target = fmt.Sprintf("[issue #%d](%s)", event.Issue.IID, event.Issue.URL)
		}
	case "MergeRequest":
		if event.MergeRequest != nil {
			target = fmt.Sprintf("[merge request !%d](%s)", event.MergeRequest.IID, event.MergeRequest.URL)
		}
	case "Commit":
		if event.Commit != nil {
			target = fmt.Sprintf("[commit](%s)", event.Commit.URL)
		}
	}

	template := "💬 {user} commented on {target} in {project}:\n> {comment}"

	if config != nil && config.Templates.Comment != "" {
		template = config.Templates.Comment
	}

	return strings.NewReplacer(
		"{user}", event.UserName,
		"{target}", target,
		"{project}", event.Project.Name,
		"{comment}", event.ObjectAttributes.Note,
	).Replace(template)
}

// formatGitLabPipeline форматирует pipeline событие
func (h *Handler) formatGitLabPipeline(event *domain.PipelineEvent, config *domain.GitLabConfig) string {
	var emoji string
	switch event.ObjectAttributes.Status {
	case "success":
		emoji = "✅"
	case "failed":
		emoji = "❌"
	case "running":
		emoji = "🔄"
	default:
		emoji = "⏳"
	}

	template := "{emoji} Pipeline {status} for [{project}]({project_url}) on `{ref}`\n" +
		"Duration: {duration}s"

	if config != nil && config.Templates.Pipeline != "" {
		template = config.Templates.Pipeline
	}

	return strings.NewReplacer(
		"{emoji}", emoji,
		"{status}", event.ObjectAttributes.Status,
		"{project}", event.Project.Name,
		"{project_url}", event.Project.WebURL,
		"{ref}", event.ObjectAttributes.Ref,
		"{duration}", fmt.Sprintf("%d", event.ObjectAttributes.Duration),
	).Replace(template)
}

// matchBranch проверяет соответствие ветки паттерну
func (h *Handler) matchBranch(branch, pattern string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(branch, parts[0]) && strings.HasSuffix(branch, parts[1])
		}
	}
	return branch == pattern
}
