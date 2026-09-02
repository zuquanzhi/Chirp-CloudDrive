import { useCallback, useEffect, useState } from 'react'
import { useOutletContext } from 'react-router'
import { File as FileIcon, Folder as FolderIcon, RotateCcw, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { hardDeleteTrashItem, listTrash, restoreTrashItem } from '@/lib/api'
import { formatSize, formatTime } from '@/lib/format'
import type { DriveFile, Folder } from '@/types'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

interface TrashRow {
  kind: 'folders' | 'files'
  id: number
  name: string
  size: number | null
  deletedAt?: string
}

export default function TrashPage() {
  const { refreshQuota } = useOutletContext<{ refreshQuota: () => void }>()
  const [rows, setRows] = useState<TrashRow[]>([])
  const [confirmRow, setConfirmRow] = useState<TrashRow | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await listTrash()
      const folderRows: TrashRow[] = data.folders.map((f: Folder) => ({
        kind: 'folders',
        id: f.id,
        name: f.name,
        size: null,
        deletedAt: f.deleted_at,
      }))
      const fileRows: TrashRow[] = data.files.map((f: DriveFile) => ({
        kind: 'files',
        id: f.id,
        name: f.original_name,
        size: f.size,
        deletedAt: f.deleted_at,
      }))
      setRows([...folderRows, ...fileRows])
    } catch {
      toast.error('加载回收站失败')
    }
  }, [])

  useEffect(() => {
    const timer = setTimeout(refresh, 0)
    return () => clearTimeout(timer)
  }, [refresh])

  const handleRestore = async (row: TrashRow) => {
    try {
      await restoreTrashItem(row.kind, row.id)
      toast.success('已还原')
      refresh()
    } catch {
      toast.error('还原失败')
    }
  }

  const handleHardDelete = async () => {
    if (!confirmRow) return
    try {
      await hardDeleteTrashItem(confirmRow.kind, confirmRow.id)
      toast.success('已彻底删除')
      setConfirmRow(null)
      refresh()
      refreshQuota()
    } catch {
      toast.error('删除失败')
    }
  }

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-bold text-slate-800">回收站</h1>
        <p className="text-sm text-slate-500 mt-1">删除的内容会保留在这里，可以还原或彻底删除</p>
      </div>

      <div className="rounded-lg border bg-white">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>名称</TableHead>
              <TableHead className="w-32">大小</TableHead>
              <TableHead className="w-48">删除时间</TableHead>
              <TableHead className="w-48 text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="text-center py-12 text-slate-400">
                  回收站是空的
                </TableCell>
              </TableRow>
            )}
            {rows.map((row) => (
              <TableRow key={`${row.kind}-${row.id}`}>
                <TableCell>
                  <div className="flex items-center gap-2">
                    {row.kind === 'folders' ? (
                      <FolderIcon className="h-5 w-5 text-amber-500" />
                    ) : (
                      <FileIcon className="h-5 w-5 text-sky-500" />
                    )}
                    <span>{row.name}</span>
                  </div>
                </TableCell>
                <TableCell className="text-slate-500">{row.size == null ? '—' : formatSize(row.size)}</TableCell>
                <TableCell className="text-slate-500">{formatTime(row.deletedAt)}</TableCell>
                <TableCell className="text-right space-x-2">
                  <Button variant="outline" size="sm" onClick={() => handleRestore(row)}>
                    <RotateCcw className="h-4 w-4 mr-1" />
                    还原
                  </Button>
                  <Button variant="destructive" size="sm" onClick={() => setConfirmRow(row)}>
                    <Trash2 className="h-4 w-4 mr-1" />
                    彻底删除
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <AlertDialog open={confirmRow !== null} onOpenChange={(open) => !open && setConfirmRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认彻底删除？</AlertDialogTitle>
            <AlertDialogDescription>
              「{confirmRow?.name}」将被永久删除，无法恢复。
              {confirmRow?.kind === 'folders' && ' 文件夹内的所有内容也会一并删除。'}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={handleHardDelete} className="bg-red-600 hover:bg-red-700">
              彻底删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
