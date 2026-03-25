import { create } from 'zustand'
import type { WeeklyLeaderboardResponse, SeasonLeaderboardEntry } from '../types'
import { getWeeklyLeaderboard, getSeasonLeaderboard } from '../api/client'

interface LeaderboardState {
  weeklyData: WeeklyLeaderboardResponse | null
  seasonData: SeasonLeaderboardEntry[]
  weeklyLoading: boolean
  seasonLoading: boolean
  loadWeekly: (week: number, season: number) => Promise<void>
  loadSeason: (season: number) => Promise<void>
}

export const useLeaderboardStore = create<LeaderboardState>((set) => ({
  weeklyData: null,
  seasonData: [],
  weeklyLoading: false,
  seasonLoading: false,

  loadWeekly: async (week, season) => {
    set({ weeklyLoading: true })
    try {
      const { data } = await getWeeklyLeaderboard(week, season)
      set({ weeklyData: data, weeklyLoading: false })
    } catch {
      set({ weeklyLoading: false })
    }
  },

  loadSeason: async (season) => {
    set({ seasonLoading: true })
    try {
      const { data } = await getSeasonLeaderboard(season)
      set({ seasonData: data.entries, seasonLoading: false })
    } catch {
      set({ seasonLoading: false })
    }
  },
}))
