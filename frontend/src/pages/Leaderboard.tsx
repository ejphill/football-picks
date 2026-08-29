import { useEffect, useState } from 'react'
import { useLeaderboardStore } from '../stores/leaderboardStore'
import { useAuthStore } from '../stores/authStore'
import { getActiveWeek, getSeasons, getWeeks } from '../api/client'
import type { WeeklyLeaderboardEntry, WeeklyLeaderboardGame, SeasonLeaderboardEntry, Season, Week } from '../types'

type Tab = 'weekly' | 'season'

const selectCls =
  'text-sm font-medium border border-gray-300 rounded-lg px-2.5 py-1.5 text-gray-700 bg-white ' +
  'focus:outline-none focus:ring-2 focus:ring-indigo-500'

export default function Leaderboard() {
  const [tab, setTab] = useState<Tab>('weekly')
  const [seasons, setSeasons] = useState<Season[]>([])
  const [weeks, setWeeks] = useState<Week[]>([])
  const [selectedSeason, setSelectedSeason] = useState<number | null>(null)
  const [selectedWeek, setSelectedWeek] = useState<number | null>(null)
  const { weeklyData, seasonData, weeklyLoading, seasonLoading, loadWeekly, loadSeason } =
    useLeaderboardStore()
  const currentUser = useAuthStore((s) => s.user)

  // Seed defaults from the active week, and load the season list for the picker.
  useEffect(() => {
    getActiveWeek()
      .then((res) => {
        setSelectedSeason(res.data.season_year)
        setSelectedWeek(res.data.week_number)
      })
      .catch(() => {})
    getSeasons()
      .then((res) => setSeasons(res.data))
      .catch(() => {})
  }, [])

  // Load this season's weeks whenever the season selection changes.
  useEffect(() => {
    if (selectedSeason == null) return
    getWeeks(selectedSeason)
      .then((res) => {
        setWeeks(res.data)
        // If the currently selected week doesn't belong to this season, default
        // to the most recent week in it.
        if (!res.data.some((w) => w.week_number === selectedWeek) && res.data.length > 0) {
          setSelectedWeek(res.data[res.data.length - 1].week_number)
        }
      })
      .catch(() => {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedSeason])

  useEffect(() => {
    if (selectedSeason == null) return
    if (tab === 'weekly') {
      if (selectedWeek == null) return
      loadWeekly(selectedWeek, selectedSeason)
    } else {
      loadSeason(selectedSeason)
    }
  }, [tab, selectedSeason, selectedWeek])

  return (
    <div className="max-w-5xl mx-auto px-4 py-6 space-y-4">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h2 className="text-xl font-bold text-gray-900">
          Leaderboard {tab === 'weekly' && selectedWeek ? `— Week ${selectedWeek}` : ''}
        </h2>

        <div className="flex items-center gap-2">
          <select
            className={selectCls}
            value={selectedSeason ?? ''}
            onChange={(e) => setSelectedSeason(Number(e.target.value))}
          >
            {seasons.map((s) => (
              <option key={s.year} value={s.year}>
                {s.year}
              </option>
            ))}
          </select>

          {tab === 'weekly' && (
            <select
              className={selectCls}
              value={selectedWeek ?? ''}
              onChange={(e) => setSelectedWeek(Number(e.target.value))}
            >
              {weeks.map((w) => (
                <option key={w.week_number} value={w.week_number}>
                  Week {w.week_number}
                </option>
              ))}
            </select>
          )}
        </div>
      </div>

      <div className="flex bg-gray-100 rounded-lg p-1 gap-1">
        {(['weekly', 'season'] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`flex-1 py-2 text-sm font-medium rounded-md transition-colors capitalize ${
              tab === t ? 'bg-white text-indigo-700 shadow-sm' : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {tab === 'weekly' ? (
        <WeeklyView
          games={weeklyData?.games ?? []}
          entries={weeklyData?.entries ?? []}
          locked={weeklyData?.locked ?? false}
          loading={weeklyLoading}
          currentUserId={currentUser?.id}
        />
      ) : (
        <SeasonView data={seasonData} loading={seasonLoading} currentUserId={currentUser?.id} />
      )}
    </div>
  )
}

function WeeklyView({
  games,
  entries,
  locked,
  loading,
  currentUserId,
}: {
  games: WeeklyLeaderboardGame[]
  entries: WeeklyLeaderboardEntry[]
  locked: boolean
  loading: boolean
  currentUserId?: string
}) {
  if (loading) return <Spinner />
  if (entries.length === 0)
    return <div className="text-center py-16 text-gray-400">No picks yet this week.</div>

  // Sort players: most correct first, then alphabetical
  const sorted = [...entries].sort(
    (a, b) => b.correct - a.correct || a.display_name.localeCompare(b.display_name)
  )

  // Build a lookup: userId -> gameId -> PickView
  const pickMap: Record<string, Record<string, { picked_team: string; is_correct: boolean | null }>> = {}
  for (const entry of sorted) {
    pickMap[entry.user_id] = {}
    for (const p of entry.picks) {
      pickMap[entry.user_id][p.game_id] = { picked_team: p.picked_team, is_correct: p.is_correct }
    }
  }

  return (
    <div className="space-y-3">
      {!locked && (
        <p className="text-xs text-amber-600 bg-amber-50 border border-amber-200 rounded-lg px-3 py-2">
          Picks are hidden until lock time. Only your own picks are shown.
        </p>
      )}

      <div className="overflow-x-auto rounded-xl border border-gray-200">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="bg-gray-50 border-b border-gray-200">
              <th className="text-left px-3 py-2 font-semibold text-gray-600 whitespace-nowrap sticky left-0 bg-gray-50 z-10">
                Game
              </th>
              {sorted.map((e) => (
                <th
                  key={e.user_id}
                  className={`px-3 py-2 font-semibold text-center whitespace-nowrap ${
                    e.user_id === currentUserId ? 'text-indigo-700' : 'text-gray-600'
                  }`}
                >
                  {e.display_name}
                  {e.user_id === currentUserId && (
                    <span className="block text-xs font-normal text-indigo-400">(you)</span>
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {games.map((game) => (
              <tr key={game.id} className="hover:bg-gray-50">
                <td className="px-3 py-2 whitespace-nowrap text-gray-700 sticky left-0 bg-white z-10 font-medium">
                  {game.away_team} @ {game.home_team}
                </td>
                {sorted.map((entry) => {
                  const pick = pickMap[entry.user_id]?.[game.id]
                  const isMe = entry.user_id === currentUserId

                  if (!pick || (!locked && !isMe)) {
                    return (
                      <td key={entry.user_id} className="px-3 py-2 text-center text-gray-300">
                        —
                      </td>
                    )
                  }

                  const abbr =
                    pick.picked_team === 'home'
                      ? game.home_team
                      : pick.picked_team === 'away'
                        ? game.away_team
                        : pick.picked_team

                  const cellColor =
                    pick.is_correct === true
                      ? 'bg-green-100 text-green-800'
                      : pick.is_correct === false
                        ? 'bg-red-100 text-red-700'
                        : 'text-gray-700'

                  return (
                    <td key={entry.user_id} className={`px-3 py-2 text-center font-medium ${cellColor}`}>
                      {abbr || '—'}
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr className="bg-gray-50 border-t-2 border-gray-200 font-semibold">
              <td className="px-3 py-2 text-gray-600 sticky left-0 bg-gray-50 z-10">Score</td>
              {sorted.map((e) => (
                <td key={e.user_id} className="px-3 py-2 text-center text-gray-800">
                  {e.correct}–{e.total - e.correct}
                </td>
              ))}
            </tr>
          </tfoot>
        </table>
      </div>
    </div>
  )
}

function SeasonView({
  data,
  loading,
  currentUserId,
}: {
  data: SeasonLeaderboardEntry[]
  loading: boolean
  currentUserId?: string
}) {
  if (loading) return <Spinner />
  if (data.length === 0)
    return <div className="text-center py-16 text-gray-400">No season data yet.</div>

  return (
    <div className="rounded-xl border border-gray-200 overflow-hidden">
      <table className="w-full text-sm">
        <thead className="bg-gray-50 text-gray-500 uppercase text-xs">
          <tr>
            <th className="text-left px-4 py-3">Rank</th>
            <th className="text-left px-4 py-3">Player</th>
            <th className="text-right px-4 py-3">W-L</th>
            <th className="text-right px-4 py-3">Win%</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100">
          {data.map((entry) => {
            const isMe = entry.user_id === currentUserId
            const losses = entry.total - entry.correct
            return (
              <tr key={entry.user_id} className={isMe ? 'bg-indigo-50' : 'bg-white'}>
                <td className="px-4 py-3 font-bold text-gray-500">
                  {entry.rank === 1 ? '🥇' : entry.rank === 2 ? '🥈' : entry.rank === 3 ? '🥉' : entry.rank}
                </td>
                <td className="px-4 py-3 font-medium text-gray-900">
                  {entry.display_name}
                  {isMe && <span className="ml-2 text-xs text-indigo-500">(you)</span>}
                </td>
                <td className="px-4 py-3 text-right text-gray-600">
                  {entry.correct}–{losses}
                </td>
                <td className="px-4 py-3 text-right font-semibold text-gray-800">
                  {(entry.win_pct * 100).toFixed(1)}%
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

function Spinner() {
  return (
    <div className="flex justify-center items-center h-48">
      <div className="w-7 h-7 border-4 border-indigo-500 border-t-transparent rounded-full animate-spin" />
    </div>
  )
}
