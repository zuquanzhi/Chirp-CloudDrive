import { useEffect, useState } from 'react'
import { Copy, Link2, RefreshCw, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { deleteShare, listShares } from '@/lib/api'
import { formatSize, formatTime } from '@/lib/format'
import type { Share } from '@/types'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'

export default function SharesPage() {
  const [shares, setShares] = useState<Share[]>([])
  const [loading, setLoading] = useState(false)

  const refresh = () => {
    setLoading(true)
    listShares()
      .then((data) => setShares(data.shares ?? []))
      .catch(() => toast.error('加载失败'))
      .finally(() => setLoading(false))
  }

  useEffect(refresh, [])

  const copy = (token: string) => {
    navigator.clipboard
      .writeText(`${window.location.origin}/share/${token}`)
      .then(() => toast.success('链接已复制'))
  }

  const cancel = async (id: number) => {
    try {
      await deleteShare(id)
      setShares((cur) => cur.filter((s) => s.id !== id))
      toast.success('分享已取消')
    } catch {
      toast.error('取消失败')
    }
  }

  const isExpired = (s: Share) => s.expires_at && new Date(s.expires_at).getTime() < Date.now()

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-slate-800 flex items-center gap-2">
          <Link2 className="h-5 w-5 text-sky-600" />
          我的分享
        </h1>
        <Button variant="ghost" size="icon" onClick={refresh} title="刷新">
          <RefreshCw className="h-4 w-4" />
        </Button>
      </div>
      <div className="rounded-lg border bg-white">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>文件</TableHead>
              <TableHead className="w-28">大小</TableHead>
              <TableHead className="w-24">状态</TableHead>
              <TableHead className="w-28">下载次数</TableHead>
              <TableHead className="w-48">过期时间</TableHead>
              <TableHead className="w-24" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {shares.length === 0 && !loading && (
              <TableRow>
                <TableCell colSpan={6} className="text-center py-12 text-slate-400">
                  还没有分享，去网盘里选一个文件创建分享链接吧
                </TableCell>
              </TableRow>
            )}
            {shares.map((s) => (
              <TableRow key={s.id}>
                <TableCell className="font-medium">{s.file_name || `#${s.resource_id}`}</TableCell>
                <TableCell className="text-slate-500">{s.file_size ? formatSize(s.file_size) : '—'}</TableCell>
                <TableCell>
                  {isExpired(s) ? (
                    <Badge variant="destructive">已过期</Badge>
                  ) : (
                    <Badge variant="secondary">{s.has_password ? '有提取码' : '公开'}</Badge>
                  )}
                </TableCell>
                <TableCell className="text-slate-500">{s.downloads}</TableCell>
                <TableCell className="text-slate-500">
                  {s.expires_at ? formatTime(s.expires_at) : '永久有效'}
                </TableCell>
                <TableCell>
                  <div className="flex gap-1">
                    <Button variant="ghost" size="icon" title="复制链接" onClick={() => copy(s.token)}>
                      <Copy className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      title="取消分享"
                      className="text-red-500"
                      onClick={() => cancel(s.id)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
