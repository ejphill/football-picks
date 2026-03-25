package notify

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/resend/resend-go/v2"
)

var boldRe = regexp.MustCompile(`\*\*(.*?)\*\*`)

// introPlain strips **bold** markers for plain-text email.
func introPlain(s string) string {
	return boldRe.ReplaceAllString(s, "$1")
}

// introHTML escapes HTML, converts **bold** → <strong>, and newlines → <br>.
func introHTML(s string) string {
	escaped := html.EscapeString(s)
	withBold := boldRe.ReplaceAllString(escaped, "<strong>$1</strong>")
	return strings.ReplaceAll(withBold, "\n", "<br>")
}

type Mailer interface {
	SendAnnouncement(ctx context.Context, user models.User, a *models.Announcement, weekNumber int) error
}

// NoopMailer drops all emails silently (used when RESEND_API_KEY is not set).
type NoopMailer struct{}

func (NoopMailer) SendAnnouncement(_ context.Context, _ models.User, _ *models.Announcement, _ int) error {
	return nil
}

type ResendMailer struct {
	client    *resend.Client
	fromEmail string
	fromName  string
	appURL    string
}

func NewResendMailer(apiKey, fromEmail, appURL string) *ResendMailer {
	return &ResendMailer{
		client:    resend.NewClient(apiKey),
		fromEmail: fromEmail,
		fromName:  "Football Picks",
		appURL:    appURL,
	}
}

func (m *ResendMailer) SendAnnouncement(ctx context.Context, user models.User, a *models.Announcement, weekNumber int) error {
	subject := fmt.Sprintf("Week %d Picks are open!", weekNumber)

	plainText := fmt.Sprintf("Hey %s,\n\n%s\n\nMake your picks: %s/picks\n\nTo unsubscribe, visit your profile settings.",
		user.DisplayName, introPlain(a.Intro), m.appURL)

	htmlBody := fmt.Sprintf(`<p>Hey %s,</p>
<p style="line-height:1.6;">%s</p>
<p><a href="%s/picks">Make your picks &rarr;</a></p>
<p style="color:#999;font-size:12px;">To unsubscribe, update your <a href="%s/profile">profile settings</a>.</p>`,
		user.DisplayName, introHTML(a.Intro), m.appURL, m.appURL)

	from := fmt.Sprintf("%s <%s>", m.fromName, m.fromEmail)
	params := &resend.SendEmailRequest{
		From:    from,
		To:      []string{user.Email},
		Subject: subject,
		Text:    plainText,
		Html:    htmlBody,
	}

	_, err := m.client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("resend send: %w", err)
	}
	slog.Info("notify: sent announcement email", "email", user.Email)
	return nil
}

const sendTimeout = 10 * time.Second

// 20 workers: drops a 10K-user send from ~83min serial to ~4min
const sendWorkers = 20

// SendAll sends concurrently and logs each result.
// weekNumber is human-readable (e.g. 7); weekID is the DB primary key.
func SendAll(ctx context.Context, pool *pgxpool.Pool, mailer Mailer, users []models.User, a *models.Announcement, weekNumber int, weekID int) {
	if len(users) == 0 {
		return
	}

	work := make(chan models.User, len(users))
	for _, u := range users {
		work <- u
	}
	close(work)

	workers := sendWorkers
	if len(users) < workers {
		workers = len(users)
	}

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range work {
				if ctx.Err() != nil {
					return
				}
				sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
				sendErr := mailer.SendAnnouncement(sendCtx, u, a, weekNumber)
				cancel()

				success := sendErr == nil
				errMsg := ""
				if sendErr != nil {
					errMsg = sendErr.Error()
					slog.Error("notify: send failed", "email", u.Email, "err", sendErr)
				}
				if logErr := queries.LogNotification(ctx, pool, u.ID, weekID, success, errMsg); logErr != nil {
					slog.Error("notify: log notification", "err", logErr)
				}
			}
		}()
	}
	wg.Wait()
}
