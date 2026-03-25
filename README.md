# Football Picks

My brother has run our family's NFL picks league for 18 years years, tracking picks over email, managing a Google Form every week, and manually updating a spreadsheet to keep score. I built this to replace all of that.

A full-stack NFL weekly picks app for a private league: pick game winners each week, compete on a leaderboard, and get email blasts when the week opens.

## Overview

- Users submit predictions for each week's included NFL games before kickoff; picks lock per-game at kickoff time, not as a single weekly deadline
- Leaderboard uses a **floor scoring** system: if a user missed an entire kickoff window, they receive credit equal to the minimum correct count any other picker achieved in that window — preventing one missed week from unfairly tanking a season standing
- Used by a private group; backend is production-ready with auth, email notifications, caching, and automated score updates

## Demo

**Season leaderboard** — cumulative W-L and win% across all 18 weeks, with floor scoring applied for any missed kickoff windows.

![Season leaderboard](assets/NFL_Picks_Season_Leaderboard.png)

**Weekly picks grid** — all players' picks revealed side-by-side after kickoff; correct picks in green, incorrect in red.

![Weekly picks grid](assets/NFL_Picks_Weekly_Leaderboard.png)

**Pick submission** — per-game cards with spread, team names, and lock status. Each game locks independently at its kickoff time.

![Pick submission](assets/NFL_Picks_Weekly_Submission.png)

## Architecture

![System Architecture](assets/NFL_Picks_Architecture.svg)

- **Auth + Database** — Supabase hosts the Postgres database and handles auth. The Go server verifies JWTs against Supabase's JWKS endpoint. Two identities per user: a Supabase UUID (auth) and an app UUID (data).
- **ESPN** — public API polled on a schedule to sync game statuses and scores; no API key required
- **Email** — Resend via a `Mailer` interface; swapped to a no-op in tests and when no key is configured

## Key Technical Decisions

- **Per-game pick locking** — picks lock at each game's individual kickoff time, not a single weekly deadline. The lock check happens inside the SQL upsert itself, not in application code, so there's no race window between checking and writing.

- **Floor scoring in pure SQL** — the floor scoring calculation is handled entirely in a single SQL query using window functions and CTEs rather than application-level loops. When a user misses a kickoff window, the query computes the minimum correct count across all pickers in that window and credits it inline.

- **Event-driven cache invalidation** — the leaderboard cache isn't just TTL-based. When scoring runs, it fires an `onScored` callback that wipes the cache immediately. The syncer doesn't know anything about caches, the hook is injected at startup. The 1-hour TTL only exists as a fallback if the callback is somehow missed.

- **Two-tier caching** — both the user lookup cache and leaderboard cache use Redis if available and fall back to an in-process TTL cache otherwise. The app runs fine without Redis configured.

- **Scoring is idempotent** — re-running the score job never produces wrong results. It also handles the edge case where ESPN corrects a winner after the fact, by re-scoring any pick whose game was updated more recently than the pick itself.

- **In-process scheduler** — the ESPN poller, email dispatcher, and retry loop run as goroutines alongside the HTTP server rather than as separate cron jobs. This keeps deployment simple (one binary, one process) and lets the scheduler share the DB pool and cache directly. The tradeoff is that horizontal scaling would require a distributed lock or external cron to avoid duplicate work.

- **Integration tests against a real database** — tests run against an actual Postgres instance (via Docker), not mocks. Constraint violations and migration issues surface in tests rather than in production.

- **Graceful shutdown** — on SIGTERM the server stops accepting new requests, drains in-flight HTTP requests, and waits for any in-progress email sends to finish before exiting.

## Features

- Weekly pick submission with per-game kickoff locking
- Floor scoring leaderboard (weekly + season standings)
- Picks hidden until lock time; revealed to all users after kickoff
- Automated week-open email announcements via Resend
- Notification dedup — users are never emailed twice for the same week
- Retry loop for failed notifications (hourly, max 3 attempts)
- Admin endpoints: sync games from ESPN, score a week, toggle game inclusion, draft/post announcements
- ETag middleware for bandwidth-efficient leaderboard polling

## Local Setup

**Prerequisites:** Docker, Go 1.22+, Node 18+

```bash
docker-compose up -d                        # start Postgres (dev + test)
cp .env.example .env                        # fill in SUPABASE_URL
go run ./cmd/server                         # backend on :8080

cd frontend
cp .env.example .env.local                  # fill in Supabase anon key
npm install && npm run dev                  # frontend on :5173

# tests
TEST_DATABASE_URL=postgres://picks:picks@localhost:5433/footballpicks_test \
  go test ./... -p 1
```

## Future Improvements

- Wire an external cron or Redis distributed lock to support horizontal scaling of the scheduler (needed if scale increases)
- Pre-compute leaderboard incrementally on score events rather than recomputing the full 6-CTE query (worthwhile at 10k+ users)
