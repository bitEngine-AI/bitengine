import { useState, useRef } from 'react'
import { X, Send, ExternalLink } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { createAppSSE, regenerateAppSSE } from '../api/client'
import { useAppStore } from '../stores/appStore'
import Progress, { type Step } from './Progress'

interface AIPanelProps {
  onClose: () => void
  modifyAppId?: string
  modifyAppName?: string
}

const CREATE_STEPS: Step[] = [
  { step: 1, name: 'intent', status: 'pending' },
  { step: 2, name: 'codegen', status: 'pending' },
  { step: 3, name: 'review', status: 'pending' },
  { step: 4, name: 'build', status: 'pending' },
  { step: 5, name: 'deploy', status: 'pending' },
  { step: 6, name: 'route', status: 'pending' },
]

const MODIFY_STEPS: Step[] = [
  { step: 1, name: 'load', status: 'pending' },
  { step: 2, name: 'codegen', status: 'pending' },
  { step: 3, name: 'review', status: 'pending' },
  { step: 4, name: 'build', status: 'pending' },
  { step: 5, name: 'deploy', status: 'pending' },
]

export default function AIPanel({ onClose, modifyAppId, modifyAppName }: AIPanelProps) {
  const { t } = useTranslation()
  const isModify = !!modifyAppId
  const initialSteps = isModify ? MODIFY_STEPS : CREATE_STEPS
  const [prompt, setPrompt] = useState('')
  const [generating, setGenerating] = useState(false)
  const [steps, setSteps] = useState<Step[]>(initialSteps)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<{
    app_id: string
    slug: string
    domain: string
    url: string
  } | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = prompt.trim()
    if (!trimmed || generating) return

    setGenerating(true)
    setError(null)
    setResult(null)
    setSteps(initialSteps.map((s) => ({ ...s, status: 'pending' })))

    const onEvent = (event: string, data: any) => {
      if (event === 'step') {
        setSteps((prev) =>
          prev.map((s) =>
            s.step === data.step ? { ...s, status: data.status, name: data.name } : s,
          ),
        )
      } else if (event === 'complete') {
        setResult({
          app_id: data.app_id,
          slug: data.slug,
          domain: data.domain,
          url: data.url,
        })
        setGenerating(false)
        useAppStore.getState().fetchApps()
      } else if (event === 'error') {
        setError(data.message || t('ai.errorGenerate'))
        setGenerating(false)
      }
    }

    const onErr = (err: Error) => {
      setError(err.message || t('ai.errorConnect'))
      setGenerating(false)
    }

    if (isModify) {
      abortRef.current = regenerateAppSSE(modifyAppId!, trimmed, onEvent, onErr)
    } else {
      abortRef.current = createAppSSE(trimmed, onEvent, onErr)
    }
  }

  function handleClose() {
    if (abortRef.current) {
      abortRef.current.abort()
      abortRef.current = null
    }
    onClose()
  }

  function handleReset() {
    setPrompt('')
    setGenerating(false)
    setSteps(initialSteps.map((s: Step) => ({ ...s, status: 'pending' })))
    setError(null)
    setResult(null)
  }

  return (
    <div className="fixed bottom-6 right-6 w-96 bg-gray-900 rounded-2xl shadow-2xl border border-gray-800 flex flex-col max-h-[80vh] z-50">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-800">
        <h2 className="text-sm font-semibold text-gray-100">
          {isModify ? `${t('ai.modifyTitle')}: ${modifyAppName}` : t('ai.title')}
        </h2>
        <button
          onClick={handleClose}
          className="text-gray-400 hover:text-gray-200 transition-colors"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto p-4">
        {error && (
          <div className="mb-4 p-3 bg-red-900/30 border border-red-800 rounded-lg text-sm text-red-300">
            {error}
          </div>
        )}

        {generating && <Progress steps={steps} />}

        {result && !generating && (
          <div className="space-y-3">
            <div className="p-3 bg-green-900/30 border border-green-800 rounded-lg text-sm text-green-300">
              {t('ai.success')}
            </div>
            <a
              href={result.url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 text-blue-400 hover:text-blue-300 text-sm transition-colors"
            >
              <ExternalLink className="w-4 h-4" />
              {result.domain}
            </a>
            <button
              onClick={handleReset}
              className="text-sm text-gray-400 hover:text-gray-200 transition-colors"
            >
              {t('ai.continueCreate')}
            </button>
          </div>
        )}

        {!generating && !result && !error && (
          <p className="text-sm text-gray-500">
            {isModify ? t('ai.modifyHint') : t('ai.hint')}
          </p>
        )}
      </div>

      {/* Footer input */}
      {!generating && !result && (
        <form
          onSubmit={handleSubmit}
          className="p-4 border-t border-gray-800 flex gap-2"
        >
          <input
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder={isModify ? t('ai.modifyPlaceholder') : t('ai.placeholder')}
            className="flex-1 bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <button
            type="submit"
            disabled={!prompt.trim()}
            className="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg px-3 py-2 transition-colors"
          >
            <Send className="w-4 h-4" />
          </button>
        </form>
      )}
    </div>
  )
}
