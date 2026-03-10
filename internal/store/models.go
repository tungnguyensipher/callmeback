package store

import "time"

const DefaultProfile = "default"

type ScheduleType string

const (
	ScheduleTypeInterval ScheduleType = "interval"
	ScheduleTypeOneTime  ScheduleType = "onetime"
	ScheduleTypeCron     ScheduleType = "cron"
)

type JobStatus string

const (
	StatusActive JobStatus = "active"
	StatusPaused JobStatus = "paused"
)

type Job struct {
	ID           string
	Name         string
	Profile      string
	ScheduleType ScheduleType
	Schedule     string
	Command      []string
	Status       JobStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateJobParams struct {
	Name         string
	Profile      string
	ScheduleType ScheduleType
	Schedule     string
	Command      []string
}

type UpdateJobParams struct {
	Name         *string
	Profile      *string
	ScheduleType *ScheduleType
	Schedule     *string
	Command      []string
	Status       *JobStatus
}

type ListJobsParams struct {
	Profile     string
	AllProfiles bool
}

type RunRequest struct {
	ID          int64
	JobID       string
	RequestedAt time.Time
	ProcessedAt *time.Time
}

type JobRun struct {
	ID          int64
	JobID       string
	TriggerType string
	StartedAt   time.Time
	FinishedAt  *time.Time
	ExitCode    *int
	Stdout      string
	Stderr      string
	ErrorText   string
}

type CreateJobRunParams struct {
	JobID       string
	TriggerType string
	StartedAt   time.Time
	FinishedAt  *time.Time
	ExitCode    *int
	Stdout      string
	Stderr      string
	ErrorText   string
}
