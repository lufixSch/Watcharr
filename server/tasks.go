package main

import (
	"errors"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"gorm.io/gorm"
)

type TaskRescheduleRequest struct {
	// Number of seconds inbetween each run of this task.
	Seconds int `json:"seconds" binding:"required"`
}

type AllTasksResponse struct {
	// The tasks name.
	Name string `json:"name"`
	// When this task will next run.
	NextRun time.Time `json:"nextRun"`
}

var taskScheduler gocron.Scheduler

// All task functions are stored here so when updating (rescheduling)
// a job, we can give it this function again.
// Doesn't seem to be a way to only update the schedule of a job,
// the .Update func wants the whole definition again.
//
// All funcs simply call a cleaning/routine method where the rest of the
// related code lives so it's kept tidy.
var taskFuncs map[string]func()

// Setup recurring tasks (eg cleanup every x mins)
func setupTasks(db *gorm.DB) {
	ts, err := gocron.NewScheduler()
	if err != nil {
		slog.Error("SetupTasks: Failed to create new scheduler!", "error", err)
		return
	}
	taskScheduler = ts

	// Define all task funcs.
	taskFuncs = map[string]func(){
		"Cleanup Tokens": func() {
			cleanupTokens(db)
		},
		"Refresh Arr Queues": func() {
			refreshArrQueues()
		},
		"Cleanup Images": func() {
			cleanupImages(db)
		},
	}

	// Add all jobs to scheduler.
	err = addTaskToScheduler("Cleanup Tokens", 60*time.Second)
	if err != nil {
		slog.Error("SetupTasks: Failed to add new job", "job", "Cleanup Tokens", "err", err)
	}
	err = addTaskToScheduler("Refresh Arr Queues", 60*time.Second)
	if err != nil {
		slog.Error("SetupTasks: Failed to add new job", "job", "Refresh Arr Queues", "err", err)
	}
	err = addTaskToScheduler("Cleanup Images", 24*time.Hour)
	if err != nil {
		slog.Error("SetupTasks: Failed to add new job", "job", "Cleanup Images", "err", err)
	}

	taskScheduler.Start()
	slog.Info("SetupTasks: Jobs created and scheduler started.")
}

// Small helper to add a new job to the scheduler.
// Makes the setupTasks function a little easier to read.
// Gets schedule from config, or `defaultDur` if not manually configured.
func addTaskToScheduler(name string, defaultDur time.Duration) error {
	s := defaultDur
	if Config.TASK_SCHEDULE[name] != 0 {
		s = time.Duration(Config.TASK_SCHEDULE[name]) * time.Second
	}
	_, err := taskScheduler.NewJob(
		gocron.DurationJob(s),
		gocron.NewTask(taskFuncs[name]),
		gocron.WithName(name),
	)
	slog.Debug("addTaskToScheduler: Job added.", "job_name", name, "duration_used", s, "duration_default", defaultDur)
	return err
}

// Get all tasks in a consumable format.
func getAllTasks() []AllTasksResponse {
	jobs := []AllTasksResponse{}
	for _, j := range taskScheduler.Jobs() {
		j2a := AllTasksResponse{
			Name: j.Name(),
		}
		nextRun, err := j.NextRun()
		if err != nil {
			slog.Error("getAllTasks: Failed to get next run time for a job.", "job_name", j2a.Name)
		} else {
			j2a.NextRun = nextRun
		}
		jobs = append(jobs, j2a)
	}
	return jobs
}

// Get task (job) from scheduler by name.
func getTask(name string) *gocron.Job {
	var job *gocron.Job
	for _, j := range taskScheduler.Jobs() {
		if j.Name() == name {
			job = &j
			break
		}
	}
	return job
}

// Reschedule a task by name.
func rescheduleTask(name string, req TaskRescheduleRequest) error {
	j := getTask(name)
	if j == nil {
		return errors.New("no task found")
	}
	_, err := taskScheduler.Update(
		(*j).ID(),
		gocron.DurationJob(
			time.Duration(req.Seconds)*time.Second,
		),
		gocron.NewTask(
			taskFuncs[name],
		),
		gocron.WithName(name),
	)
	if err != nil {
		slog.Error("rescheduleTask: Failed to update job!", "error", err)
		return errors.New("failed to update job")
	}
	return nil
}
