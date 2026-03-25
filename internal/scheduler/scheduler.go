package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/draft"
	"github.com/evan/football-picks/internal/espn"
	"github.com/evan/football-picks/internal/notify"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// systemUserID is the fixed UUID of the auto-sender row in the users table.
var systemUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

type Scheduler struct {
	pool   *pgxpool.Pool
	syncer *espn.Syncer
	mailer notify.Mailer
}

func New(pool *pgxpool.Pool, mailer notify.Mailer, syncer *espn.Syncer) *Scheduler {
	return &Scheduler{pool: pool, syncer: syncer, mailer: mailer}
}

// Start launches the poller and retry loop, then runs the announce loop. Call as a goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	go s.pollLoop(ctx)
	go s.retryLoop(ctx)
	s.announceLoop(ctx)
}

// retryLoop waits 30min for initial sends to settle, then retries hourly.
func (s *Scheduler) retryLoop(ctx context.Context) {
	s.sleep(ctx, 30*time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		notify.RetryFailed(ctx, s.pool, s.mailer)
		s.sleep(ctx, time.Hour)
	}
}

// pollLoop polls every 60s during active games, sleeps until next kickoff otherwise.
// Consecutive failures back off exponentially (30s×n, cap 5m) with 10s jitter.
func (s *Scheduler) pollLoop(ctx context.Context) {
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		s.sleep(ctx, s.syncIfNeeded(ctx, &failures))
	}
}

// syncIfNeeded returns how long to sleep. failures tracks consecutive errors across calls.
func (s *Scheduler) syncIfNeeded(ctx context.Context, failures *int) time.Duration {
	week, err := queries.GetActiveWeek(ctx, s.pool)
	if err != nil {
		return time.Hour
	}

	games, err := queries.GetGamesByWeek(ctx, s.pool, week.ID)
	if err != nil {
		return time.Hour
	}

	now := time.Now()

	// Any included game that has kicked off but isn't final yet?
	for _, g := range games {
		if now.After(g.KickoffAt) && g.Status != "final" {
			syncCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			seasonType := 2
			if week.WeekNumber > 18 {
				seasonType = 3 // playoffs
			}
			err := s.syncer.SyncWeek(syncCtx, week, seasonType)
			cancel()
			if err != nil {
				*failures++
				backoff := time.Duration(*failures) * 30 * time.Second
				if backoff > 5*time.Minute {
					backoff = 5 * time.Minute
				}
				jitter := time.Duration(rand.Int63n(int64(10 * time.Second)))
				slog.Warn("poller: sync week failed",
					"week", week.WeekNumber, "consecutive_failures", *failures,
					"err", err, "retry_in", (backoff+jitter).Round(time.Second).String())
				return backoff + jitter
			}
			*failures = 0
			return 60 * time.Second
		}
	}

	// No active games — wake up right before the next kickoff if it's within the hour.
	for _, g := range games {
		if g.Status == "scheduled" && g.KickoffAt.After(now) && g.KickoffAt.Before(now.Add(time.Hour)) {
			until := time.Until(g.KickoffAt)
			slog.Info("poller: next kickoff", "week", week.WeekNumber, "in", until.Round(time.Minute).String())
			return until
		}
	}

	return time.Hour
}

// announceLoop fires a default weekly announcement on Saturday at 1 PM EST
// if no announcement has been posted for the active week.
func (s *Scheduler) announceLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		week, err := queries.GetActiveWeek(ctx, s.pool)
		if err != nil {
			slog.Error("scheduler: get active week", "err", err)
			s.sleep(ctx, time.Hour)
			continue
		}

		target, err := s.saturdayTarget(ctx, week.ID)
		if err != nil {
			slog.Error("scheduler: compute target", "week", week.WeekNumber, "err", err)
			s.sleep(ctx, time.Hour)
			continue
		}

		if time.Now().After(target) {
			slog.Info("scheduler: past target, skipping", "week", week.WeekNumber, "target", target)
			s.sleep(ctx, 24*time.Hour)
			continue
		}

		slog.Info("scheduler: waiting for announce target", "week", week.WeekNumber, "target", target)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(target)):
		}

		s.maybeAutoSend(ctx, week.ID, week.WeekNumber)

		// Sleep 24h before looping so we pick up the next week's active week.
		s.sleep(ctx, 24*time.Hour)
	}
}

func (s *Scheduler) saturdayTarget(ctx context.Context, weekID int) (time.Time, error) {
	games, err := queries.GetGamesByWeek(ctx, s.pool, weekID)
	if err != nil {
		return time.Time{}, err
	}
	if len(games) == 0 {
		return time.Time{}, fmt.Errorf("no games for week %d", weekID)
	}

	est, _ := time.LoadLocation("America/New_York")
	first := games[0].KickoffAt.In(est)
	daysUntilSat := (int(time.Saturday) - int(first.Weekday()) + 7) % 7
	sat := first.AddDate(0, 0, daysUntilSat)
	return time.Date(sat.Year(), sat.Month(), sat.Day(), 13, 0, 0, 0, est), nil
}

func (s *Scheduler) maybeAutoSend(ctx context.Context, weekID, weekNumber int) {
	existing, err := queries.GetAnnouncementsByWeek(ctx, s.pool, weekID)
	if err != nil {
		slog.Error("scheduler: check announcements", "err", err)
		return
	}
	if len(existing) > 0 {
		slog.Info("scheduler: announcement already exists, skipping", "week", weekNumber)
		return
	}

	week, err := queries.GetWeekByID(ctx, s.pool, weekID)
	if err != nil {
		slog.Error("scheduler: get week", "err", err)
		return
	}

	d, err := draft.BuildDraft(ctx, s.pool, week)
	if err != nil {
		slog.Error("scheduler: build draft", "err", err)
		return
	}

	body := draft.Assemble(d)
	a, err := queries.CreateAutoAnnouncement(ctx, s.pool, systemUserID, weekID, body)
	if err != nil {
		slog.Error("scheduler: create announcement", "err", err)
		return
	}
	if a == nil {
		// Another instance inserted first; they will handle SendAll.
		slog.Info("scheduler: auto-announcement already created by another instance", "week", weekNumber)
		return
	}
	slog.Info("scheduler: auto-announcement created", "week", weekNumber)

	users, err := queries.GetUsersForNotification(ctx, s.pool, weekID)
	if err != nil {
		slog.Error("scheduler: get users", "err", err)
		return
	}
	notify.SendAll(ctx, s.pool, s.mailer, users, a, weekNumber, weekID)
}

func (s *Scheduler) sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
