import { useEffect, useState } from 'react'
import { useParams } from 'react-router'
import { Cloud, Download, FileIcon, Loader2, Lock } from 'lucide-react'
import { getShareInfo, shareDownloadUrl } from '@/lib/api'
import { formatSize, formatTime } from '@/lib/format'
import type { Share } from '@/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

export default function SharePage() {
  const { token = '' } = useParams()
  const [share, setShare] = useState<Share | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [password, setPassword] = useState('')
  const [downloading, setDownloading] = useState(false)

  useEffect(() => {
    getShareInfo(token)
      .then(setShare)
      .catch((err) => setError(err instanceof Error ? err.message : '分享不存在'))
      .finally(() => setLoading(false))
  }, [token])

  const handleDownload = async () => {
    setDownloading(true)
    try {
      const url = shareDownloadUrl(token, password)
      const resp = await fetch(url)
      if (!resp.ok) {
        const text = await resp.text()
        throw new Error(text || '下载失败')
      }
      const blob = await resp.blob()
      const objectUrl = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = objectUrl
      a.download = share?.file_name ?? 'download'
      a.click()
      URL.revokeObjectURL(objectUrl)
    } catch (err) {
      setError(err instanceof Error && err.message.includes('extraction') ? '提取码错误' : '下载失败，请检查提取码')
    } finally {
      setDownloading(false)
    }
  }

  return (
    <div className="min-h-screen bg-slate-50 flex flex-col items-center justify-center p-6">
      <div className="flex items-center gap-2 mb-8">
        <Cloud className="h-7 w-7 text-sky-600" />
        <span className="font-bold text-xl text-slate-800">Chirp CloudDrive</span>
      </div>
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle>文件分享</CardTitle>
          <CardDescription>有人通过 Chirp 分享了一个文件给你</CardDescription>
        </CardHeader>
        <CardContent>
          {loading && (
            <div className="flex items-center justify-center gap-2 py-10 text-slate-400">
              <Loader2 className="h-5 w-5 animate-spin" />
              加载中…
            </div>
          )}
          {!loading && error && !share && (
            <div className="py-10 text-center text-slate-500">{error}</div>
          )}
          {!loading && share && (
            <div className="space-y-4">
              <div className="flex items-center gap-3 rounded-lg border bg-slate-50 p-4">
                <FileIcon className="h-10 w-10 text-sky-500 shrink-0" />
                <div className="min-w-0">
                  <p className="font-medium text-slate-800 truncate">{share.file_name}</p>
                  <p className="text-xs text-slate-400">
                    {formatSize(share.file_size ?? 0)}
                    {share.expires_at ? ` · ${formatTime(share.expires_at)} 过期` : ' · 永久有效'}
                    {` · 已被下载 ${share.downloads} 次`}
                  </p>
                </div>
              </div>
              {share.has_password && (
                <div className="flex items-center gap-2">
                  <Lock className="h-4 w-4 text-slate-400 shrink-0" />
                  <Input
                    placeholder="请输入提取码"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && handleDownload()}
                  />
                </div>
              )}
              {error && <p className="text-sm text-red-500">{error}</p>}
              <Button className="w-full" onClick={handleDownload} disabled={downloading}>
                {downloading ? <Loader2 className="h-4 w-4 mr-2 animate-spin" /> : <Download className="h-4 w-4 mr-2" />}
                下载文件
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
