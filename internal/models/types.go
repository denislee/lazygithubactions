package models

import "time"

type Repo struct {
	Name      string    `json:"name"`
	Owner     string    `json:"owner"`
	FullName  string    // "owner/name"
	UpdatedAt time.Time `json:"updatedAt"`
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
}

type Job struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Steps      []Step `json:"steps"`
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

type RunDetail struct {
	WorkflowRun
	Jobs []Job `json:"jobs"`
}
