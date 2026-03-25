import { useEffect } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { supabase } from './api/supabase'
import { useAuthStore } from './stores/authStore'
import ProtectedRoute from './components/ProtectedRoute'
import Nav from './components/Nav'
import Login from './pages/Login'
import Register from './pages/Register'
import Picks from './pages/Picks'
import Leaderboard from './pages/Leaderboard'
import Profile from './pages/Profile'
import AdminCompose from './pages/AdminCompose'
import AdminGames from './pages/AdminGames'
import Home from './pages/Home'
import ResetPassword from './pages/ResetPassword'

function AppShell() {
  return (
    <div className="min-h-screen bg-gray-50">
      <Nav />
      <main>
        <Routes>
          <Route path="/picks" element={<ProtectedRoute><Picks /></ProtectedRoute>} />
          <Route path="/leaderboard" element={<ProtectedRoute><Leaderboard /></ProtectedRoute>} />
          <Route path="/profile" element={<ProtectedRoute><Profile /></ProtectedRoute>} />
          <Route path="/admin/compose" element={<ProtectedRoute><AdminCompose /></ProtectedRoute>} />
          <Route path="/admin/games" element={<ProtectedRoute><AdminGames /></ProtectedRoute>} />
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/reset-password" element={<ResetPassword />} />
          <Route path="/" element={<ProtectedRoute><Home /></ProtectedRoute>} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </div>
  )
}

export default function App() {
  const { setSession, loadUser } = useAuthStore()

  useEffect(() => {
    // Hydrate session on mount.
    supabase.auth.getSession().then(({ data }) => {
      setSession(data.session)
      if (data.session) {
        loadUser()
      } else {
        useAuthStore.setState({ loading: false })
      }
    })

    // Keep session in sync with Supabase auth events.
    const { data: listener } = supabase.auth.onAuthStateChange((_event, session) => {
      setSession(session)
      if (session) {
        loadUser()
      } else {
        useAuthStore.setState({ user: null, loading: false })
      }
    })

    return () => listener.subscription.unsubscribe()
  }, [])

  return (
    <BrowserRouter>
      <AppShell />
    </BrowserRouter>
  )
}
