import { useEffect, useState } from 'react'
import { getActiveWeek, getAdminGames, setGameIncluded, syncGames } from '../api/client'
import type { Game, Week } from '../types'

const DAY_ORDER = ['Thursday', 'Friday', 'Saturday', 'Sunday', 'Monday', 'Tuesday', 'Wednesday']

function groupByDay(games: Game[]): [string, Game[]][] {
  const dayMap = new Map<string, Game[]>()
  for (const g of games) {
    const day = new Date(g.kickoff_at).toLocaleDateString('en-US', {
      weekday: 'long',
      timeZone: 'America/New_York',
    })
    const list = dayMap.get(day) ?? []
    list.push(g)
    dayMap.set(day, list)
  }
  return [...dayMap.entries()].sort((a, b) => {
    const ai = DAY_ORDER.indexOf(a[0])
    const bi = DAY_ORDER.indexOf(b[0])
    return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi)
  })
}

export default function AdminGames() {
  const [week, setWeek] = useState<Week | null>(null)
  const [games, setGames] = useState<Game[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [toggling, setToggling] = useState<Set<string>>(new Set())
  const [syncing, setSyncing] = useState(false)
  const [syncMsg, setSyncMsg] = useState('')

  useEffect(() => {
    const load = async () => {
      try {
        const { data: w } = await getActiveWeek()
        setWeek(w)
        const { data: g } = await getAdminGames(w.week_number, w.season_year)
        setGames(g)
      } catch {
        setError('Failed to load games.')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [])

  const handleSync = async () => {
    if (!week) return
    setSyncing(true)
    setSyncMsg('')
    try {
      await syncGames(week.week_number, week.season_year)
      const { data: g } = await getAdminGames(week.week_number, week.season_year)
      setGames(g)
      setSyncMsg('Synced.')
      setTimeout(() => setSyncMsg(''), 3000)
    } catch {
      setSyncMsg('Sync failed.')
    } finally {
      setSyncing(false)
    }
  }

  const handleToggle = async (game: Game) => {
    setToggling((prev) => new Set(prev).add(game.id))
    try {
      const { data: updated } = await setGameIncluded(game.id, !game.included_in_picks)
      setGames((prev) => prev.map((g) => (g.id === updated.id ? updated : g)))
    } catch {
      // leave as-is on error
    } finally {
      setToggling((prev) => {
        const next = new Set(prev)
        next.delete(game.id)
        return next
      })
    }
  }

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="w-8 h-8 border-4 border-indigo-500 border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  if (error) {
    return <div className="text-center py-16 text-red-500">{error}</div>
  }

  const grouped = groupByDay(games)

  return (
    <div className="max-w-lg mx-auto px-4 py-6 space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-xl font-bold text-gray-900">Manage Games</h2>
          <p className="text-sm text-gray-500 mt-1">
            Week {week?.week_number} · {week?.season_year} season — toggle games in/out of picks
          </p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {syncMsg && <span className="text-xs text-gray-500">{syncMsg}</span>}
          <button
            onClick={handleSync}
            disabled={syncing}
            className="text-sm font-medium px-3 py-1.5 rounded-lg border border-gray-300 text-gray-700
                       hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {syncing ? 'Syncing…' : 'Sync ESPN'}
          </button>
        </div>
      </div>

      {grouped.map(([day, dayGames]) => (
        <div key={day}>
          <p className="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-2">{day}</p>
          <div className="space-y-2">
            {dayGames.map((game) => {
              const included = game.included_in_picks
              const busy = toggling.has(game.id)
              return (
                <div
                  key={game.id}
                  className={`flex items-center justify-between rounded-xl border px-4 py-3 transition-colors ${
                    included ? 'border-gray-200 bg-white' : 'border-gray-100 bg-gray-50'
                  }`}
                >
                  <span className={`text-sm font-medium ${included ? 'text-gray-900' : 'text-gray-400'}`}>
                    {game.away_team_name} vs. {game.home_team_name}
                  </span>
                  <button
                    onClick={() => handleToggle(game)}
                    disabled={busy}
                    className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none disabled:opacity-50 ${
                      included ? 'bg-indigo-600' : 'bg-gray-200'
                    }`}
                  >
                    <span
                      className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
                        included ? 'translate-x-6' : 'translate-x-1'
                      }`}
                    />
                  </button>
                </div>
              )
            })}
          </div>
        </div>
      ))}
    </div>
  )
}
