package models

import "time"

type Repo struct {
	Name      string    `json:"name"`
	Owner     string    `json:"owner"`
	FullName  string    // "owner/name"
	UpdatedAt time.Time `json:"updatedAt"`
}

type Commit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  struct {
		Name  string    `json:"name"`
		Date  time.Time `json:"date"`
	} `json:"author"`
}

type WorkflowRun struct {
	ID           int64     `json:"databaseId"`
	Name         string    `json:"name"`
	DisplayTitle string    `json:"displayTitle"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	Branch       string    `json:"headBranch"`
	Event        string    `json:"event"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	URL          string    `json:"url"`
	WorkflowName string   `json:"workflowName"`
	Attempt      int       `json:"attempt"`
	Actor        string    `json:"actor"` // username who triggered
}

type Job struct {
	ID          int64     `json:"id"`
	DatabaseID  int64     `json:"databaseId"`
	URL         string    `json:"url"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
	Steps       []Step    `json:"steps"`
}

type Step struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Number     int    `json:"number"`
}

type Workflow struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
	Path  string `json:"path"`
}

type WorkflowInput struct {
	Name        string   // key from the YAML map
	Description string   `yaml:"description"`
	Required    bool     `yaml:"required"`
	Default     string   `yaml:"default"`
	Type        string   `yaml:"type"` // "string", "choice", "boolean", "environment"
	Options     []string `yaml:"options"`
}

type RunDetail struct {
	WorkflowRun
	Jobs []Job `json:"jobs"`
}
