package domain

import "time"

// GitLabWebhook - базовая структура для всех GitLab событий
type GitLabWebhook struct {
	ObjectKind   string  `json:"object_kind"`
	EventType    string  `json:"event_type"`
	Project      Project `json:"project"`
	UserID       int     `json:"user_id"`
	UserName     string  `json:"user_name"`
	UserEmail    string  `json:"user_email"`
	UserUsername string  `json:"user_username"`
}

// Project - информация о проекте GitLab
type Project struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	WebURL            string `json:"web_url"`
	PathWithNamespace string `json:"path_with_namespace"`
}

// Commit - коммит GitLab
type Commit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Author  struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
	Timestamp time.Time `json:"timestamp"`
}

// PushEvent - событие push
type PushEvent struct {
	GitLabWebhook
	Ref               string   `json:"ref"`
	Before            string   `json:"before"`
	After             string   `json:"after"`
	Commits           []Commit `json:"commits"`
	TotalCommitsCount int      `json:"total_commits_count"`
}

// MergeRequest - информация о merge request
type MergeRequest struct {
	ID    int    `json:"id"`
	IID   int    `json:"iid"`
	Title string `json:"title"`
	URL   string `json:"url"`
	State string `json:"state"`
}

// Issue - задача GitLab
type Issue struct {
	ID    int    `json:"id"`
	IID   int    `json:"iid"`
	Title string `json:"title"`
	URL   string `json:"url"`
	State string `json:"state"`
}

// MergeRequestEvent - событие merge request
type MergeRequestEvent struct {
	GitLabWebhook
	ObjectAttributes struct {
		ID           int       `json:"id"`
		IID          int       `json:"iid"`
		Title        string    `json:"title"`
		Description  string    `json:"description"`
		State        string    `json:"state"`
		Action       string    `json:"action"`
		SourceBranch string    `json:"source_branch"`
		TargetBranch string    `json:"target_branch"`
		URL          string    `json:"url"`
		CreatedAt    time.Time `json:"created_at"`
	} `json:"object_attributes"`
	Assignee User `json:"assignee"`
	Reviewer User `json:"reviewer"`
}

// PipelineEvent - событие pipeline
type PipelineEvent struct {
	GitLabWebhook
	ObjectAttributes struct {
		ID       int      `json:"id"`
		Ref      string   `json:"ref"`
		Tag      bool     `json:"tag"`
		SHA      string   `json:"sha"`
		Status   string   `json:"status"`
		Stages   []string `json:"stages"`
		Duration int      `json:"duration"`
		URL      string   `json:"url"`
	} `json:"object_attributes"`
	MergeRequest *MergeRequest `json:"merge_request"`
}

// CommentEvent - событие комментария
type CommentEvent struct {
	GitLabWebhook
	ObjectAttributes struct {
		ID           int       `json:"id"`
		Note         string    `json:"note"`
		NoteableType string    `json:"noteable_type"`
		CreatedAt    time.Time `json:"created_at"`
	} `json:"object_attributes"`
	Issue        *Issue        `json:"issue"`
	MergeRequest *MergeRequest `json:"merge_request"`
	Commit       *Commit       `json:"commit"`
}

// GitLabConfig - конфигурация для GitLab webhook
type GitLabConfig struct {
	SecretToken   string   `json:"secret_token"`
	Events        []string `json:"events"`
	BranchFilter  string   `json:"branch_filter"`
	ProjectFilter []string `json:"project_filter"`
	Templates     struct {
		Push         string `json:"push"`
		MergeRequest string `json:"merge_request"`
		Pipeline     string `json:"pipeline"`
		Issue        string `json:"issue"`
		Comment      string `json:"comment"`
	} `json:"templates"`
}
