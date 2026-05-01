import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Cpu } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '../stores/authStore'
import { setupAPI } from '../api/client'

interface SetupProps {
  onSetupComplete: () => void
}

export default function Setup({ onSetupComplete }: SetupProps) {
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { isAuthenticated, login } = useAuthStore()

  const [phase, setPhase] = useState<'loading' | 'create' | 'login'>('loading')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    setupAPI
      .status()
      .then((s) => {
        if (s.completed && isAuthenticated) {
          navigate('/desktop', { replace: true })
        } else {
          setPhase(s.completed ? 'login' : 'create')
        }
      })
      .catch(() => setPhase('create'))
  }, [isAuthenticated, navigate])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setError('')

    if (password !== confirmPassword) {
      setError(t('setup.errorPasswordMismatch'))
      return
    }
    if (password.length < 6) {
      setError(t('setup.errorPasswordShort'))
      return
    }
    if (!username.trim()) {
      setError(t('setup.errorUsernameRequired'))
      return
    }

    setSubmitting(true)
    try {
      await setupAPI.step1(username, password)
      onSetupComplete()
      setPhase('login')
      setPassword('')
      setConfirmPassword('')
    } catch (err: any) {
      setError(err.message || t('setup.errorSetupFailed'))
    } finally {
      setSubmitting(false)
    }
  }

  async function handleLogin(e: React.FormEvent) {
    e.preventDefault()
    setError('')

    if (!username.trim() || !password) {
      setError(t('setup.errorLoginRequired'))
      return
    }

    setSubmitting(true)
    try {
      await login(username, password)
      navigate('/desktop', { replace: true })
    } catch (err: any) {
      setError(err.message || t('setup.errorLoginFailed'))
    } finally {
      setSubmitting(false)
    }
  }

  if (phase === 'loading') {
    return <div className="min-h-screen bg-gray-950" />
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-950">
      <div className="bg-gray-900 rounded-2xl p-8 w-full max-w-md shadow-xl">
        <div className="flex items-center justify-center gap-3 mb-2">
          <Cpu className="w-8 h-8 text-blue-500" />
          <h1 className="text-2xl font-bold text-gray-100">BitEngine</h1>
        </div>

        {phase === 'create' ? (
          <>
            <p className="text-center text-gray-400 mb-8">
              {t('setup.title')}
            </p>

            <form onSubmit={handleCreate} className="space-y-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1">{t('setup.username')}</label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder={t('setup.usernamePlaceholder')}
                  autoFocus
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">{t('setup.password')}</label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder={t('setup.passwordPlaceholder')}
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">{t('setup.confirmPassword')}</label>
                <input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder={t('setup.confirmPlaceholder')}
                />
              </div>

              {error && <p className="text-red-400 text-sm">{error}</p>}

              <button
                type="submit"
                disabled={submitting}
                className="w-full bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg py-3 font-medium transition"
              >
                {submitting ? t('setup.creating') : t('setup.createAccount')}
              </button>
            </form>
          </>
        ) : (
          <>
            <p className="text-center text-gray-400 mb-8">{t('setup.login')}</p>

            <form onSubmit={handleLogin} className="space-y-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1">{t('setup.username')}</label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder={t('setup.usernamePlaceholder')}
                  autoFocus
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">{t('setup.password')}</label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder={t('setup.loginPlaceholder')}
                />
              </div>

              {error && <p className="text-red-400 text-sm">{error}</p>}

              <button
                type="submit"
                disabled={submitting}
                className="w-full bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg py-3 font-medium transition"
              >
                {submitting ? t('setup.loggingIn') : t('setup.loginButton')}
              </button>
            </form>
          </>
        )}
      </div>
    </div>
  )
}
