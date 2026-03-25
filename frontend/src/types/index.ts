export interface User {
  id: string
  supabase_uid: string
  display_name: string
  email: string
  notify_email: boolean
  is_admin: boolean
  created_at: string
  updated_at: string
}

export interface Season {
  id: number
  year: number
  is_active: boolean
}

export interface Week {
  id: number
  season_id: number
  season_year: number
  week_number: number
  picks_lock_at: string
}

export interface Game {
  id: string
  week_id: number
  espn_game_id: string
  home_team: string
  away_team: string
  home_team_name: string
  away_team_name: string
  spread: number | null
  kickoff_at: string
  home_score: number | null
  away_score: number | null
  winner: 'home' | 'away' | 'tie' | null
  status: 'scheduled' | 'in_progress' | 'final'
  included_in_picks: boolean
}

export interface Pick {
  id: string
  user_id: string
  game_id: string
  picked_team: 'home' | 'away'
  is_correct: boolean | null
  created_at: string
  updated_at: string
}

export interface PickView {
  game_id: string
  picked_team: 'home' | 'away' | ''
  is_correct: boolean | null
}

export interface WeeklyLeaderboardEntry {
  user_id: string
  display_name: string
  picks: PickView[]
  correct: number
  total: number
}

export interface WeeklyLeaderboardGame {
  id: string
  home_team: string
  away_team: string
  home_team_name: string
  away_team_name: string
  winner: 'home' | 'away' | 'tie' | null
}

export interface WeeklyLeaderboardResponse {
  locked: boolean
  games: WeeklyLeaderboardGame[]
  entries: WeeklyLeaderboardEntry[]
  total: number
  limit: number
  offset: number
}

export interface SeasonLeaderboardResponse {
  entries: SeasonLeaderboardEntry[]
  total: number
  limit: number
  offset: number
}

export interface SeasonLeaderboardEntry {
  rank: number
  user_id: string
  display_name: string
  correct: number
  total: number
  win_pct: number
}

export interface DraftSections {
  intro: string
  results: string
  records: string
  pre_games_note: string
  games: string
  outro: string
}

export interface Announcement {
  id: string
  author_id: string
  week_id: number
  intro: string
  body_json: string | null
  published_at: string
  created_at: string
}
