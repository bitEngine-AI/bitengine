import { Loader2, Check, AlertCircle, X } from 'lucide-react'

export interface Step {
  step: number
  name: string
  status: 'pending' | 'running' | 'done' | 'warning' | 'error'
}

interface ProgressProps {
  steps: Step[]
}

const STEP_LABELS: Record<string, string> = {
  intent: '意图理解',
  codegen: '代码生成',
  review: '代码审查',
  build: '镜像构建',
  deploy: '容器部署',
  route: '路由配置',
}

const STATUS_TEXT: Record<Step['status'], string> = {
  pending: '等待中',
  running: '进行中...',
  done: '完成',
  warning: '警告',
  error: '失败',
}

export default function Progress({ steps }: ProgressProps) {
  return (
    <div className="space-y-3">
      {steps.map((s) => {
        const label = STEP_LABELS[s.name] || s.name
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
                {STATUS_TEXT[s.status]}
              </span>
            </div>
          </div>
        )
      })}
    </div>
  )
}
