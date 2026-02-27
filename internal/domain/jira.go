// Путь: internal/domain/jira.go
package domain

import "time"

// JiraWebhook - базовая структура для Jira вебхуков
type JiraWebhook struct {
	Timestamp    int64          `json:"timestamp"`
	WebhookEvent string         `json:"webhookEvent"`
	Issue        JiraIssue      `json:"issue"`
	User         JiraUser       `json:"user"`
	Changelog    *JiraChangelog `json:"changelog,omitempty"`
	Comment      *JiraComment   `json:"comment,omitempty"`
}

// JiraIssue - задача Jira
type JiraIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary     string `json:"summary"`
		Description string `json:"description"`
		Status      struct {
			Name string `json:"name"`
		} `json:"status"`
		Priority struct {
			Name string `json:"name"`
		} `json:"priority"`
		Assignee *JiraUser `json:"assignee"`
		Reporter *JiraUser `json:"reporter"`
		Created  time.Time `json:"created"`
		Updated  time.Time `json:"updated"`
	} `json:"fields"`
	Self string `json:"self"`
}

// JiraUser - пользователь Jira
type JiraUser struct {
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
	Active       bool   `json:"active"`
}

// JiraComment - комментарий Jira
type JiraComment struct {
	ID      string    `json:"id"`
	Body    string    `json:"body"`
	Author  JiraUser  `json:"author"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// JiraChangelog - история изменений
type JiraChangelog struct {
	ID    string `json:"id"`
	Items []struct {
		Field      string      `json:"field"`
		FieldType  string      `json:"fieldtype"`
		From       interface{} `json:"from"`
		FromString string      `json:"fromString"`
		To         interface{} `json:"to"`
		ToString   string      `json:"toString"`
	} `json:"items"`
}

// JiraConfig - конфигурация для Jira
type JiraConfig struct {
	ProjectKeys  []string               `json:"project_keys"`
	MinPriority  string                 `json:"min_priority"`
	Events       map[string]bool        `json:"events"`
	Template     string                 `json:"template"`
	CustomFields map[string]interface{} `json:"custom_fields"`
}

// ShouldProcess проверяет, нужно ли обрабатывать событие
func (c *JiraConfig) ShouldProcess(event string, priority string, project string) bool {
	// Проверка события
	if len(c.Events) > 0 {
		if !c.Events[event] {
			return false
		}
	}

	// Проверка проекта
	if len(c.ProjectKeys) > 0 {
		found := false
		for _, p := range c.ProjectKeys {
			if p == project {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Проверка приоритета
	if c.MinPriority != "" {
		if !isPriorityMet(priority, c.MinPriority) {
			return false
		}
	}

	return true
}

// Приоритеты Jira: Highest, High, Medium, Low, Lowest
func isPriorityMet(current, min string) bool {
	levels := map[string]int{
		"Highest": 5,
		"High":    4,
		"Medium":  3,
		"Low":     2,
		"Lowest":  1,
	}
	return levels[current] >= levels[min]
}
