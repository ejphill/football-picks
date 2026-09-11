import { useEffect, useMemo, useState } from 'react'
import FormattedText from '../components/FormattedText'
import {
  getActiveWeek,
  getAnnounceStatus,
  getDraftAnnouncement,
  postAnnouncement,
  setSkipAnnounce,
} from '../api/client'
import type { AnnounceStatus, Week } from '../types'

function formatAutoSendAt(iso: string): string {
  return new Date(iso).toLocaleString('en-US', {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    timeZoneName: 'short',
    timeZone: 'America/New_York',
  })
}

const ta = `w-full rounded-xl border border-gray-300 px-3 py-2 text-sm text-gray-900
            font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500
            focus:border-transparent resize-none`

function SectionBlock({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children: React.ReactNode
}) {
  return (
    <div>
      <div className="flex items-baseline gap-2 mb-1">
        <span className="text-sm font-semibold text-gray-700">{label}</span>
        {hint && <span className="text-xs text-gray-400">{hint}</span>}
      </div>
      {children}
    </div>
  )
}

export default function AdminCompose() {
  const [week, setWeek] = useState<Week | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  // Editable sections
  const [intro, setIntro] = useState('')
  const [resultsBlock, setResultsBlock] = useState('')
  const [recordsBlock, setRecordsBlock] = useState('')
  const [preGamesNote, setPreGamesNote] = useState('')
  const [gamesBlock, setGamesBlock] = useState('')
  const [outro, setOutro] = useState('')

  const [submitting, setSubmitting] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [submitError, setSubmitError] = useState('')

  const [announceStatus, setAnnounceStatus] = useState<AnnounceStatus | null>(null)
  const [togglingSkip, setTogglingSkip] = useState(false)

  useEffect(() => {
    const load = async () => {
      try {
        const { data: w } = await getActiveWeek()
        setWeek(w)

        const [{ data: draft }, { data: status }] = await Promise.all([
          getDraftAnnouncement(w.week_number, w.season_year),
          getAnnounceStatus(w.week_number, w.season_year),
        ])
        setIntro(draft.intro)
        setResultsBlock(draft.results)
        setRecordsBlock(draft.records)
        setPreGamesNote(draft.pre_games_note)
        setGamesBlock(draft.games)
        setOutro(draft.outro)
        setAnnounceStatus(status)
      } catch {
        setLoadError('Failed to load data. Try refreshing.')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [])

  const handleToggleSkip = async () => {
    if (!week || !announceStatus) return
    setTogglingSkip(true)
    try {
      await setSkipAnnounce(week.id, !announceStatus.skip_auto_announce)
      setAnnounceStatus({ ...announceStatus, skip_auto_announce: !announceStatus.skip_auto_announce })
    } finally {
      setTogglingSkip(false)
    }
  }

  // Live-assembled preview — updates as any section changes.
  const assembled = useMemo(() => {
    const parts = [intro, resultsBlock, recordsBlock, preGamesNote, gamesBlock, outro]
      .map((s) => s.trim())
      .filter(Boolean)
    return parts.join('\n\n')
  }, [intro, resultsBlock, recordsBlock, preGamesNote, gamesBlock, outro])

  const handlePost = async () => {
    if (!week || !assembled.trim()) return
    setSubmitting(true)
    setSubmitError('')
    try {
      await postAnnouncement(week.id, assembled.trim())
      setSubmitted(true)
    } catch {
      setSubmitError('Failed to post. Try again.')
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="w-8 h-8 border-4 border-indigo-500 border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  if (loadError) {
    return <div className="text-center py-16 text-red-500">{loadError}</div>
  }

  if (submitted) {
    return (
      <div className="max-w-lg mx-auto px-4 py-8 text-center space-y-4">
        <div className="w-16 h-16 rounded-full bg-green-100 flex items-center justify-center mx-auto">
          <svg className="w-8 h-8 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
          </svg>
        </div>
        <h2 className="text-xl font-bold text-gray-900">Announcement Posted!</h2>
        <p className="text-sm text-gray-500">
          Week {week?.week_number} — emails sent to all subscribers.
        </p>
        <button
          onClick={() => { setSubmitted(false); setPreGamesNote('') }}
          className="text-sm text-indigo-600 underline"
        >
          Post another
        </button>
      </div>
    )
  }

  return (
    <div className="max-w-6xl mx-auto px-4 py-6">
      <div className="mb-6">
        <h2 className="text-xl font-bold text-gray-900">Compose Announcement</h2>
        <p className="text-sm text-gray-500 mt-1">
          Week {week?.week_number} · {week?.season_year} season
        </p>
      </div>

      {announceStatus && (
        <div className="mb-6 flex items-center justify-between gap-4 rounded-xl border border-gray-200 bg-gray-50 px-4 py-3">
          <p className="text-sm text-gray-700">
            {announceStatus.has_announcement ? (
              'An announcement has already been posted for this week — the automatic email won’t send.'
            ) : announceStatus.skip_auto_announce ? (
              'Automatic email is turned off for this week.'
            ) : announceStatus.auto_send_at ? (
              <>Auto-send scheduled for <span className="font-semibold">{formatAutoSendAt(announceStatus.auto_send_at)}</span> if nothing's posted before then.</>
            ) : (
              'No games synced yet — auto-send time unknown.'
            )}
          </p>
          {!announceStatus.has_announcement && (
            <button
              onClick={handleToggleSkip}
              disabled={togglingSkip}
              className={`shrink-0 text-sm font-medium px-3 py-1.5 rounded-lg border transition-colors disabled:opacity-50 ${
                announceStatus.skip_auto_announce
                  ? 'border-indigo-300 text-indigo-700 hover:bg-indigo-50'
                  : 'border-gray-300 text-gray-700 hover:bg-gray-100'
              }`}
            >
              {togglingSkip ? 'Updating…' : announceStatus.skip_auto_announce ? 'Turn back on' : 'Cancel auto-send'}
            </button>
          )}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* Left: section editors */}
        <div className="space-y-5">
          <SectionBlock label="1. Intro" hint="Custom opening">
            <textarea
              value={intro}
              onChange={(e) => setIntro(e.target.value)}
              rows={3}
              className={ta}
            />
          </SectionBlock>

          <SectionBlock label="2. Last Week's Results" hint="Auto-generated · edit freely">
            <textarea
              value={resultsBlock}
              onChange={(e) => setResultsBlock(e.target.value)}
              rows={8}
              placeholder={week && week.week_number <= 1 ? 'No results yet (week 1)' : ''}
              className={ta}
            />
          </SectionBlock>

          <SectionBlock label="3. Season Records" hint="Auto-generated · edit freely">
            <textarea
              value={recordsBlock}
              onChange={(e) => setRecordsBlock(e.target.value)}
              rows={10}
              className={ta}
            />
          </SectionBlock>

          <SectionBlock label="4. Special Note" hint="Optional — e.g. Christmas games on Thursday">
            <textarea
              value={preGamesNote}
              onChange={(e) => setPreGamesNote(e.target.value)}
              rows={3}
              placeholder="Leave blank to omit…"
              className={ta}
            />
          </SectionBlock>

          <SectionBlock label="5. This Week's Games" hint="Auto-generated · edit freely">
            <textarea
              value={gamesBlock}
              onChange={(e) => setGamesBlock(e.target.value)}
              rows={12}
              className={ta}
            />
          </SectionBlock>

          <SectionBlock label="6. Sign-off">
            <textarea
              value={outro}
              onChange={(e) => setOutro(e.target.value)}
              rows={3}
              className={ta}
            />
          </SectionBlock>
        </div>

        {/* Right: live preview + post */}
        <div className="lg:sticky lg:top-20 lg:self-start space-y-4">
          <p className="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            Live Preview
          </p>
          <div className="text-sm text-gray-800 whitespace-pre-wrap bg-gray-50 border border-gray-200 rounded-xl p-4 min-h-[300px] leading-relaxed overflow-auto max-h-[70vh]">
            {assembled
              ? <FormattedText text={assembled} />
              : <span className="text-gray-400">Edit sections on the left…</span>
            }
          </div>

          {submitError && <p className="text-sm text-red-500">{submitError}</p>}

          <button
            onClick={handlePost}
            disabled={submitting || !assembled.trim()}
            className="w-full py-3 rounded-xl bg-indigo-600 text-white font-semibold text-sm
                       hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {submitting ? 'Posting…' : 'Post Announcement'}
          </button>
          <p className="text-xs text-gray-400 text-center">
            Emails all subscribers. The announcement also appears on the Home page.
          </p>
        </div>
      </div>
    </div>
  )
}
