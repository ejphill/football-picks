import { Navigate } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'

interface Props {
  children: React.ReactNode
}

export default function ProtectedRoute({ children }: Props) {
  const { session, user, loading, signOut } = useAuthStore()

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="w-8 h-8 border-4 border-indigo-500 border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  if (!session) return <Navigate to="/login" replace />

  if (!user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 px-4">
        <div className="max-w-sm w-full text-center space-y-4">
          <h1 className="text-xl font-bold text-gray-900">Account setup incomplete</h1>
          <p className="text-gray-500 text-sm">
            You're signed in, but there's no player profile for this account yet.
          </p>
          <a
            href="/register"
            className="inline-block w-full bg-indigo-600 hover:bg-indigo-700 text-white font-semibold py-3 rounded-lg transition-colors"
          >
            Finish sign up
          </a>
          <button onClick={signOut} className="text-sm text-indigo-600 underline">
            Sign out
          </button>
        </div>
      </div>
    )
  }

  return <>{children}</>
}
