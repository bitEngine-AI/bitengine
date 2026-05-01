import { Play, Square, Trash2, ExternalLink, Pencil } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { appsAPI, type AppInfo } from '../api/client'
import { useAppStore } from '../stores/appStore'

interface AppCardProps {
  app: AppInfo
  onModify?: (appId: string, appName: string) => void
}

const STATUS_DOT: Record<string, string> = {
  running: 'bg-green-500',
  stopped: 'bg-gray-500',
  creating: 'bg-blue-500 animate-pulse',
  error: 'bg-red-500',
}

export default function AppCard({ app, onModify }: AppCardProps) {
  const { t } = useTranslation()
  const dotClass = STATUS_DOT[app.status] || 'bg-gray-500'

  async function handleStart() {
    try {
      await appsAPI.start(app.id)
      useAppStore.getState().fetchApps()
    } catch (err) {
      console.error('app: start failed:', err)
    }
  }

  async function handleStop() {
    try {
      await appsAPI.stop(app.id)
      useAppStore.getState().fetchApps()
    } catch (err) {
      console.error('app: stop failed:', err)
    }
  }

  async function handleDelete() {
    try {
      await appsAPI.delete(app.id)
      useAppStore.getState().fetchApps()
    } catch (err) {
      console.error('app: delete failed:', err)
    }
  }

  return (
    <div className="bg-gray-900 rounded-xl border border-gray-800 p-4 flex flex-col gap-3">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h3 className="font-medium text-gray-100 truncate">{app.name}</h3>
        <span className={`w-2.5 h-2.5 rounded-full flex-shrink-0 ${dotClass}`} />
      </div>

      {/* Prompt preview */}
      <p className="text-sm text-gray-400 line-clamp-2">{app.prompt}</p>

      {/* Actions */}
      <div className="flex items-center gap-2 mt-auto pt-2 border-t border-gray-800">
        {app.status === 'running' ? (
          <button
            onClick={handleStop}
            className="inline-flex items-center gap-1 text-sm text-gray-300 hover:text-gray-100 transition-colors"
          >
            <Square className="w-4 h-4" />
            {t('app.stop')}
          </button>
        ) : (
          <button
            onClick={handleStart}
            className="inline-flex items-center gap-1 text-sm text-gray-300 hover:text-gray-100 transition-colors"
          >
            <Play className="w-4 h-4" />
            {t('app.start')}
          </button>
        )}

        <a
          href={`http://${app.domain}`}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 text-sm text-gray-300 hover:text-gray-100 transition-colors"
        >
          <ExternalLink className="w-4 h-4" />
          {t('app.visit')}
        </a>

        <button
          onClick={() => onModify?.(app.id, app.name)}
          className="inline-flex items-center gap-1 text-sm text-gray-300 hover:text-gray-100 transition-colors"
        >
          <Pencil className="w-4 h-4" />
          {t('app.modify')}
        </button>

        <button
          onClick={handleDelete}
          className="inline-flex items-center gap-1 text-sm text-red-400 hover:text-red-300 ml-auto transition-colors"
        >
          <Trash2 className="w-4 h-4" />
        </button>
      </div>
    </div>
  )
}
