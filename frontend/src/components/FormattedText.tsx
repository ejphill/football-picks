// renders **bold** markers and preserves newlines
export default function FormattedText({ text, className }: { text: string; className?: string }) {
  const lines = text.split('\n')
  return (
    <span className={className}>
      {lines.map((line, li) => {
        const parts = line.split(/\*\*(.*?)\*\*/)
        return (
          <span key={li}>
            {parts.map((part, pi) =>
              pi % 2 === 1 ? <strong key={pi}>{part}</strong> : part
            )}
            {li < lines.length - 1 && <br />}
          </span>
        )
      })}
    </span>
  )
}
