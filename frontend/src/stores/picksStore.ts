import { create } from 'zustand'
import axios from 'axios'
import type { Pick } from '../types'
import { getPicks, submitPick as apiSubmitPick } from '../api/client'

interface PicksState {
  picksByGameId: Record<string, Pick>
  loading: boolean
  error: string | null
  submitError: string | null
  loadPicks: (week: number, season: number) => Promise<void>
  submitPick: (gameId: string, team: 'home' | 'away') => Promise<void>
}

export const usePicksStore = create<PicksState>((set, get) => ({
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

  submitPick: async (gameId, team) => {
    // Optimistic update.
    const prev = get().picksByGameId
    const optimistic = { ...prev[gameId], game_id: gameId, picked_team: team } as Pick
    set({ picksByGameId: { ...prev, [gameId]: optimistic } })

    try {
      const { data } = await apiSubmitPick(gameId, team)
      set((s) => ({ picksByGameId: { ...s.picksByGameId, [gameId]: data }, submitError: null }))
    } catch (e) {
      set({ picksByGameId: prev })
      if (axios.isAxiosError(e) && e.response?.status === 423) {
        set({ submitError: 'Picks are locked for this game.' })
      } else {
        set({ submitError: 'Something went wrong submitting your pick.' })
      }
    }
  },
}))
