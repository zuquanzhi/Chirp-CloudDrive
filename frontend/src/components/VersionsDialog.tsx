import { useEffect, useState } from 'react'
import { Download, History, RotateCcw } from 'lucide-react'
import { toast } from 'sonner'
import { downloadFile, listVersions, restoreVersion } from '@/lib/api'
import { formatSize, formatTime } from '@/lib/format'
import type { DriveFile } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'

export default function VersionsDialog({
  file,
  onClose,
  onRestored,
}: {
  file: DriveFile | null
  onClose: () => void
  onRestored: () => void
}) {
  const [versions, setVersions] = useState<DriveFile[]>([])
  const [loading, setLoading] = useState(false)
  const [restoring, setRestoring] = useState<number | null>(null)

  useEffect(() => {
    if (!file) return
    setLoading(true)
    listVersions(file.id)
      .then((data) => setVersions(data.versions ?? []))
      .catch(() => toast.error('版本列表加载失败'))
      .finally(() => setLoading(false))
  }, [file])

  const handleRestore = async (versionId: number) => {
    if (!file) return
    setRestoring(versionId)
    try {
      await restoreVersion(file.id, versionId)
      toast.success('已恢复为该版本（生成新版本）')
      onRestored()
      onClose()
    } catch {
      toast.error('恢复失败')
    } finally {
      setRestoring(null)
    }
  }

  return (
    <Dialog open={file !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <History className="h-5 w-5 text-sky-600" />
            历史版本
          </DialogTitle>
          <DialogDescription className="truncate">
            {file?.original_name} · 同名上传会自动保留旧版本
          </DialogDescription>
        </DialogHeader>
        <div className="max-h-72 overflow-auto divide-y rounded-md border">
          {loading && <div className="py-10 text-center text-sm text-slate-400">加载中…</div>}
          {!loading && versions.length <= 1 && (
            <div className="py-10 text-center text-sm text-slate-400">暂无历史版本</div>
          )}
          {versions.map((v) => (
            <div key={v.id} className="flex items-center gap-3 px-3 py-2.5 text-sm">
              <Badge variant={v.is_latest ? 'default' : 'secondary'} className="shrink-0">
                v{v.version}
              </Badge>
              <div className="flex-1 min-w-0">
                <div className="text-slate-700">{formatTime(v.created_at)}</div>
                <div className="text-xs text-slate-400">{formatSize(v.size)}</div>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                title="下载此版本"
                onClick={() => downloadFile(v.id, v.original_name)}
              >
                <Download className="h-4 w-4" />
              </Button>
              {!v.is_latest && (
                <Button
                  variant="outline"
                  size="sm"
                  disabled={restoring !== null}
                  onClick={() => handleRestore(v.id)}
                >
                  <RotateCcw className="h-3.5 w-3.5 mr-1" />
                  恢复
                </Button>
              )}
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  )
}
