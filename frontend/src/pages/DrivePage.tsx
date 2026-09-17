import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useOutletContext } from 'react-router'
import {
  ArrowUpDown,
  ChevronRight,
  Download,
  Eye,
  File as FileIcon,
  Folder as FolderIcon,
  FolderInput,
  FolderPlus,
  History,
  Home,
  Link2,
  Loader2,
  MoreHorizontal,
  Pencil,
  RefreshCw,
  Search,
  Trash2,
  Upload,
  UploadCloud,
  X,
  Zap,
} from 'lucide-react'
import { toast } from 'sonner'
import {
  batchDelete,
  batchDownload,
  batchMove,
  createFolder,
  deleteFile,
  deleteFolder,
  downloadFile,
  listItems,
  moveFile,
  moveFolder,
  renameFile,
  renameFolder,
} from '@/lib/api'
import { smartUpload, type UploadProgress } from '@/lib/upload'
import { formatSize, formatTime } from '@/lib/format'
import type { DriveFile, Folder } from '@/types'
import PreviewDialog from '@/components/PreviewDialog'
import ShareDialog from '@/components/ShareDialog'
import VersionsDialog from '@/components/VersionsDialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { cn } from '@/lib/utils'

interface Crumb {
  id: number | null
  name: string
}

type ItemRef =
  | { kind: 'folder'; item: Folder }
  | { kind: 'file'; item: DriveFile }

type SortKey = 'name' | 'size' | 'time'

const DND_TYPE = 'application/x-chirp-item'

interface UploadTask {
  name: string
  progress: UploadProgress
  instant?: boolean
  error?: string
}

const phaseLabel: Record<UploadProgress['phase'], string> = {
  hashing: '计算特征码…',
  instant: '秒传…',
  uploading: '上传中…',
  merging: '合并分片…',
  done: '完成',
}

