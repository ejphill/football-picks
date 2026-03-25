import { useState } from 'react'
import { useAuthStore } from '../stores/authStore'
import { updateMe } from '../api/client'

export default function Profile() {
  const { user, loadUser, signOut } = useAuthStore()

  const [displayName, setDisplayName] = useState(user?.display_name ?? '')
  const [notifyEmail, setNotifyEmail] = useState(user?.notify_email ?? true)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

  if (!user) return null

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setError('')
    setSaved(false)
    try {
      await updateMe({ display_name: displayName.trim(), notify_email: notifyEmail })
      await loadUser()
      setSaved(true)
      setTimeout(() => setSaved(false), 2500)
    } catch {
      setError('Could not save changes.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="max-w-sm mx-auto px-4 py-8 space-y-6">
      <h2 className="text-xl font-bold text-gray-900">Profile</h2>

      <form onSubmit={handleSave} className="space-y-5">
        <div className="space-y-1">
          <label className="block text-sm font-medium text-gray-700">Display name</label>
          <input
            type="text"
            required
            maxLength={40}
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            className="w-full border border-gray-300 rounded-lg px-4 py-2.5 text-gray-900 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
        </div>

        <div className="space-y-1">
          <label className="block text-sm font-medium text-gray-700">Email</label>
          <p className="text-gray-500 text-sm">{user.email}</p>
        </div>

        <div className="flex items-center gap-3">
          <button
            type="button"
            role="switch"
            aria-checked={notifyEmail}
            onClick={() => setNotifyEmail((v) => !v)}
            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500 ${
              notifyEmail ? 'bg-indigo-600' : 'bg-gray-300'
            }`}
          >
            <span
              className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                notifyEmail ? 'translate-x-6' : 'translate-x-1'
              }`}
            />
          </button>
          <span className="text-sm text-gray-700">Sunday reminder emails</span>
        </div>

        {error && <p className="text-red-600 text-sm">{error}</p>}
        {saved && <p className="text-green-600 text-sm">Saved!</p>}

        <button
          type="submit"
          disabled={saving}
          className="w-full bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white font-semibold py-2.5 rounded-lg transition-colors"
        >
          {saving ? 'Saving…' : 'Save changes'}
        </button>
      </form>

      <div className="border-t border-gray-100 pt-5">
        <button
          onClick={signOut}
          className="text-sm text-red-500 hover:text-red-700 transition-colors"
        >
          Sign out
        </button>
      </div>
    </div>
  )
}
