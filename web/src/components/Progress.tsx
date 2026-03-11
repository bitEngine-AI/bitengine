import { Loader2, Check, AlertCircle, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export interface Step {
  step: number
  name: string
  status: 'pending' | 'running' | 'done' | 'warning' | 'error'
}

interface ProgressProps {
  steps: Step[]
}

export default function Progress({ steps }: ProgressProps) {
  const { t } = useTranslation()

  return (
    <div className="space-y-3">
      {steps.map((s) => {
        const label = t(`progress.steps.${s.name}`, s.name)
        const statusText = t(`progress.status.${s.status}`)
        return (
          <div key={s.step} className="flex items-center gap-3">
            {s.status === 'pending' && (
              <div className="w-5 h-5 rounded-full border-2 border-gray-600" />
            )}
            {s.status === 'running' && (
              <Loader2 className="w-5 h-5 text-blue-500 animate-spin" />
            )}
            {s.status === 'done' && (
              <Check className="w-5 h-5 text-green-500" />
            )}
            {s.status === 'warning' && (
              <AlertCircle className="w-5 h-5 text-yellow-500" />
            )}
            {s.status === 'error' && (
              <X className="w-5 h-5 text-red-500" />
            )}

            <div className="flex flex-col">
              <span
                className={
                  s.status === 'pending' ? 'text-gray-500' : 'text-gray-200'
                }
              >
                {label}
              </span>
              <span className="text-xs text-gray-500">
                {statusText}
              </span>
            </div>
          </div>
        )
      })}
    </div>
  )
}