export default function DrivePage() {
  const { refreshQuota } = useOutletContext<{ refreshQuota: () => void }>()
  const [crumbs, setCrumbs] = useState<Crumb[]>([{ id: null, name: '全部文件' }])
  const [folders, setFolders] = useState<Folder[]>([])
  const [files, setFiles] = useState<DriveFile[]>([])
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(false)

  const [sortKey, setSortKey] = useState<SortKey>('name')
  const [sortAsc, setSortAsc] = useState(true)

  const [newFolderOpen, setNewFolderOpen] = useState(false)
  const [newFolderName, setNewFolderName] = useState('')

  const [renameTarget, setRenameTarget] = useState<ItemRef | null>(null)
  const [renameValue, setRenameValue] = useState('')

  const [moveTarget, setMoveTarget] = useState<ItemRef | null>(null)
  const [batchMoveOpen, setBatchMoveOpen] = useState(false)

  const [previewFile, setPreviewFile] = useState<DriveFile | null>(null)
  const [shareFile, setShareFile] = useState<DriveFile | null>(null)
  const [versionsFile, setVersionsFile] = useState<DriveFile | null>(null)

  // Selection state: "folder:<id>" / "file:<id>"
  const [selected, setSelected] = useState<Set<string>>(new Set())

  // Upload tasks (smart upload pipeline)
  const [tasks, setTasks] = useState<UploadTask[]>([])

  // DnD state
  const [dropFolderId, setDropFolderId] = useState<number | null>(null)
  const [dropCrumbIdx, setDropCrumbIdx] = useState<number | null>(null)
  const [dragFilesOver, setDragFilesOver] = useState(false)
  const dragDepth = useRef(0)

  const fileInputRef = useRef<HTMLInputElement>(null)

  const currentFolderId = crumbs[crumbs.length - 1].id

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const data = await listItems(currentFolderId, search || undefined)
      setFolders(data.folders)
      setFiles(data.files)
    } catch {
      toast.error('加载失败')
    } finally {
      setLoading(false)
    }
  }, [currentFolderId, search])

  useEffect(() => {
    const timer = setTimeout(refresh, search ? 300 : 0)
    return () => clearTimeout(timer)
  }, [refresh, search])

  // Clear selection when navigating
  useEffect(() => {
    setSelected(new Set())
  }, [currentFolderId])

  const sortedFolders = useMemo(
    () => [...folders].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN')),
    [folders],
  )
  const sortedFiles = useMemo(() => {
    const list = [...files]
    list.sort((a, b) => {
      let cmp = 0
      if (sortKey === 'name') cmp = a.original_name.localeCompare(b.original_name, 'zh-CN')
      else if (sortKey === 'size') cmp = a.size - b.size
      else cmp = new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
      return sortAsc ? cmp : -cmp
    })
    return list
  }, [files, sortKey, sortAsc])

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) setSortAsc(!sortAsc)
    else {
      setSortKey(key)
      setSortAsc(true)
    }
  }

  const enterFolder = (folder: Folder) => {
    setCrumbs([...crumbs, { id: folder.id, name: folder.name }])
    setSearch('')
  }

  const jumpTo = (index: number) => {
    setCrumbs(crumbs.slice(0, index + 1))
    setSearch('')
  }

  const handleCreateFolder = async () => {
    if (!newFolderName.trim()) return
    try {
      await createFolder(newFolderName.trim(), currentFolderId)
      toast.success('文件夹已创建')
      setNewFolderOpen(false)
      setNewFolderName('')
      refresh()
    } catch {
      toast.error('创建失败')
    }
  }

  // ---- Smart upload (instant / chunked / classic) ----

  const updateTask = (name: string, patch: Partial<UploadTask>) => {
    setTasks((cur) => cur.map((t) => (t.name === name ? { ...t, ...patch } : t)))
  }

  const uploadFiles = useCallback(
    async (list: FileList | File[]) => {
      const arr = Array.from(list)
      if (arr.length === 0) return
      setTasks((cur) => [
        ...cur,
        ...arr.map((f) => ({ name: f.name, progress: { phase: 'hashing', percent: 0 } as UploadProgress })),
      ])
      let ok = 0
      for (const file of arr) {
        try {
          const result = await smartUpload(file, currentFolderId, (p) => updateTask(file.name, { progress: p }))
          ok++
          updateTask(file.name, { instant: result.instant })
          if (result.instant) toast.success(`「${file.name}」秒传成功`, { icon: <Zap className="h-4 w-4" /> })
        } catch (err) {
          const msg = err instanceof Error && err.message.includes('quota') ? '存储空间不足' : '上传失败'
          updateTask(file.name, { error: msg })
          toast.error(`「${file.name}」${msg}`)
        }
      }
      if (ok > 0) {
        refresh()
        refreshQuota()
      }
      // Auto-clear finished tasks
      setTimeout(() => setTasks((cur) => cur.filter((t) => t.progress.phase !== 'done' && !t.error)), 4000)
    },
    [currentFolderId, refresh, refreshQuota],
  )

  const handleUploadInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    // Copy the FileList before clearing the input. Some browsers invalidate the
    // live FileList when the input value is reset for the next selection.
    const files = e.target.files ? Array.from(e.target.files) : []
    e.target.value = ''
    if (files.length > 0) uploadFiles(files)
  }

  const handleRename = async () => {
    if (!renameTarget || !renameValue.trim()) return
    try {
      if (renameTarget.kind === 'folder') {
        await renameFolder(renameTarget.item.id, renameValue.trim())
      } else {
        await renameFile(renameTarget.item.id, renameValue.trim())
      }
      toast.success('已重命名')
      setRenameTarget(null)
      refresh()
    } catch {
      toast.error('重命名失败')
    }
  }

  const handleDelete = async (target: ItemRef) => {
    try {
      if (target.kind === 'folder') {
        await deleteFolder(target.item.id)
      } else {
        await deleteFile(target.item.id)
      }
      toast.success('已移入回收站')
      refresh()
    } catch {
      toast.error('删除失败')
    }
  }

  const handleDownload = async (file: DriveFile) => {
    try {
      await downloadFile(file.id, file.original_name)
    } catch {
      toast.error('下载失败')
    }
  }

  // ---- Selection & batch operations ----

  const keyOf = (ref: ItemRef) => `${ref.kind}:${ref.item.id}`
  const selectedFiles = sortedFiles.filter((f) => selected.has(`file:${f.id}`))
  const selectedFolders = sortedFolders.filter((f) => selected.has(`folder:${f.id}`))
  const allSelected = sortedFolders.length + sortedFiles.length > 0 &&
    selected.size === sortedFolders.length + sortedFiles.length

  const toggleOne = (ref: ItemRef, checked: boolean) => {
    setSelected((cur) => {
      const next = new Set(cur)
      if (checked) next.add(keyOf(ref))
      else next.delete(keyOf(ref))
      return next
    })
  }

  const toggleAll = (checked: boolean) => {
    if (!checked) {
      setSelected(new Set())
      return
    }
    setSelected(new Set([
      ...sortedFolders.map((f) => `folder:${f.id}`),
      ...sortedFiles.map((f) => `file:${f.id}`),
    ]))
  }

  const handleBatchDelete = async () => {
    try {
      const res = await batchDelete(
        selectedFiles.map((f) => f.id),
        selectedFolders.map((f) => f.id),
      )
      const failCount = Object.keys(res.failed ?? {}).length
      if (failCount > 0) toast.warning(`${res.deleted} 项已删除，${failCount} 项失败`)
      else toast.success(`${res.deleted} 项已移入回收站`)
      setSelected(new Set())
      refresh()
    } catch {
      toast.error('批量删除失败')
    }
  }

  const handleBatchDownload = async () => {
    if (selectedFiles.length === 0) {
      toast.info('请先勾选要下载的文件')
      return
    }
    try {
      await batchDownload(selectedFiles.map((f) => f.id))
      toast.success('打包下载已开始')
    } catch {
      toast.error('打包下载失败')
    }
  }

  const handleBatchMove = async (targetFolderId: number | null) => {
    try {
      const res = await batchMove(selectedFiles.map((f) => f.id), targetFolderId)
      const failCount = Object.keys(res.failed ?? {}).length
      if (failCount > 0) toast.warning(`${res.moved} 个文件已移动，${failCount} 个失败`)
      else toast.success(`${res.moved} 个文件已移动`)
      setSelected(new Set())
      setBatchMoveOpen(false)
      refresh()
    } catch {
      toast.error('批量移动失败')
    }
  }

  // ---- Internal drag & drop (move) ----

  const doMove = useCallback(
    async (ref: ItemRef, targetFolderId: number | null) => {
      if (ref.kind === 'folder' && ref.item.id === targetFolderId) return
      try {
        if (ref.kind === 'folder') {
          await moveFolder(ref.item.id, targetFolderId)
        } else {
          await moveFile(ref.item.id, targetFolderId)
        }
        toast.success('已移动')
        refresh()
      } catch (err) {
        toast.error(err instanceof Error ? err.message : '移动失败')
      }
    },
    [refresh],
  )

  const onItemDragStart = (e: React.DragEvent, ref: ItemRef) => {
    e.dataTransfer.setData(DND_TYPE, JSON.stringify({ kind: ref.kind, id: ref.item.id }))
    e.dataTransfer.effectAllowed = 'move'
  }

  const readDragRef = (e: React.DragEvent): ItemRef | null => {
    const raw = e.dataTransfer.getData(DND_TYPE)
    if (!raw) return null
    try {
      const { kind, id } = JSON.parse(raw)
      const source = kind === 'folder' ? folders.find((f) => f.id === id) : files.find((f) => f.id === id)
      if (!source) return null
      return kind === 'folder' ? { kind, item: source as Folder } : { kind, item: source as DriveFile }
    } catch {
      return null
    }
  }

  const folderDropProps = (folder: Folder) => ({
    onDragOver: (e: React.DragEvent) => {
      if (e.dataTransfer.types.includes(DND_TYPE)) {
        e.preventDefault()
        e.dataTransfer.dropEffect = 'move'
        setDropFolderId(folder.id)
      }
    },
    onDragLeave: () => setDropFolderId((cur) => (cur === folder.id ? null : cur)),
    onDrop: (e: React.DragEvent) => {
      setDropFolderId(null)
      const ref = readDragRef(e)
      if (!ref) return
      e.preventDefault()
      if (ref.kind === 'folder' && ref.item.id === folder.id) return
      doMove(ref, folder.id)
    },
  })

  const crumbDropProps = (crumb: Crumb, index: number) => ({
    onDragOver: (e: React.DragEvent) => {
      if (e.dataTransfer.types.includes(DND_TYPE) && index !== crumbs.length - 1) {
        e.preventDefault()
        e.dataTransfer.dropEffect = 'move'
        setDropCrumbIdx(index)
      }
    },
    onDragLeave: () => setDropCrumbIdx((cur) => (cur === index ? null : cur)),
    onDrop: (e: React.DragEvent) => {
      setDropCrumbIdx(null)
      const ref = readDragRef(e)
      if (!ref) return
      e.preventDefault()
      doMove(ref, crumb.id)
    },
  })

  // ---- OS file drag & drop (upload) ----

  const onPageDragEnter = (e: React.DragEvent) => {
    if (!e.dataTransfer.types.includes('Files')) return
    e.preventDefault()
    dragDepth.current++
    setDragFilesOver(true)
  }
  const onPageDragLeave = (e: React.DragEvent) => {
    if (!e.dataTransfer.types.includes('Files')) return
    dragDepth.current--
    if (dragDepth.current <= 0) {
      dragDepth.current = 0
      setDragFilesOver(false)
    }
  }
  const onPageDrop = (e: React.DragEvent) => {
    if (!e.dataTransfer.types.includes('Files') || e.dataTransfer.types.includes(DND_TYPE)) return
    e.preventDefault()
    dragDepth.current = 0
    setDragFilesOver(false)
    uploadFiles(e.dataTransfer.files)
  }

  return (
    <div
      className="relative space-y-4"
      onDragEnter={onPageDragEnter}
      onDragOver={(e) => e.dataTransfer.types.includes('Files') && e.preventDefault()}
      onDragLeave={onPageDragLeave}
      onDrop={onPageDrop}
    >
      {/* Drag-over upload overlay */}
      {dragFilesOver && (
        <div className="pointer-events-none absolute inset-0 z-20 flex items-center justify-center rounded-lg border-2 border-dashed border-sky-400 bg-sky-50/80">
          <div className="flex flex-col items-center gap-2 text-sky-600">
            <UploadCloud className="h-12 w-12" />
            <p className="font-medium">松开鼠标，上传到当前目录</p>
          </div>
        </div>
      )}

      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-2">
        <Button onClick={() => fileInputRef.current?.click()}>
          <Upload className="h-4 w-4 mr-2" />
          上传文件
        </Button>
        <input ref={fileInputRef} type="file" multiple className="hidden" onChange={handleUploadInput} />
        <Button variant="outline" onClick={() => setNewFolderOpen(true)}>
          <FolderPlus className="h-4 w-4 mr-2" />
          新建文件夹
        </Button>
        <div className="relative ml-auto">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400" />
          <Input
            className="pl-9 w-64"
            placeholder="搜索当前目录文件…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <Button variant="ghost" size="icon" onClick={refresh} title="刷新">
          <RefreshCw className="h-4 w-4" />
        </Button>
      </div>

      {/* Batch action bar */}
      {selected.size > 0 && (
        <div className="flex items-center gap-3 rounded-lg border border-sky-200 bg-sky-50 px-4 py-2 text-sm">
          <span className="text-sky-700">已选 {selected.size} 项</span>
          <Button size="sm" variant="outline" onClick={handleBatchDownload} disabled={selectedFiles.length === 0}>
            <Download className="h-4 w-4 mr-1" />
            打包下载 ({selectedFiles.length})
          </Button>
          <Button size="sm" variant="outline" onClick={() => setBatchMoveOpen(true)} disabled={selectedFiles.length === 0}>
            <FolderInput className="h-4 w-4 mr-1" />
            移动到…
          </Button>
          <Button size="sm" variant="outline" className="text-red-600" onClick={handleBatchDelete}>
            <Trash2 className="h-4 w-4 mr-1" />
            删除
          </Button>
          <Button size="sm" variant="ghost" className="ml-auto" onClick={() => setSelected(new Set())}>
            <X className="h-4 w-4 mr-1" />
            取消选择
          </Button>
        </div>
      )}

      {/* Breadcrumb (also drop targets) */}
      <div className="flex items-center gap-1 text-sm text-slate-600">
        {crumbs.map((crumb, i) => (
          <span key={i} className="flex items-center gap-1">
            {i > 0 && <ChevronRight className="h-4 w-4 text-slate-400" />}
            <button
              {...crumbDropProps(crumb, i)}
              className={cn(
                'hover:text-sky-600 flex items-center gap-1 rounded px-1 py-0.5',
                i === crumbs.length - 1 && 'font-semibold text-slate-800',
                dropCrumbIdx === i && 'bg-sky-100 text-sky-700 outline outline-1 outline-sky-400',
              )}
              onClick={() => jumpTo(i)}
            >
              {i === 0 && <Home className="h-4 w-4" />}
              {crumb.name}
            </button>
          </span>
        ))}
      </div>

      {/* Listing */}
      <div className="rounded-lg border bg-white">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox checked={allSelected} onCheckedChange={(v) => toggleAll(!!v)} />
              </TableHead>
              <TableHead>
                <button className="flex items-center gap-1 hover:text-sky-600" onClick={() => toggleSort('name')}>
                  名称
                  <ArrowUpDown className={cn('h-3.5 w-3.5', sortKey === 'name' ? 'text-sky-600' : 'text-slate-300')} />
                </button>
              </TableHead>
              <TableHead className="w-32">
                <button className="flex items-center gap-1 hover:text-sky-600" onClick={() => toggleSort('size')}>
                  大小
                  <ArrowUpDown className={cn('h-3.5 w-3.5', sortKey === 'size' ? 'text-sky-600' : 'text-slate-300')} />
                </button>
              </TableHead>
              <TableHead className="w-48">
                <button className="flex items-center gap-1 hover:text-sky-600" onClick={() => toggleSort('time')}>
                  修改时间
                  <ArrowUpDown className={cn('h-3.5 w-3.5', sortKey === 'time' ? 'text-sky-600' : 'text-slate-300')} />
                </button>
              </TableHead>
              <TableHead className="w-16" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {sortedFolders.length === 0 && sortedFiles.length === 0 && !loading && (
              <TableRow>
                <TableCell colSpan={5} className="text-center py-12 text-slate-400">
                  这里空空如也，点击上传或直接把文件拖进来
                </TableCell>
              </TableRow>
            )}
            {sortedFolders.map((folder) => (
              <TableRow
                key={`f-${folder.id}`}
                className={cn(
                  'cursor-pointer',
                  dropFolderId === folder.id && 'bg-sky-50 outline outline-1 outline-sky-400',
                  selected.has(`folder:${folder.id}`) && 'bg-sky-50/60',
                )}
                draggable
                onDragStart={(e) => onItemDragStart(e, { kind: 'folder', item: folder })}
                {...folderDropProps(folder)}
                onDoubleClick={() => enterFolder(folder)}
              >
                <TableCell onClick={(e) => e.stopPropagation()}>
                  <Checkbox
                    checked={selected.has(`folder:${folder.id}`)}
                    onCheckedChange={(v) => toggleOne({ kind: 'folder', item: folder }, !!v)}
                  />
                </TableCell>
                <TableCell>
                  <button className="flex items-center gap-2 hover:text-sky-600" onClick={() => enterFolder(folder)}>
                    <FolderIcon className="h-5 w-5 text-amber-500" />
                    <span className="font-medium">{folder.name}</span>
                  </button>
                </TableCell>
                <TableCell className="text-slate-500">—</TableCell>
                <TableCell className="text-slate-500">{formatTime(folder.created_at)}</TableCell>
                <TableCell>
                  <RowActions
                    onRename={() => {
                      setRenameTarget({ kind: 'folder', item: folder })
                      setRenameValue(folder.name)
                    }}
                    onMove={() => setMoveTarget({ kind: 'folder', item: folder })}
                    onDelete={() => handleDelete({ kind: 'folder', item: folder })}
                  />
                </TableCell>
              </TableRow>
            ))}
            {sortedFiles.map((file) => (
              <TableRow
                key={`file-${file.id}`}
                className={cn(selected.has(`file:${file.id}`) && 'bg-sky-50/60')}
                draggable
                onDragStart={(e) => onItemDragStart(e, { kind: 'file', item: file })}
              >
                <TableCell>
                  <Checkbox
                    checked={selected.has(`file:${file.id}`)}
                    onCheckedChange={(v) => toggleOne({ kind: 'file', item: file }, !!v)}
                  />
                </TableCell>
                <TableCell>
                  <button
                    className="flex items-center gap-2 hover:text-sky-600 text-left"
                    onClick={() => setPreviewFile(file)}
                    title="点击预览"
                  >
                    <FileIcon className="h-5 w-5 text-sky-500 shrink-0" />
                    <span>{file.original_name}</span>
                    {file.version > 1 && (
                      <span className="text-[10px] rounded bg-violet-100 text-violet-600 px-1 py-0.5">v{file.version}</span>
                    )}
                  </button>
                </TableCell>
                <TableCell className="text-slate-500">{formatSize(file.size)}</TableCell>
                <TableCell className="text-slate-500">{formatTime(file.created_at)}</TableCell>
                <TableCell>
                  <RowActions
                    onPreview={() => setPreviewFile(file)}
                    onDownload={() => handleDownload(file)}
                    onShare={() => setShareFile(file)}
                    onVersions={() => setVersionsFile(file)}
                    onRename={() => {
                      setRenameTarget({ kind: 'file', item: file })
                      setRenameValue(file.original_name)
                    }}
                    onMove={() => setMoveTarget({ kind: 'file', item: file })}
                    onDelete={() => handleDelete({ kind: 'file', item: file })}
                  />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <p className="text-xs text-slate-400">
        提示：点击文件名可在线预览；重复内容秒传；大于 8MB 自动分片续传；同名上传自动保留历史版本。
      </p>

      {/* Upload tasks panel */}
      {tasks.length > 0 && (
        <div className="fixed bottom-4 right-4 z-30 w-80 rounded-lg border bg-white shadow-lg p-3 space-y-2">
          {tasks.map((t) => (
            <div key={t.name} className="space-y-1">
              <div className="flex items-center gap-2 text-xs">
                {t.error ? (
                  <X className="h-3.5 w-3.5 text-red-500 shrink-0" />
                ) : t.progress.phase === 'done' ? (
                  t.instant ? (
                    <Zap className="h-3.5 w-3.5 text-amber-500 shrink-0" />
                  ) : (
                    <Upload className="h-3.5 w-3.5 text-emerald-500 shrink-0" />
                  )
                ) : (
                  <Loader2 className="h-3.5 w-3.5 animate-spin text-sky-500 shrink-0" />
                )}
                <span className="truncate flex-1 text-slate-700">{t.name}</span>
                <span className={cn('shrink-0', t.error ? 'text-red-500' : 'text-slate-400')}>
                  {t.error ?? (t.instant && t.progress.phase === 'done' ? '秒传完成' : phaseLabel[t.progress.phase])}
                </span>
              </div>
              {!t.error && <Progress value={t.progress.percent} className="h-1" />}
            </div>
          ))}
        </div>
      )}

      {/* New folder dialog */}
      <Dialog open={newFolderOpen} onOpenChange={setNewFolderOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新建文件夹</DialogTitle>
            <DialogDescription>在当前目录下创建一个新文件夹</DialogDescription>
          </DialogHeader>
          <Input
            placeholder="文件夹名称"
            value={newFolderName}
            onChange={(e) => setNewFolderName(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleCreateFolder()}
            autoFocus
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setNewFolderOpen(false)}>取消</Button>
            <Button onClick={handleCreateFolder}>创建</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Rename dialog */}
      <Dialog open={renameTarget !== null} onOpenChange={(open) => !open && setRenameTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>重命名</DialogTitle>
            <DialogDescription>输入新的名称</DialogDescription>
          </DialogHeader>
          <Input
            value={renameValue}
            onChange={(e) => setRenameValue(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleRename()}
            autoFocus
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setRenameTarget(null)}>取消</Button>
            <Button onClick={handleRename}>确定</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Move dialogs (single + batch) */}
      <MoveDialog
        key={moveTarget ? `${moveTarget.kind}-${moveTarget.item.id}` : 'none'}
        open={moveTarget !== null}
        title="移动到…"
        description={
          moveTarget
            ? `选择「${moveTarget.kind === 'folder' ? (moveTarget.item as Folder).name : (moveTarget.item as DriveFile).original_name}」的目标位置`
            : ''
        }
        excludeFolderId={moveTarget?.kind === 'folder' ? moveTarget.item.id : undefined}
        currentFolderId={currentFolderId}
        onClose={() => setMoveTarget(null)}
        onConfirm={async (targetId) => {
          if (!moveTarget) return
          await doMove(moveTarget, targetId)
          setMoveTarget(null)
        }}
      />
      <MoveDialog
        open={batchMoveOpen}
        title="批量移动"
        description={`将选中的 ${selectedFiles.length} 个文件移动到…`}
        currentFolderId={currentFolderId}
        onClose={() => setBatchMoveOpen(false)}
        onConfirm={handleBatchMove}
      />

      {/* Preview / Share / Versions dialogs */}
      <PreviewDialog file={previewFile} onClose={() => setPreviewFile(null)} />
      <ShareDialog file={shareFile} onClose={() => setShareFile(null)} />
      <VersionsDialog
        file={versionsFile}
        onClose={() => setVersionsFile(null)}
        onRestored={() => {
          refresh()
          refreshQuota()
        }}
      />
    </div>
  )
}

function RowActions({
  onPreview,
  onDownload,
  onShare,
  onVersions,
  onRename,
  onMove,
  onDelete,
}: {
  onPreview?: () => void
  onDownload?: () => void
  onShare?: () => void
  onVersions?: () => void
  onRename: () => void
  onMove: () => void
  onDelete: () => void
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon">
          <MoreHorizontal className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {onPreview && (
          <DropdownMenuItem onClick={onPreview}>
            <Eye className="h-4 w-4 mr-2" />
            预览
          </DropdownMenuItem>
        )}
        {onDownload && (
          <DropdownMenuItem onClick={onDownload}>
            <Download className="h-4 w-4 mr-2" />
            下载
          </DropdownMenuItem>
        )}
        {onShare && (
          <DropdownMenuItem onClick={onShare}>
            <Link2 className="h-4 w-4 mr-2" />
            分享
          </DropdownMenuItem>
        )}
        {onVersions && (
          <DropdownMenuItem onClick={onVersions}>
            <History className="h-4 w-4 mr-2" />
            历史版本
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onClick={onRename}>
          <Pencil className="h-4 w-4 mr-2" />
          重命名
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onMove}>
          <FolderInput className="h-4 w-4 mr-2" />
          移动到…
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onDelete} className="text-red-600">
          <Trash2 className="h-4 w-4 mr-2" />
          删除
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function MoveDialog({
  open,
  title,
  description,
  excludeFolderId,
  currentFolderId,
  onClose,
  onConfirm,
}: {
  open: boolean
  title: string
  description: string
  excludeFolderId?: number
  currentFolderId: number | null
  onClose: () => void
  onConfirm: (targetFolderId: number | null) => Promise<void>
}) {
  const [crumbs, setCrumbs] = useState<Crumb[]>([{ id: null, name: '全部文件' }])
  const [folders, setFolders] = useState<Folder[]>([])
  const [busy, setBusy] = useState(false)
  const browseId = crumbs[crumbs.length - 1].id

  useEffect(() => {
    if (!open) return
    setCrumbs([{ id: null, name: '全部文件' }])
  }, [open])

  useEffect(() => {
    if (!open) return
    listItems(browseId).then((data) => setFolders(data.folders)).catch(() => setFolders([]))
  }, [browseId, open])

  const isCurrent = browseId === currentFolderId

  const confirm = async () => {
    setBusy(true)
    try {
      await onConfirm(browseId)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <div className="flex items-center gap-1 text-sm text-slate-600">
          {crumbs.map((crumb, i) => (
            <span key={i} className="flex items-center gap-1">
              {i > 0 && <ChevronRight className="h-4 w-4 text-slate-400" />}
              <button className="hover:text-sky-600" onClick={() => setCrumbs(crumbs.slice(0, i + 1))}>
                {crumb.name}
              </button>
            </span>
          ))}
        </div>
        <div className="max-h-56 overflow-auto rounded-md border divide-y">
          {folders.filter((f) => f.id !== excludeFolderId).length === 0 && (
            <div className="py-8 text-center text-sm text-slate-400">此目录下没有文件夹</div>
          )}
          {folders
            .filter((f) => f.id !== excludeFolderId)
            .map((folder) => (
              <button
                key={folder.id}
                className="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-slate-50"
                onClick={() => setCrumbs([...crumbs, { id: folder.id, name: folder.name }])}
              >
                <FolderIcon className="h-4 w-4 text-amber-500" />
                {folder.name}
              </button>
            ))}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button onClick={confirm} disabled={isCurrent || busy}>
            {busy && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
            移动到此处
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
