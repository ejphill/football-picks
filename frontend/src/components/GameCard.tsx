import type { Game } from '../types'

interface GameCardProps {
  game: Game
  selected: 'home' | 'away' | null
  locked: boolean
  onSelect: (gameId: string, team: 'home' | 'away') => void
}

export default function GameCard({ game, selected, locked: isLocked, onSelect }: GameCardProps) {

  const spreadStr =
    game.spread != null
      ? game.spread === 0
        ? 'PK'
        : game.spread > 0
          ? `+${game.spread}`
          : `${game.spread}`
      : 'N/A'

  const teamButton = (side: 'home' | 'away') => {
    const name = side === 'home' ? game.home_team_name : game.away_team_name
    const abbr = side === 'home' ? game.home_team : game.away_team
    const isPicked = selected === side

    const base =
      'flex-1 flex flex-col items-center py-3 px-2 rounded-lg border-2 transition-all select-none'
    const style = isLocked
      ? 'border-gray-100 bg-gray-50 cursor-not-allowed opacity-60'
      : isPicked
        ? 'border-indigo-500 bg-indigo-50 cursor-pointer'
        : 'border-gray-200 hover:border-gray-400 bg-white cursor-pointer'

    return (
      <button
        key={side}
        onClick={() => !isLocked && onSelect(game.id, side)}
        className={`${base} ${style}`}
        aria-pressed={isPicked}
        disabled={isLocked}
      >
        <span className="text-lg font-bold text-gray-800">{abbr}</span>
        <span className="text-xs text-gray-500 mt-0.5 truncate max-w-full">{name}</span>
      </button>
    )
  }

  return (
    <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-4 space-y-3">
      <div className="flex justify-between items-center">
        <span className="text-xs text-gray-400">
          Spread: <span className="font-medium text-gray-600">{spreadStr}</span>
        </span>
        {isLocked && (
          <span className="text-xs font-medium text-gray-400">Locked</span>
        )}
      </div>
      <div className="flex gap-3">
        {teamButton('away')}
        <div className="flex items-center text-gray-400 font-bold text-sm">@</div>
        {teamButton('home')}
      </div>
    </div>
  )
}
