import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useOutletContext } from 'react-router'
import {
  ArrowUpDown,
  ChevronRight,
  Download,
  File as FileIcon,
  Folder as FolderIcon,
  FolderInput,
  FolderPlus,
  Home,
  MoreHorizontal,
  Pencil,
  RefreshCw,
  Search,
  Trash2,
  Upload,
  UploadCloud,
} from 'lucide-react'
import { toast } from 'sonner'
import {
  createFolder,
  deleteFile,
  deleteFolder,
  downloadFile,
  listItems,
  moveFile,
  moveFolder,
  renameFile,
  renameFolder,
  uploadFile,
} from '@/lib/api'
import { formatSize, formatTime } from '@/lib/format'
import type { DriveFile, Folder } from '@/types'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
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

  // DnD state
  const [dropFolderId, setDropFolderId] = useState<number | null>(null) // highlighted folder row
  const [dropCrumbIdx, setDropCrumbIdx] = useState<number | null>(null) // highlighted breadcrumb
  const [dragFilesOver, setDragFilesOver] = useState(false) // OS files dragged over the page
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

  const uploadFiles = useCallback(
    async (list: FileList | File[]) => {
      const arr = Array.from(list)
      if (arr.length === 0) return
      let ok = 0
      for (const file of arr) {
        try {
          await uploadFile(file, currentFolderId)
          ok++
        } catch (err) {
          toast.error(
            err instanceof Error && err.message.includes('quota')
              ? `「${file.name}」上传失败：存储空间不足`
              : `「${file.name}」上传失败`,
          )
        }
      }
      if (ok > 0) {
        toast.success(ok === arr.length ? `${ok} 个文件上传成功` : `${ok}/${arr.length} 个文件上传成功`)
        refresh()
        refreshQuota()
      }
    },
    [currentFolderId, refresh, refreshQuota],
  )

  const handleUploadInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    const list = e.target.files
    e.target.value = ''
    if (list) uploadFiles(list)
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

  // ---- Internal drag & drop (move) ----

  const doMove = useCallback(
    async (ref: ItemRef, targetFolderId: number | null) => {
      if (ref.kind === 'folder' && ref.item.id === targetFolderId) return
      if (ref.item.id === (ref.kind === 'folder' ? targetFolderId : -1)) return
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
                <TableCell colSpan={4} className="text-center py-12 text-slate-400">
                  这里空空如也，点击上传或直接把文件拖进来
                </TableCell>
              </TableRow>
            )}
            {sortedFolders.map((folder) => (
              <TableRow
                key={`f-${folder.id}`}
                className={cn('cursor-pointer', dropFolderId === folder.id && 'bg-sky-50 outline outline-1 outline-sky-400')}
                draggable
                onDragStart={(e) => onItemDragStart(e, { kind: 'folder', item: folder })}
                {...folderDropProps(folder)}
                onDoubleClick={() => enterFolder(folder)}
              >
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
                draggable
                onDragStart={(e) => onItemDragStart(e, { kind: 'file', item: file })}
              >
                <TableCell>
                  <div className="flex items-center gap-2">
                    <FileIcon className="h-5 w-5 text-sky-500" />
                    <span>{file.original_name}</span>
                  </div>
                </TableCell>
                <TableCell className="text-slate-500">{formatSize(file.size)}</TableCell>
                <TableCell className="text-slate-500">{formatTime(file.created_at)}</TableCell>
                <TableCell>
                  <RowActions
                    onDownload={() => handleDownload(file)}
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

      <p className="text-xs text-slate-400">提示：可以把文件/文件夹拖到某个文件夹上移动，也可以直接把电脑里的文件拖进页面上传。</p>

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

      {/* Move dialog (key forces fresh state for each target) */}
      <MoveDialog
        key={moveTarget ? `${moveTarget.kind}-${moveTarget.item.id}` : 'none'}
        target={moveTarget}
        currentFolderId={currentFolderId}
        onClose={() => setMoveTarget(null)}
        onMoved={() => {
          setMoveTarget(null)
          refresh()
        }}
      />
    </div>
  )
}

function RowActions({
  onDownload,
  onRename,
  onMove,
  onDelete,
}: {
  onDownload?: () => void
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
        {onDownload && (
          <DropdownMenuItem onClick={onDownload}>
            <Download className="h-4 w-4 mr-2" />
            下载
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
  target,
  currentFolderId,
  onClose,
  onMoved,
}: {
  target: ItemRef | null
  currentFolderId: number | null
  onClose: () => void
  onMoved: () => void
}) {
  const [crumbs, setCrumbs] = useState<Crumb[]>([{ id: null, name: '全部文件' }])
  const [folders, setFolders] = useState<Folder[]>([])
  const browseId = crumbs[crumbs.length - 1].id

  useEffect(() => {
    if (!target) return
    listItems(browseId).then((data) => setFolders(data.folders)).catch(() => setFolders([]))
  }, [browseId, target])

  if (!target) return null

  const excludedId = target.kind === 'folder' ? target.item.id : -1
  const isCurrent = browseId === currentFolderId

  const doMove = async () => {
    try {
      if (target.kind === 'folder') {
        await moveFolder(target.item.id, browseId)
      } else {
        await moveFile(target.item.id, browseId)
      }
      toast.success('已移动')
      onMoved()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '移动失败')
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>移动到…</DialogTitle>
          <DialogDescription>
            选择「{target.kind === 'folder' ? (target.item as Folder).name : (target.item as DriveFile).original_name}」的目标位置
          </DialogDescription>
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
          {folders.filter((f) => f.id !== excludedId).length === 0 && (
            <div className="py-8 text-center text-sm text-slate-400">此目录下没有文件夹</div>
          )}
          {folders
            .filter((f) => f.id !== excludedId)
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
          <Button onClick={doMove} disabled={isCurrent}>
            移动到此处
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
