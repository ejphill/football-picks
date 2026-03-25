import axios from 'axios'
import { supabase } from './supabase'
import type {
  Game,
  Pick,
  User,
  Week,
  WeeklyLeaderboardResponse,
  SeasonLeaderboardResponse,
  Announcement,
  DraftSections,
} from '../types'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? 'http://localhost:8080/api/v1',
})

// Attach the Supabase JWT to every request.
api.interceptors.request.use(async (config) => {
  const { data } = await supabase.auth.getSession()
  if (data.session?.access_token) {
    config.headers.Authorization = `Bearer ${data.session.access_token}`
  }
  return config
})

// ---- Auth / Users ----

export const register = (displayName: string, email: string) =>
  api.post<User>('/auth/register', { display_name: displayName, email })

export const getMe = () => api.get<User>('/users/me')

export const updateMe = (patch: { display_name?: string; notify_email?: boolean }) =>
  api.patch<User>('/users/me', patch)

// ---- Weeks ----

export const getActiveWeek = () => api.get<Week>('/weeks/active')

// ---- Games ----

export const getGames = (week: number, season: number) =>
  api.get<Game[]>(`/games?week=${week}&season=${season}`)

export const getGame = (gameId: string) => api.get<Game>(`/games/${gameId}`)

// ---- Picks ----

export const getPicks = (week: number, season: number) =>
  api.get<Pick[]>(`/picks?week=${week}&season=${season}`)

export const submitPick = (gameId: string, pickedTeam: 'home' | 'away') =>
  api.post<Pick>('/picks', { game_id: gameId, picked_team: pickedTeam })

export const deletePick = (gameId: string) => api.delete(`/picks/${gameId}`)

// ---- Leaderboard ----

export const getWeeklyLeaderboard = (week: number, season: number) =>
  api.get<WeeklyLeaderboardResponse>(`/leaderboard/weekly?week=${week}&season=${season}`)

export const getSeasonLeaderboard = (season: number) =>
  api.get<SeasonLeaderboardResponse>(`/leaderboard/season?season=${season}`)

// ---- Announcements ----

export const getAnnouncements = (season: number) =>
  api.get<Announcement[]>(`/announcements?season=${season}`)

export const getAnnouncement = (id: string) => api.get<Announcement>(`/announcements/${id}`)

export const postAnnouncement = (weekId: number, intro: string) =>
  api.post<Announcement>('/admin/announcements', { week_id: weekId, intro })

export const syncGames = (week: number, season: number, seasonType = 2) =>
  api.post(`/admin/sync-games?week=${week}&season=${season}&seasontype=${seasonType}`)

export const getAdminGames = (week: number, season: number) =>
  api.get<Game[]>(`/admin/games?week=${week}&season=${season}`)

export const setGameIncluded = (gameId: string, includedInPicks: boolean) =>
  api.patch<Game>(`/admin/games/${gameId}`, { included_in_picks: includedInPicks })

export const getDraftAnnouncement = (week: number, season: number) =>
  api.get<DraftSections>(`/admin/draft-announcement?week=${week}&season=${season}`)

export default api
