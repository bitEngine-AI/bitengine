import { useState, useEffect } from 'react'
import { Cpu, LogOut, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '../stores/authStore'
import { useAppStore } from '../stores/appStore'
import AIPanel from '../components/AIPanel'
import AppCard from '../components/AppCard'

export default function Desktop() {
  const { t } = useTranslation()
  const { apps, loading, fetchApps } = useAppStore()
  const { logout } = useAuthStore()
  const [showAI, setShowAI] = useState(false)

  useEffect(() => {
    fetchApps()
  }, [fetchApps])

  return (
    <div className="min-h-screen bg-gray-950">
      {/* Header */}
      <header className="border-b border-gray-800 px-6 py-4 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Cpu className="w-6 h-6 text-blue-500" />
          <h1 className="text-xl font-bold text-gray-100">BitEngine</h1>
        </div>
        <button
          onClick={logout}
          className="text-gray-400 hover:text-gray-200 flex items-center gap-2 transition"
        >
          <LogOut className="w-4 h-4" />
          {t('desktop.logout')}
        </button>
      </header>

      {/* App Grid */}
      <main className="p-6">
        {loading ? (
          <div className="text-center text-gray-500 py-20">{t('desktop.loading')}</div>
        ) : apps.length === 0 ? (
          <div className="text-center py-20">
            <p className="text-gray-500 mb-4">
              {t('desktop.empty')}
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
            {apps.map((app) => (
              <AppCard key={app.id} app={app} />
            ))}
          </div>
        )}
      </main>

      {/* FAB + AI Panel */}
      {!showAI && (
        <button
          onClick={() => setShowAI(true)}
          className="fixed bottom-6 right-6 bg-blue-600 hover:bg-blue-700 text-white rounded-full w-14 h-14 flex items-center justify-center shadow-lg transition"
        >
          <Plus className="w-6 h-6" />
        </button>
      )}
      {showAI && <AIPanel onClose={() => setShowAI(false)} />}
    </div>
  )
}
