package formatter

import (
	"fmt"
	"strings"

	"yandex-messenger-bridge/internal/domain"
)

// GitLabFormatter форматирует сообщения из GitLab
type GitLabFormatter struct{}

// NewGitLabFormatter создает новый форматтер
func NewGitLabFormatter() *GitLabFormatter {
	return &GitLabFormatter{}
}

// FormatPush форматирует push событие
func (f *GitLabFormatter) FormatPush(event *domain.PushEvent, config *domain.GitLabConfig) string {
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

	// Добавляем первые 3 коммита
	for i, commit := range event.Commits {
		if i >= 3 {
			msg += fmt.Sprintf("\n  ... и еще %d", commits-3)
			break
		}
		msg += fmt.Sprintf("\n  • [`%s`](%s) %s", commit.ID[:8], commit.URL, commit.Title)
	}

	return msg
}

// FormatMergeRequest форматирует merge request событие
func (f *GitLabFormatter) FormatMergeRequest(event *domain.MergeRequestEvent, config *domain.GitLabConfig) string {
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

// FormatPipeline форматирует pipeline событие
func (f *GitLabFormatter) FormatPipeline(event *domain.PipelineEvent, config *domain.GitLabConfig) string {
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

// FormatComment форматирует комментарий
func (f *GitLabFormatter) FormatComment(event *domain.CommentEvent, config *domain.GitLabConfig) string {
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
