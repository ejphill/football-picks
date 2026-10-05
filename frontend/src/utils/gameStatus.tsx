import type { Game } from '../types'

export function periodLabel(period: number): string {
  return period <= 4 ? `Q${period}` : period === 5 ? 'OT' : `${period - 4}OT`
}

// Live/Final badge, shared between GameCard and the Home page scoreboard.
// Returns null for a game that hasn't started — callers decide what to show
// instead (e.g. "Locked" on the Picks tab, kickoff time on a scoreboard).
export function liveStatusBadge(game: Game) {
  if (game.status === 'in_progress') {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-semibold text-red-600">
        <span className="w-1.5 h-1.5 rounded-full bg-red-600 animate-pulse" />
        {game.period != null ? periodLabel(game.period) : 'Live'}
        {game.display_clock ? ` ${game.display_clock}` : ''}
      </span>
    )
  }
  if (game.status === 'final') {
    return <span className="text-xs font-semibold text-gray-500">Final</span>
  }
  return null
}
