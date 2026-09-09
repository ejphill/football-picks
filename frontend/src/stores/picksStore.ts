import { create } from 'zustand'
import type { Pick } from '../types'
import { getPicks, submitPicks as apiSubmitPicks } from '../api/client'

interface PicksState {
  picksByGameId: Record<string, Pick>
  loading: boolean
  error: string | null
  submitError: string | null
  loadPicks: (week: number, season: number) => Promise<void>
  // Submits all picks in one request; returns true only if every pick saved.
  submitPicks: (picks: { gameId: string; team: 'home' | 'away' }[]) => Promise<boolean>
}

export const usePicksStore = create<PicksState>((set) => ({
  picksByGameId: {},
  loading: false,
  error: null,
  submitError: null,

  loadPicks: async (week, season) => {
    set({ loading: true, error: null })
    try {
      const { data } = await getPicks(week, season)
      const map: Record<string, Pick> = {}
      for (const p of data) {
        map[p.game_id] = p
      }
      set({ picksByGameId: map, loading: false })
    } catch (e: unknown) {
      set({ error: 'Failed to load picks', loading: false })
    }
  },

  submitPicks: async (picks) => {
    set({ submitError: null })
    try {
      const { data: results } = await apiSubmitPicks(
        picks.map((p) => ({ game_id: p.gameId, picked_team: p.team }))
      )

      const saved: Record<string, Pick> = {}
      const failures: string[] = []
      for (const r of results) {
        if (r.pick) saved[r.game_id] = r.pick
        else failures.push(r.error ?? 'Something went wrong')
      }
      set((s) => ({ picksByGameId: { ...s.picksByGameId, ...saved } }))

      if (failures.length > 0) {
        const unique = [...new Set(failures)]
        set({
          submitError:
            failures.length === 1
              ? unique[0]
              : `${failures.length} picks couldn't be saved: ${unique.join(', ')}`,
        })
        return false
      }
      return true
    } catch {
      set({ submitError: 'Something went wrong submitting your picks.' })
      return false
    }
  },
}))
