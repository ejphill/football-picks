package notify

import (
	"context"
	"log/slog"
	"sync"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RetryFailed resends eligible failures using the same worker pool as SendAll.
func RetryFailed(ctx context.Context, pool *pgxpool.Pool, mailer Mailer) {
	failed, err := queries.GetFailedNotifications(ctx, pool)
	if err != nil {
		slog.Error("notify: get failed notifications", "err", err)
		return
	}
	if len(failed) == 0 {
		return
	}

	slog.Info("notify: retrying failed notifications", "count", len(failed))

	work := make(chan queries.FailedNotificationRow, len(failed))
	for _, r := range failed {
		work <- r
	}
	close(work)

	workers := sendWorkers
	if len(failed) < workers {
		workers = len(failed)
	}

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for row := range work {
				if ctx.Err() != nil {
					return
				}

				user := models.User{
					ID:          row.UserID,
					DisplayName: row.DisplayName,
					Email:       row.Email,
				}
				a := &models.Announcement{
					ID:     row.AnnouncementID,
					WeekID: row.WeekID,
					Intro:  row.AnnouncementIntro,
				}

				sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
				sendErr := mailer.SendAnnouncement(sendCtx, user, a, row.WeekNumber)
				cancel()

				success := sendErr == nil
				errMsg := ""
				if sendErr != nil {
					errMsg = sendErr.Error()
					slog.Error("notify: retry failed",
						"email", user.Email, "week", row.WeekNumber,
						"attempt", row.Attempts+1, "err", sendErr)
				} else {
					slog.Info("notify: retry succeeded",
						"email", user.Email, "week", row.WeekNumber,
						"attempt", row.Attempts+1)
				}

				if logErr := queries.LogNotification(ctx, pool, user.ID, row.WeekID, success, errMsg); logErr != nil {
					slog.Error("notify: log retry", "err", logErr)
				}
			}
		}()
	}
	wg.Wait()
}
