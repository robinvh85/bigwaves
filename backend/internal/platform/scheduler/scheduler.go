// Package scheduler runs the periodic jobs in-process. Every run is recorded in job_runs; a
// failing job only affects itself (the Rails app ran unrelated steps in one job, so the first
// failure skipped the rest).
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"

	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/events"
	"bigwaves/internal/platform/notify"
)

// Job is a named periodic task.
type Job struct {
	Name        string
	Description string
	// Cron has 6 fields with seconds: "20 * * * * *". Prefix "CRON_TZ=Asia/Ho_Chi_Minh " to pin a
	// time zone; without it the scheduler runs in UTC.
	Cron    string
	Timeout time.Duration
	Run     func(ctx context.Context) (string, error)
}

// ErrSkip, returned (wrapped) by Run, records the run as SKIPPED instead of FAILED: the job had
// nothing to do or is switched off.
var ErrSkip = errors.New("skipped")

func Skipf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrSkip, fmt.Sprintf(format, args...))
}

type Scheduler struct {
	db      *db.DB
	alerter *notify.Alerter
	events  *events.Broker

	mu      sync.Mutex
	jobs    map[string]*entry
	cron    gocron.Scheduler
	baseCtx context.Context
}

type entry struct {
	job     Job
	running sync.Mutex
	cronJob gocron.Job
}

var ErrBusy = errors.New("job is already running")
var ErrUnknownJob = errors.New("unknown job")

func New(d *db.DB, alerter *notify.Alerter, ev *events.Broker) *Scheduler {
	return &Scheduler{db: d, alerter: alerter, events: ev, jobs: map[string]*entry{}, baseCtx: context.Background()}
}

func (s *Scheduler) Register(jobs ...Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range jobs {
		if j.Timeout == 0 {
			j.Timeout = 10 * time.Minute
		}
		s.jobs[j.Name] = &entry{job: j}
	}
}

// Start schedules every registered job. Runs left RUNNING by a crash are marked FAILED first.
func (s *Scheduler) Start(ctx context.Context) error {
	s.baseCtx = ctx
	if err := dbq.New(s.db.Write).MarkStaleJobRuns(ctx, ptr(dbtype.Now())); err != nil {
		return err
	}
	c, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.jobs {
		e := e
		j, err := c.NewJob(
			gocron.CronJob(e.job.Cron, true),
			gocron.NewTask(func() { _ = s.execute(e, "schedule") }),
			gocron.WithName(e.job.Name),
			gocron.WithSingletonMode(gocron.LimitModeReschedule),
		)
		if err != nil {
			return fmt.Errorf("schedule %s (%q): %w", e.job.Name, e.job.Cron, err)
		}
		e.cronJob = j
	}
	s.cron = c
	c.Start()
	slog.Info("scheduler started", "jobs", len(s.jobs))
	return nil
}

func (s *Scheduler) Stop() error {
	if s.cron == nil {
		return nil
	}
	return s.cron.Shutdown()
}

// RunNow starts a job in the background (manual trigger from the UI).
func (s *Scheduler) RunNow(name string) error {
	s.mu.Lock()
	e, ok := s.jobs[name]
	s.mu.Unlock()
	if !ok {
		return ErrUnknownJob
	}
	if !e.running.TryLock() {
		return ErrBusy
	}
	e.running.Unlock()
	go func() { _ = s.execute(e, "manual") }()
	return nil
}

// RunSync runs a job and waits (used by bwctl and tests).
func (s *Scheduler) RunSync(name string) error {
	s.mu.Lock()
	e, ok := s.jobs[name]
	s.mu.Unlock()
	if !ok {
		return ErrUnknownJob
	}
	return s.execute(e, "manual")
}

func (s *Scheduler) execute(e *entry, trigger string) error {
	if !e.running.TryLock() {
		return ErrBusy
	}
	defer e.running.Unlock()

	ctx, cancel := context.WithTimeout(s.baseCtx, e.job.Timeout)
	defer cancel()
	q := dbq.New(s.db.Write)
	start := time.Now()
	runID, err := q.StartJobRun(ctx, dbq.StartJobRunParams{Job: e.job.Name, Trigger: trigger, StartedAt: dbtype.NewTime(start)})
	if err != nil {
		slog.Error("start job run", "job", e.job.Name, "err", err)
		return err
	}

	msg, runErr := safeRun(ctx, e.job)
	status := "OK"
	switch {
	case errors.Is(runErr, ErrSkip):
		status, msg = "SKIPPED", runErr.Error()
		runErr = nil
	case runErr != nil:
		status, msg = "FAILED", runErr.Error()
	}
	// Record the result even if the job's context timed out.
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finishCancel()
	if err := q.FinishJobRun(finishCtx, dbq.FinishJobRunParams{
		Status: status, Message: msg, FinishedAt: ptr(dbtype.Now()), ID: runID,
	}); err != nil {
		slog.Error("finish job run", "job", e.job.Name, "err", err)
	}
	slog.Info("job finished", "job", e.job.Name, "status", status, "dur_ms", time.Since(start).Milliseconds(), "msg", msg)
	if runErr != nil && s.alerter != nil {
		s.alerter.Report(finishCtx, "job "+e.job.Name, runErr)
	}
	s.events.Publish("job.finished", map[string]string{"job": e.job.Name, "status": status})
	return runErr
}

func safeRun(ctx context.Context, j Job) (msg string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return j.Run(ctx)
}

type JobInfo struct {
	Name        string
	Description string
	Cron        string
	Running     bool
	NextRun     *time.Time
}

// Jobs lists the registered jobs, sorted by name.
func (s *Scheduler) Jobs() []JobInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]JobInfo, 0, len(s.jobs))
	for _, e := range s.jobs {
		info := JobInfo{Name: e.job.Name, Description: e.job.Description, Cron: e.job.Cron}
		if e.running.TryLock() {
			e.running.Unlock()
		} else {
			info.Running = true
		}
		if e.cronJob != nil {
			if t, err := e.cronJob.NextRun(); err == nil && !t.IsZero() {
				info.NextRun = &t
			}
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func ptr[T any](v T) *T { return &v }
