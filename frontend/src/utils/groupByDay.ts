import type { Game } from '../types'

export const DAY_ORDER = ['Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday', 'Monday']

export function groupByDay(games: Game[]): [string, Game[]][] {
  const dayMap = new Map<string, Game[]>()
  for (const g of games) {
    const day = new Date(g.kickoff_at).toLocaleDateString('en-US', {
      weekday: 'long',
      timeZone: 'America/New_York',
    })
    const list = dayMap.get(day) ?? []
    list.push(g)
    dayMap.set(day, list)
  }
  return [...dayMap.entries()].sort((a, b) => {
    const ai = DAY_ORDER.indexOf(a[0])
    const bi = DAY_ORDER.indexOf(b[0])
    return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi)
  })
}
