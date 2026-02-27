// Путь: internal/service/webhook/jira.go
package webhook

import (
	"encoding/json"
	"net/http"

	"yandex-messenger-bridge/internal/domain"
)

func (h *Handler) HandleJira(w http.ResponseWriter, r *http.Request) {
	integrationID := r.PathValue("id")

	ctx := r.Context()

	integration, err := h.getIntegrationByID(ctx, integrationID)
	if err != nil {
		http.Error(w, "Integration not found", http.StatusNotFound)
		return
	}

	var event domain.JiraWebhook
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid Jira payload", http.StatusBadRequest)
		return
	}

	// Форматируем сообщение
	message := h.formatJiraMessage(&event)

	// Отправляем в Yandex
	if err := h.sendToYandex(ctx, integration, message); err != nil {
		h.logDelivery(integrationID, event, err)
		http.Error(w, "Failed to send", http.StatusInternalServerError)
		return
	}

	h.logDelivery(integrationID, event, nil)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) formatJiraMessage(event *domain.JiraWebhook) string {
	switch event.WebhookEvent {
	case "jira:issue_created":
		return h.formatJiraIssueCreated(event)
	case "jira:issue_updated":
		return h.formatJiraIssueUpdated(event)
	case "comment_created":
		return h.formatJiraCommentCreated(event)
	default:
		return h.formatJiraGeneric(event)
	}
}

func (h *Handler) formatJiraIssueCreated(event *domain.JiraWebhook) string {
	return "🆕 Новая задача: " + event.Issue.Key + " - " + event.Issue.Fields.Summary
}

func (h *Handler) formatJiraIssueUpdated(event *domain.JiraWebhook) string {
	return "✏️ Обновлена задача: " + event.Issue.Key
}

func (h *Handler) formatJiraCommentCreated(event *domain.JiraWebhook) string {
	return "💬 Новый комментарий в задаче " + event.Issue.Key
}

func (h *Handler) formatJiraGeneric(event *domain.JiraWebhook) string {
	return "📋 Событие Jira: " + event.WebhookEvent
}
