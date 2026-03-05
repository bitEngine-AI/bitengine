import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Cpu } from 'lucide-react'
import { useAuthStore } from '../stores/authStore'
import { setupAPI } from '../api/client'

interface SetupProps {
  onSetupComplete: () => void
}

export default function Setup({ onSetupComplete }: SetupProps) {
  const navigate = useNavigate()
  const { isAuthenticated, login } = useAuthStore()

  const [phase, setPhase] = useState<'loading' | 'create' | 'login'>('loading')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (isAuthenticated) {
      navigate('/desktop', { replace: true })
      return
    }
    setupAPI
      .status()
      .then((s) => setPhase(s.completed ? 'login' : 'create'))
      .catch(() => setPhase('create'))
  }, [isAuthenticated, navigate])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setError('')

    if (password !== confirmPassword) {
      setError('两次输入的密码不一致')
      return
    }
    if (password.length < 6) {
      setError('密码至少需要 6 个字符')
      return
    }
    if (!username.trim()) {
      setError('请输入用户名')
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
      setError(err.message || '设置失败，请重试')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleLogin(e: React.FormEvent) {
    e.preventDefault()
    setError('')

    if (!username.trim() || !password) {
      setError('请输入用户名和密码')
      return
    }

    setSubmitting(true)
    try {
      await login(username, password)
      navigate('/desktop', { replace: true })
    } catch (err: any) {
      setError(err.message || '登录失败，请检查用户名和密码')
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
              初始设置 &mdash; 创建管理员账户
            </p>

            <form onSubmit={handleCreate} className="space-y-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1">用户名</label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="admin"
                  autoFocus
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">密码</label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="至少 6 个字符"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">确认密码</label>
                <input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="再次输入密码"
                />
              </div>

              {error && <p className="text-red-400 text-sm">{error}</p>}

              <button
                type="submit"
                disabled={submitting}
                className="w-full bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg py-3 font-medium transition"
              >
                {submitting ? '创建中...' : '创建账户'}
              </button>
            </form>
          </>
        ) : (
          <>
            <p className="text-center text-gray-400 mb-8">登录</p>

            <form onSubmit={handleLogin} className="space-y-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1">用户名</label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="admin"
                  autoFocus
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">密码</label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="输入密码"
                />
              </div>

              {error && <p className="text-red-400 text-sm">{error}</p>}

              <button
                type="submit"
                disabled={submitting}
                className="w-full bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg py-3 font-medium transition"
              >
                {submitting ? '登录中...' : '登录'}
              </button>
            </form>
          </>
        )}
      </div>
    </div>
  )
}
