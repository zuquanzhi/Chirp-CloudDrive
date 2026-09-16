import { useEffect, useState } from 'react'
import { Copy, Link2, Loader2, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { createShare, deleteShare, listShares } from '@/lib/api'
import { formatSize, formatTime } from '@/lib/format'
import type { DriveFile, Share } from '@/types'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'

function shareLink(token: string) {
  return `${window.location.origin}/share/${token}`
}

export default function ShareDialog({ file, onClose }: { file: DriveFile | null; onClose: () => void }) {
  const [password, setPassword] = useState('')
  const [expireDays, setExpireDays] = useState('7')
  const [creating, setCreating] = useState(false)
  const [created, setCreated] = useState<Share | null>(null)
  const [existing, setExisting] = useState<Share[]>([])

  useEffect(() => {
    if (!file) return
    setCreated(null)
    setPassword('')
    listShares()
      .then((data) => setExisting((data.shares ?? []).filter((s) => s.resource_id === file.id)))
      .catch(() => setExisting([]))
  }, [file])

  const handleCreate = async () => {
    if (!file) return
    setCreating(true)
    try {
      const share = await createShare(file.id, password.trim(), Number(expireDays))
      setCreated(share)
      toast.success('分享链接已创建')
    } catch {
      toast.error('创建分享失败')
    } finally {
      setCreating(false)
    }
  }

  const copy = (text: string) => {
    navigator.clipboard.writeText(text).then(
      () => toast.success('已复制到剪贴板'),
      () => toast.error('复制失败'),
    )
  }

  const cancelShare = async (id: number) => {
    try {
      await deleteShare(id)
      setExisting((cur) => cur.filter((s) => s.id !== id))
      toast.success('分享已取消')
    } catch {
      toast.error('取消失败')
    }
  }

  return (
    <Dialog open={file !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Link2 className="h-5 w-5 text-sky-600" />
            分享文件
          </DialogTitle>
          <DialogDescription className="truncate">{file?.original_name}</DialogDescription>
        </DialogHeader>

        {created ? (
          <div className="space-y-3">
            <p className="text-sm text-slate-600">分享链接已生成：</p>
            <div className="flex gap-2">
              <Input readOnly value={shareLink(created.token)} className="text-xs" />
              <Button variant="outline" size="icon" onClick={() => copy(shareLink(created.token))}>
                <Copy className="h-4 w-4" />
              </Button>
            </div>
            {created.has_password && (
              <p className="text-sm text-slate-500">
                提取码：<span className="font-mono font-semibold text-slate-800">{password}</span>
              </p>
            )}
            <Button className="w-full" onClick={() => setCreated(null)}>
              再创建一个
            </Button>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>提取码（可选）</Label>
              <Input
                placeholder="留空则无需提取码"
                value={password}
                maxLength={12}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label>有效期</Label>
              <Select value={expireDays} onValueChange={setExpireDays}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">1 天</SelectItem>
                  <SelectItem value="7">7 天</SelectItem>
                  <SelectItem value="30">30 天</SelectItem>
                  <SelectItem value="0">永久有效</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <Button className="w-full" onClick={handleCreate} disabled={creating}>
              {creating && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
              创建分享链接
            </Button>

            {existing.length > 0 && (
              <>
                <Separator />
                <div className="space-y-2">
                  <p className="text-xs font-medium text-slate-500">该文件的已有分享</p>
                  {existing.map((s) => (
                    <div key={s.id} className="flex items-center gap-2 text-xs text-slate-600">
                      <span className="truncate flex-1 font-mono">{shareLink(s.token)}</span>
                      <span className="shrink-0 text-slate-400">
                        {s.expires_at ? `${formatTime(s.expires_at)} 过期` : '永久'}
                      </span>
                      <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => copy(shareLink(s.token))}>
                        <Copy className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 text-red-500"
                        onClick={() => cancelShare(s.id)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  ))}
                </div>
              </>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

export function shareFileSize(s: Share) {
  return s.file_size ? formatSize(s.file_size) : ''
}
