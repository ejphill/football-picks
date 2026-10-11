import { DAY_COLORS } from '../utils/announcementFormat'

const DAY_HEADER_RE = /^(Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday|Monday):$/

// renders **bold** markers, colored day headers (e.g. "Sunday:"), and preserves newlines
export default function FormattedText({ text, className }: { text: string; className?: string }) {
  const lines = text.split('\n')
  return (
    <span className={className}>
      {lines.map((line, li) => {
        const dayMatch = line.trim().match(DAY_HEADER_RE)
        return (
          <span key={li}>
            {dayMatch ? (
              <strong style={{ color: DAY_COLORS[dayMatch[1]] }}>{line}</strong>
            ) : (
              line.split(/\*\*(.*?)\*\*/).map((part, pi) =>
                pi % 2 === 1 ? <strong key={pi}>{part}</strong> : part
              )
            )}
            {li < lines.length - 1 && <br />}
          </span>
        )
      })}
    </span>
  )
}
