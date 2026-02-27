package formatter

import (
	"fmt"
	"strings"
	"time"

	"yandex-messenger-bridge/internal/domain"
)

// JiraFormatter форматирует сообщения из Jira
type JiraFormatter struct{}

// NewJiraFormatter создает новый форматтер
func NewJiraFormatter() *JiraFormatter {
	return &JiraFormatter{}
}

// FormatIssueCreated форматирует создание задачи
func (f *JiraFormatter) FormatIssueCreated(event *domain.JiraWebhook, config *domain.JiraConfig) string {
	issue := event.Issue
	user := event.User.DisplayName

	template := "🆕 *{user}* created issue [{key}]({url}): *{summary}*\n" +
		"Priority: {priority} | Status: {status}"

	if config != nil && config.Template != "" {
		template = config.Template
	}

	return strings.NewReplacer(
		"{user}", user,
		"{key}", issue.Key,
		"{url}", issue.Self,
		"{summary}", issue.Fields.Summary,
		"{priority}", issue.Fields.Priority.Name,
		"{status}", issue.Fields.Status.Name,
	).Replace(template)
}

// FormatIssueUpdated форматирует обновление задачи
func (f *JiraFormatter) FormatIssueUpdated(event *domain.JiraWebhook, config *domain.JiraConfig) string {
	issue := event.Issue
	user := event.User.DisplayName

	// Если есть changelog, показываем что изменилось
	var changes []string
	if event.Changelog != nil {
		for _, item := range event.Changelog.Items {
			changes = append(changes, fmt.Sprintf("%s: %s → %s",
				item.Field, item.FromString, item.ToString))
		}
	}

	template := "✏️ *{user}* updated [{key}]({url})\n"
	if len(changes) > 0 {
		template += "Changes: " + strings.Join(changes, ", ")
	}

	return strings.NewReplacer(
		"{user}", user,
		"{key}", issue.Key,
		"{url}", issue.Self,
	).Replace(template)
}

// FormatCommentCreated форматирует новый комментарий
func (f *JiraFormatter) FormatCommentCreated(event *domain.JiraWebhook, config *domain.JiraConfig) string {
	issue := event.Issue
	user := event.User.DisplayName
	comment := event.Comment

	template := "💬 *{user}* commented on [{key}]({url}):\n> {comment}"

	return strings.NewReplacer(
		"{user}", user,
		"{key}", issue.Key,
		"{url}", issue.Self,
		"{comment}", comment.Body,
	).Replace(template)
}
