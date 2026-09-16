import { useEffect, useState } from 'react'
import {
  Activity as ActivityIcon,
  FileUp,
  FolderInput,
  FolderPlus,
  History,
  Link2,
  MoveRight,
  Pencil,
  RefreshCw,
  RotateCcw,
  Trash2,
  Undo2,
  Zap,
} from 'lucide-react'
import { toast } from 'sonner'
import { listActivities } from '@/lib/api'
import { formatTime } from '@/lib/format'
import type { Activity } from '@/types'
import { Button } from '@/components/ui/button'

const actionMeta: Record<string, { label: string; icon: typeof FileUp; className: string }> = {
  upload: { label: '上传了文件', icon: FileUp, className: 'text-sky-600 bg-sky-50' },
  instant_upload: { label: '秒传了文件', icon: Zap, className: 'text-amber-600 bg-amber-50' },
  chunked_upload: { label: '分片上传了文件', icon: FileUp, className: 'text-sky-600 bg-sky-50' },
  new_version: { label: '上传了新版本', icon: History, className: 'text-violet-600 bg-violet-50' },
  rename: { label: '重命名了文件', icon: Pencil, className: 'text-slate-600 bg-slate-100' },
  move: { label: '移动了文件', icon: MoveRight, className: 'text-slate-600 bg-slate-100' },
  delete: { label: '删除了', icon: Trash2, className: 'text-red-500 bg-red-50' },
  restore: { label: '还原了', icon: Undo2, className: 'text-emerald-600 bg-emerald-50' },
  hard_delete: { label: '彻底删除了', icon: Trash2, className: 'text-red-700 bg-red-50' },
  share_create: { label: '创建了分享', icon: Link2, className: 'text-sky-600 bg-sky-50' },
  share_cancel: { label: '取消了分享', icon: Link2, className: 'text-slate-500 bg-slate-100' },
  restore_version: { label: '恢复了历史版本', icon: RotateCcw, className: 'text-violet-600 bg-violet-50' },
  folder_create: { label: '创建了文件夹', icon: FolderPlus, className: 'text-amber-600 bg-amber-50' },
  folder_rename: { label: '重命名了文件夹', icon: Pencil, className: 'text-slate-600 bg-slate-100' },
  folder_move: { label: '移动了文件夹', icon: FolderInput, className: 'text-slate-600 bg-slate-100' },
}

export default function ActivitiesPage() {
  const [activities, setActivities] = useState<Activity[]>([])
  const [loading, setLoading] = useState(false)

  const refresh = () => {
    setLoading(true)
    listActivities(100)
      .then((data) => setActivities(data.activities ?? []))
      .catch(() => toast.error('加载失败'))
      .finally(() => setLoading(false))
  }

  useEffect(refresh, [])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-slate-800 flex items-center gap-2">
          <ActivityIcon className="h-5 w-5 text-sky-600" />
          最近动态
        </h1>
        <Button variant="ghost" size="icon" onClick={refresh} title="刷新">
          <RefreshCw className="h-4 w-4" />
        </Button>
      </div>
      <div className="rounded-lg border bg-white divide-y">
        {activities.length === 0 && !loading && (
          <div className="py-16 text-center text-slate-400">还没有任何操作记录</div>
        )}
        {activities.map((a) => {
          const meta = actionMeta[a.action] ?? {
            label: a.action,
            icon: ActivityIcon,
            className: 'text-slate-500 bg-slate-100',
          }
          const Icon = meta.icon
          return (
            <div key={a.id} className="flex items-center gap-3 px-4 py-3">
              <span className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-full ${meta.className}`}>
                <Icon className="h-4 w-4" />
              </span>
              <div className="flex-1 min-w-0">
                <p className="text-sm text-slate-700">
                  {meta.label}
                  {a.target_name && <span className="font-medium">「{a.target_name}」</span>}
                </p>
                {a.detail && <p className="text-xs text-slate-400">{a.detail}</p>}
              </div>
              <span className="text-xs text-slate-400 shrink-0">{formatTime(a.created_at)}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
