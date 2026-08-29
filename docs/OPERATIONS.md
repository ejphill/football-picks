# Running the league — what's automatic vs. what Jack does

This is the week-to-week operator's guide. Everything here is driven by the
in-process scheduler (`internal/scheduler/scheduler.go`), which runs
alongside the API server as three goroutines: a poller, a retry loop, and an
announce loop.

## Fully automatic — no admin action needed

- **Pulling each week's schedule from ESPN.** The poller (`pollLoop`) checks
  once an hour whether the active week has any games in the database yet; if
  not, it syncs the week from ESPN itself. It keeps re-checking hourly even
  after that, which also picks up spread changes as odds firm up during the
  week (every sync overwrites `spread` on conflict).
- **Score updates during games.** Once a week's games exist, the poller
  checks every 60s whether any included game has kicked off but isn't marked
  `final` yet, and re-syncs that week from ESPN if so. Outside game windows
  it sleeps until the next kickoff instead of polling constantly.
- **Scoring.** Any sync that finds a game go `final` immediately triggers
  `ScorePicks` (`internal/espn/sync.go`) — there's no separate manual scoring
  step in normal operation. `POST /admin/score-week` exists only as a manual
  fallback if something needed to be re-scored by hand.
- **Leaderboard cache invalidation.** Wired to fire the moment scoring
  completes, so the leaderboard reflects new results immediately rather than
  waiting on the cache's 1-hour TTL fallback.
- **Failed email retries.** `retryLoop` retries any failed notification
  hourly (starting 30 min after the app boots), up to 3 attempts total.
- **Default weekly announcement.** If nobody has posted a custom
  announcement for the active week by **Saturday 1 PM ET**, `announceLoop`
  auto-generates one (via `internal/draft`, which assembles last week's
  results + this week's matchups) and emails it to everyone opted in to
  `notify_email`.

## Manual — this is Jack's job each week

1. **Review which games are excluded from picks.** New games default to
   excluded from picks if they kick off Tuesday–Friday (Thursday Night
   Football, the occasional Friday/international game); Saturday/Sunday/
   Monday default to included. Toggle any game on `/admin/games` if a
   default needs overriding for a given week — that override sticks through
   future syncs (`internal/db/queries/games.go`'s `UpsertGame` deliberately
   leaves `included_in_picks` alone on updates). The manual **Sync** button
   on that page and `POST /admin/sync-games` still exist if you ever want to
   force a refresh immediately rather than waiting for the hourly auto-sync.

2. **Optionally write a custom weekly announcement.** `/admin/compose`
   pre-fills a draft (results, records, this week's games) that can be
   edited before sending. If nothing is sent by Saturday 1 PM ET, the system
   sends its own default version automatically — so this step is optional,
   not required.

## Season setup (once a year, not weekly)

Not scheduler-driven at all — done by hand via a migration, following the
pattern in `migrations/009_seed_2026_season.sql`:

1. Deactivate the outgoing season and insert the new one (`seasons` table
   has a unique index enforcing only one `is_active = true` row at a time).
2. Insert all 18 `weeks` rows with estimated kickoff dates for sorting
   "current week" — these don't need to be exact; the real per-game lock
   time comes from ESPN sync (`games.kickoff_at`), not this table.
3. New admins need `is_admin` flipped to `true` manually in the database
   after they register — there's no self-serve admin signup:
   ```sql
   update users set is_admin = true where email = 'their-email@example.com';
   ```

## Where things live, for reference

| Concern | File |
|---|---|
| Poller / retry / announce loops | `internal/scheduler/scheduler.go` |
| ESPN sync + auto-scoring | `internal/espn/sync.go` |
| Default game inclusion rule | `internal/db/queries/games.go` (`UpsertGame`) |
| Admin endpoints | `internal/api/handlers/admin.go` |
| Announcement drafting | `internal/draft/draft.go` |
| Season/week seeding | `migrations/00X_seed_YYYY_season.sql` |
