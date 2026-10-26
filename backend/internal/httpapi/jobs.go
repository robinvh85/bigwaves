package httpapi

import (
	"context"
	"errors"
	"net/http"

	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/platform/scheduler"
)

func toJobRun(r dbq.JobRun) JobRun {
	out := JobRun{
		Id: r.ID, Job: r.Job, Trigger: JobRunTrigger(r.Trigger), Status: JobRunStatus(r.Status),
		Message: r.Message, StartedAt: r.StartedAt.Time,
	}
	if r.FinishedAt != nil {
		out.FinishedAt = &r.FinishedAt.Time
	}
	return out
}

func (s *Server) ListJobs(ctx context.Context, _ ListJobsRequestObject) (ListJobsResponseObject, error) {
	last, err := dbq.New(s.DB.Read).LatestFinishedJobRuns(ctx)
	if err != nil {
		return nil, err
	}
	byJob := make(map[string]dbq.JobRun, len(last))
	for _, r := range last {
		byJob[r.Job] = r
	}
	jobs := s.Scheduler.Jobs()
	items := make([]JobStatus, 0, len(jobs))
	for _, j := range jobs {
		st := JobStatus{Name: j.Name, Description: j.Description, Cron: j.Cron, Running: j.Running, NextRunAt: j.NextRun}
		if r, ok := byJob[j.Name]; ok {
			jr := toJobRun(r)
			st.LastRun = &jr
		}
		items = append(items, st)
	}
	return ListJobs200JSONResponse{Items: items}, nil
}

func (s *Server) ListJobRuns(ctx context.Context, req ListJobRunsRequestObject) (ListJobRunsResponseObject, error) {
	limit := int64(50)
	if req.Params.Limit != nil {
		limit = int64(*req.Params.Limit)
	}
	rows, err := dbq.New(s.DB.Read).ListJobRuns(ctx, dbq.ListJobRunsParams{Job: req.Name, Limit: limit})
	if err != nil {
		return nil, err
	}
	items := make([]JobRun, len(rows))
	for i, r := range rows {
		items[i] = toJobRun(r)
	}
	return ListJobRuns200JSONResponse{Items: items}, nil
}

func (s *Server) RunJob(_ context.Context, req RunJobRequestObject) (RunJobResponseObject, error) {
	err := s.Scheduler.RunNow(req.Name)
	switch {
	case errors.Is(err, scheduler.ErrUnknownJob):
		return nil, httpx.NewError(http.StatusNotFound, "job_not_found", "không có job này")
	case errors.Is(err, scheduler.ErrBusy):
		return nil, httpx.Conflict("job_running", "job đang chạy")
	case err != nil:
		return nil, err
	}
	return RunJob202Response{}, nil
}
