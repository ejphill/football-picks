import { useEffect, useState } from 'react'
import { getActiveWeek, getAnnouncements } from '../api/client'
import FormattedText from '../components/FormattedText'
import type { Announcement, Week } from '../types'

function HowItWorks() {
  const [open, setOpen] = useState(false)
  return (
    <div className="rounded-xl border border-gray-200 bg-white overflow-hidden">
      <button
        onClick={() => setOpen((o) => !o)}
        className="w-full flex items-center justify-between px-5 py-3 text-sm font-semibold text-gray-700 hover:bg-gray-50 transition-colors"
      >
        How the league works
        <span className="text-gray-400">{open ? '−' : '+'}</span>
      </button>
      {open && (
        <div className="px-5 pb-5 space-y-4 text-sm text-gray-700 leading-relaxed border-t border-gray-100 pt-4">
          <div>
            <p className="font-semibold text-gray-900 mb-1">Games lock one at a time</p>
            <p>
              Each game locks the moment it kicks off — not the whole week at
              once. You can keep changing your pick for any game that hasn't
              started yet, even if other games that week have already
              started or finished.
            </p>
          </div>
          <div>
            <p className="font-semibold text-gray-900 mb-1">Missed a game? You won't get zeroed out</p>
            <p>
              If you miss picking a whole batch of games (say, all the
              Sunday 1pm games), you're not scored as 0 correct for every one
              of them. Instead, you're given credit equal to whatever the
              worst score was among everyone who <em>did</em> pick that
              batch — so you're never worse off than the person who actually
              played and had the worst day.
            </p>
          </div>
          <div>
            <p className="font-semibold text-gray-900 mb-1">Picks are hidden, then revealed</p>
            <p>
              Nobody can see your picks for a game until that game kicks off
              — you can always see your own, just not anyone else's ahead of
              time.
            </p>
          </div>
        </div>
      )}
    </div>
  )
}

export default function Home() {
  const [week, setWeek] = useState<Week | null>(null)
  const [announcement, setAnnouncement] = useState<Announcement | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    const load = async () => {
      try {
        const { data: w } = await getActiveWeek()
        setWeek(w)
        const { data: announcements } = await getAnnouncements(w.season_year)
        const weekAnnouncements = announcements.filter((a) => a.week_id === w.id)
        if (weekAnnouncements.length > 0) {
          setAnnouncement(weekAnnouncements[0])
        }
      } catch {
        setError('Could not load announcement.')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [])

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

  return (
    <div className="max-w-lg mx-auto px-4 py-6 space-y-4">
      <div className="flex items-baseline justify-between">
        <h2 className="text-xl font-bold text-gray-900">Week {week?.week_number}</h2>
        <span className="text-sm text-gray-400">{week?.season_year} season</span>
      </div>

      {announcement ? (
        <div className="rounded-xl border border-gray-200 bg-white px-5 py-4 text-sm text-gray-800 leading-relaxed">
          <FormattedText text={announcement.intro} />
        </div>
      ) : (
        <div className="text-center py-16 text-gray-400">
          No announcement posted yet for this week.
        </div>
      )}

      <HowItWorks />
    </div>
  )
}
