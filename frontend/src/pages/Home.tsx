import { useEffect, useState } from 'react'
import { getActiveWeek, getAnnouncements } from '../api/client'
import FormattedText from '../components/FormattedText'
import type { Announcement, Week } from '../types'

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
    </div>
  )
}
