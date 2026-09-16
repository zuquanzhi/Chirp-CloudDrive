import { useEffect, useState } from 'react'
import { Download, Loader2 } from 'lucide-react'
import { downloadFile, getToken } from '@/lib/api'
import { formatSize } from '@/lib/format'
import type { DriveFile } from '@/types'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { toast } from 'sonner'

type PreviewKind = 'image' | 'pdf' | 'video' | 'audio' | 'text' | 'other'

function kindOf(name: string): PreviewKind {
  const ext = name.split('.').pop()?.toLowerCase() ?? ''
  if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico'].includes(ext)) return 'image'
  if (ext === 'pdf') return 'pdf'
  if (['mp4', 'webm', 'ogg', 'mov'].includes(ext)) return 'video'
  if (['mp3', 'wav', 'flac', 'm4a', 'aac'].includes(ext)) return 'audio'
  if (['txt', 'md', 'markdown', 'json', 'js', 'ts', 'tsx', 'jsx', 'css', 'html', 'xml', 'yml', 'yaml', 'toml', 'ini', 'log', 'csv', 'py', 'go', 'java', 'c', 'cpp', 'h', 'sh', 'sql'].includes(ext)) return 'text'
  return 'other'
}

export default function PreviewDialog({ file, onClose }: { file: DriveFile | null; onClose: () => void }) {
  const [url, setUrl] = useState<string | null>(null)
  const [text, setText] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const kind = file ? kindOf(file.original_name) : 'other'

  useEffect(() => {
    if (!file) return
    let objectUrl: string | null = null
    let cancelled = false
    setLoading(true)
    setUrl(null)
    setText(null)

    const run = async () => {
      try {
        const headers: Record<string, string> = {}
        const token = getToken()
        if (token) headers['Authorization'] = `Bearer ${token}`
        const resp = await fetch(`/api/drive/files/${file.id}/download?inline=1`, { headers })
        if (!resp.ok) throw new Error(await resp.text())
        if (kindOf(file.original_name) === 'text') {
          const t = await resp.text()
          if (!cancelled) setText(t.length > 200_000 ? t.slice(0, 200_000) + '\n\n…（内容过长，已截断）' : t)
        } else {
          const blob = await resp.blob()
          objectUrl = URL.createObjectURL(blob)
          if (!cancelled) setUrl(objectUrl)
        }
      } catch {
        if (!cancelled) toast.error('预览加载失败')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    run()
    return () => {
      cancelled = true
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [file])

  return (
    <Dialog open={file !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-4xl max-h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle className="truncate pr-8">{file?.original_name}</DialogTitle>
          <DialogDescription>{file ? formatSize(file.size) : ''}</DialogDescription>
        </DialogHeader>
        <div className="flex-1 min-h-0 overflow-auto rounded-md border bg-slate-50 flex items-center justify-center">
          {loading && (
            <div className="flex items-center gap-2 text-slate-400 py-20">
              <Loader2 className="h-5 w-5 animate-spin" />
              加载预览…
            </div>
          )}
          {!loading && kind === 'image' && url && (
            <img src={url} alt={file?.original_name} className="max-w-full max-h-[60vh] object-contain" />
          )}
          {!loading && kind === 'pdf' && url && (
            <iframe src={url} title="PDF 预览" className="w-full h-[60vh]" />
          )}
          {!loading && kind === 'video' && url && (
            <video src={url} controls className="max-w-full max-h-[60vh]" />
          )}
          {!loading && kind === 'audio' && url && (
            <audio src={url} controls className="w-full px-8" />
          )}
          {!loading && kind === 'text' && text !== null && (
            <pre className="w-full h-[60vh] overflow-auto whitespace-pre-wrap break-all p-4 text-xs font-mono text-slate-700">
              {text}
            </pre>
          )}
          {!loading && kind === 'other' && (
            <div className="py-16 text-center text-slate-400 space-y-3">
              <p>该文件类型暂不支持在线预览</p>
              {file && (
                <Button variant="outline" onClick={() => downloadFile(file.id, file.original_name)}>
                  <Download className="h-4 w-4 mr-2" />
                  下载查看
                </Button>
              )}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
