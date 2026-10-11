// Mirrors internal/notify/mailer.go's introHTML/introPlain exactly, so a
// clipboard copy (rich HTML + plain fallback) matches what the real sent
// email looks like. Keep in sync with that file.

export const DAY_COLORS: Record<string, string> = {
  Tuesday: '#0f766e',
  Wednesday: '#6d28d9',
  Thursday: '#c2410c',
  Friday: '#b45309',
  Saturday: '#1d4ed8',
  Sunday: '#15803d',
  Monday: '#b91c1c',
}

const DAY_HEADER_RE = /^(Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday|Monday):/gm
const BOLD_RE = /\*\*(.*?)\*\*/g

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

// Strips **bold** markers — same as introPlain, for plain-text fallback.
export function toPlainText(text: string): string {
  return text.replace(BOLD_RE, '$1')
}

// Escapes HTML, converts **bold** -> <strong>, colors day headers, and
// newlines -> <br> — same transforms as introHTML, same order.
export function toAnnouncementHtml(text: string): string {
  const escaped = escapeHtml(text)
  const withBold = escaped.replace(BOLD_RE, '<strong>$1</strong>')
  const withDayColors = withBold.replace(DAY_HEADER_RE, (match, day: string) => {
    return `<strong style="color:${DAY_COLORS[day]}">${match}</strong>`
  })
  return withDayColors.replace(/\n/g, '<br>')
}

// Copies both a rich-HTML and plain-text version to the clipboard at once —
// pasting into a rich-text target (like Gmail's compose box) picks up the
// HTML version (bold, colored day headers); a plain-text target falls back
// to the stripped version automatically.
export async function copyFormatted(text: string): Promise<void> {
  const html = toAnnouncementHtml(text)
  const plain = toPlainText(text)

  if (typeof ClipboardItem !== 'undefined') {
    const item = new ClipboardItem({
      'text/html': new Blob([html], { type: 'text/html' }),
      'text/plain': new Blob([plain], { type: 'text/plain' }),
    })
    await navigator.clipboard.write([item])
  } else {
    // Older browsers without rich clipboard support — plain text only.
    await navigator.clipboard.writeText(plain)
  }
}
