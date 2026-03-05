import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { useAuthStore } from './stores/authStore'
import { setupAPI } from './api/client'
import Setup from './pages/Setup'
import Desktop from './pages/Desktop'

function RootRedirect({ setupDone }: { setupDone: boolean | null }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)

  if (setupDone === null) {
    return <div className="min-h-screen bg-gray-950" />
  }
  if (!setupDone) {
    return <Navigate to="/setup" replace />
  }
  if (!isAuthenticated) {
    return <Navigate to="/setup" replace />
  }
  return <Navigate to="/desktop" replace />
}

function ProtectedRoute({ setupDone, children }: { setupDone: boolean | null; children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)

  if (setupDone === null) {
    return <div className="min-h-screen bg-gray-950" />
  }
  if (!setupDone || !isAuthenticated) {
    return <Navigate to="/setup" replace />
  }
  return <>{children}</>
}

export default function App() {
  const [setupDone, setSetupDone] = useState<boolean | null>(null)

  useEffect(() => {
    useAuthStore.getState().restore()
    setupAPI
      .status()
      .then((s) => setSetupDone(s.completed))
      .catch(() => setSetupDone(false))
  }, [])

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/setup" element={<Setup onSetupComplete={() => setSetupDone(true)} />} />
        <Route
          path="/desktop"
          element={
            <ProtectedRoute setupDone={setupDone}>
              <Desktop />
            </ProtectedRoute>
          }
        />
        <Route path="*" element={<RootRedirect setupDone={setupDone} />} />
      </Routes>
    </BrowserRouter>
  )
}
