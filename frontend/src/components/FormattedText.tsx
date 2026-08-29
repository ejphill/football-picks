// Mirrors internal/notify/mailer.go's dayColors — keep hex values in sync
// so the compose preview matches the actual sent email.
const DAY_COLORS: Record<string, string> = {
  Tuesday: '#0f766e',
  Wednesday: '#6d28d9',
  Thursday: '#c2410c',
  Friday: '#b45309',
  Saturday: '#1d4ed8',
  Sunday: '#15803d',
  Monday: '#b91c1c',
}

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
