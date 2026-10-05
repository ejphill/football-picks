import type { Game } from '../types'
import { liveStatusBadge } from '../utils/gameStatus'

// Compact, read-only scoreboard for the Home page — same score/status data
// that powers the Picks tab's live scores, just without the pick-selection
// UI. No auto-refresh; reflects whatever was true when the page loaded.
export default function ScoreStrip({ games }: { games: Game[] }) {
  if (games.length === 0) return null

  return (
    <div className="rounded-xl border border-gray-200 bg-white divide-y divide-gray-100 overflow-hidden">
      {games.map((game) => {
        const hasScore = game.status !== 'scheduled' && (game.home_score != null || game.away_score != null)
        const badge = liveStatusBadge(game)
        const kickoffStr = new Date(game.kickoff_at).toLocaleTimeString('en-US', {
          hour: 'numeric',
          minute: '2-digit',
          timeZoneName: 'short',
          timeZone: 'America/New_York',
        })

        return (
          <div key={game.id} className="flex items-center justify-between px-4 py-2.5 text-sm">
            <span className="font-medium text-gray-800">
              {game.away_team} <span className="text-gray-400 font-normal">@</span> {game.home_team}
            </span>
            <span className="flex items-center gap-3">
              {hasScore && (
                <span className="font-semibold text-gray-700 tabular-nums">
                  {game.away_score ?? 0}–{game.home_score ?? 0}
                </span>
              )}
              {badge ?? <span className="text-xs text-gray-400">{kickoffStr}</span>}
            </span>
          </div>
        )
      })}
    </div>
  )
}
