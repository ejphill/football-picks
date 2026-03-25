import { useEffect, useState } from 'react'
import { getActiveWeek, getGames } from '../api/client'
import { usePicksStore } from '../stores/picksStore'
import GameCard from '../components/GameCard'
import type { Game, Week } from '../types'

export default function Picks() {
  const { picksByGameId, loadPicks, submitPick, submitError } = usePicksStore()
  const [week, setWeek] = useState<Week | null>(null)
  const [games, setGames] = useState<Game[]>([])
  const [gamesLoading, setGamesLoading] = useState(true)
  const [error, setError] = useState('')
  const [draft, setDraft] = useState<Record<string, 'home' | 'away'>>({})
  const [submitting, setSubmitting] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [editing, setEditing] = useState(false)

  useEffect(() => {
    const load = async () => {
      try {
        const weekRes = await getActiveWeek()
        const activeWeek = weekRes.data
        setWeek(activeWeek)

        await Promise.all([
          getGames(activeWeek.week_number, activeWeek.season_year).then((r) => setGames(r.data)),
          loadPicks(activeWeek.week_number, activeWeek.season_year),
        ])
      } catch {
        setError('Could not load games. Try refreshing.')
      } finally {
        setGamesLoading(false)
      }
    }
    load()
  }, [])

  // Once picks are loaded, initialize draft from saved picks
  useEffect(() => {
    const initial: Record<string, 'home' | 'away'> = {}
    for (const [gameId, pick] of Object.entries(picksByGameId)) {
      initial[gameId] = pick.picked_team as 'home' | 'away'
    }
    setDraft(initial)
    // If they already have picks saved, start in submitted view
    if (Object.keys(picksByGameId).length > 0) {
      setSubmitted(true)
    }
  }, [picksByGameId])

  const handleSelect = (gameId: string, team: 'home' | 'away') => {
    setDraft((prev) => ({ ...prev, [gameId]: team }))
  }

  const handleSubmit = async () => {
    setSubmitting(true)
    try {
      await Promise.all(
        Object.entries(draft).map(([gameId, team]) => submitPick(gameId, team))
      )
      setSubmitted(true)
      setEditing(false)
    } finally {
      setSubmitting(false)
    }
  }

  if (gamesLoading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="w-8 h-8 border-4 border-indigo-500 border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  if (error) {
    return <div className="text-center py-16 text-red-500">{error}</div>
  }

  if (games.length === 0) {
    return <div className="text-center py-16 text-gray-400">No games scheduled this week.</div>
  }

  // Submitted confirmation screen
  if (submitted && !editing) {
    const pickedCount = Object.keys(draft).length
    return (
      <div className="max-w-lg mx-auto px-4 py-6 space-y-6">
        <div className="flex flex-col items-center text-center space-y-6 py-10">
          <div className="w-20 h-20 rounded-full bg-green-100 flex items-center justify-center">
            <svg className="w-10 h-10 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
            </svg>
          </div>
          <div className="space-y-2">
            <h2 className="text-2xl font-bold text-gray-900">Picks Submitted!</h2>
            <p className="text-gray-500">
              Week {week?.week_number} — {pickedCount} of {games.length} games picked
            </p>
          </div>
          <button
            onClick={() => setEditing(true)}
            className="px-6 py-2.5 rounded-xl border border-gray-300 text-gray-700 font-medium text-sm
                       hover:bg-gray-50 transition-colors"
          >
            Edit Picks
          </button>
        </div>
      </div>
    )
  }

  // Editing / draft screen
  const weekLocked = week ? Date.now() > new Date(week.picks_lock_at).getTime() : false
  const pickedCount = Object.keys(draft).length

  return (
    <div className="max-w-lg mx-auto px-4 py-6 space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-bold text-gray-900">Week {week?.week_number} Picks</h2>
        <span className="text-sm text-gray-500">{pickedCount}/{games.length} picked</span>
      </div>
      {submitError && (
        <div className="rounded-lg bg-red-50 border border-red-200 px-3 py-2 text-sm text-red-700">
          {submitError}
        </div>
      )}

      <div className="space-y-3">
        {games.map((game) => (
          <GameCard
            key={game.id}
            game={game}
            selected={draft[game.id] ?? null}
            locked={weekLocked}
            onSelect={handleSelect}
          />
        ))}
      </div>

      <div className="pt-2">
        <button
          onClick={handleSubmit}
          disabled={submitting || pickedCount === 0}
          className="w-full py-3 rounded-xl bg-indigo-600 text-white font-semibold text-sm
                     hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {submitting ? 'Submitting…' : 'Submit Picks'}
        </button>
      </div>
    </div>
  )
}
