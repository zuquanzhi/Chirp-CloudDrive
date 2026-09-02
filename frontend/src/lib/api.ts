import type { DriveFile, Folder, ItemsResponse, QuotaResponse, User } from '@/types'

const TOKEN_KEY = 'chirp_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = { ...(options.headers as Record<string, string>) }
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`
  if (options.body && typeof options.body === 'string') {
    headers['Content-Type'] = 'application/json'
  }

  const resp = await fetch(path, { ...options, headers })
  if (resp.status === 401) {
    clearToken()
    window.location.href = '/login'
    throw new Error('unauthorized')
  }
  if (resp.status === 204) {
    return undefined as T
  }
  const text = await resp.text()
  if (!resp.ok) {
    throw new Error(text || `request failed: ${resp.status}`)
  }
  return (text ? JSON.parse(text) : undefined) as T
}

// ---- Auth ----

export async function signup(name: string, email: string, password: string) {
  return request<{ id: number; email: string }>('/signup', {
    method: 'POST',
    body: JSON.stringify({ name, email, password }),
  })
}

export async function login(email: string, password: string) {
  return request<{ token: string }>('/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}

export async function getMe() {
  return request<User>('/api/me')
}

export async function updateMe(data: Partial<User>) {
  return request<User>('/api/me', { method: 'PATCH', body: JSON.stringify(data) })
}

// ---- Drive ----

export async function getQuota() {
  return request<QuotaResponse>('/api/drive/quota')
}

export async function listItems(folderId?: number | null, q?: string) {
  const params = new URLSearchParams()
  if (folderId != null) params.set('folder_id', String(folderId))
  if (q) params.set('q', q)
  const qs = params.toString()
  return request<ItemsResponse>(`/api/drive/items${qs ? `?${qs}` : ''}`)
}

export async function createFolder(name: string, parentId?: number | null) {
  return request<Folder>('/api/drive/folders', {
    method: 'POST',
    body: JSON.stringify({ name, parent_id: parentId ?? null }),
  })
}

export async function renameFolder(id: number, name: string) {
  return request<{ message: string }>(`/api/drive/folders/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ name }),
  })
}

export async function moveFolder(id: number, parentId: number | null) {
  return request<{ message: string }>(`/api/drive/folders/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ parent_id: parentId }),
  })
}

export async function deleteFolder(id: number) {
  return request<void>(`/api/drive/folders/${id}`, { method: 'DELETE' })
}

export async function uploadFile(file: File, folderId?: number | null) {
  const form = new FormData()
  if (folderId != null) form.set('folder_id', String(folderId))
  form.set('file', file)
  return request<DriveFile>('/api/drive/files', { method: 'POST', body: form })
}

export async function renameFile(id: number, name: string) {
  return request<{ message: string }>(`/api/drive/files/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ name }),
  })
}

export async function moveFile(id: number, folderId: number | null) {
  return request<{ message: string }>(`/api/drive/files/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ folder_id: folderId }),
  })
}

export async function deleteFile(id: number) {
  return request<void>(`/api/drive/files/${id}`, { method: 'DELETE' })
}

export function downloadUrl(id: number) {
  return `/api/drive/files/${id}/download`
}

// downloadFile fetches with the auth header and triggers a browser download.
export async function downloadFile(id: number, filename: string) {
  const headers: Record<string, string> = {}
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`
  const resp = await fetch(downloadUrl(id), { headers })
  if (!resp.ok) throw new Error(await resp.text() || 'download failed')
  const blob = await resp.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

// ---- Trash ----

export async function listTrash() {
  return request<ItemsResponse>('/api/drive/trash')
}

export async function restoreTrashItem(kind: 'folders' | 'files', id: number) {
  return request<{ message: string }>(`/api/drive/trash/${kind}/${id}/restore`, { method: 'POST' })
}

export async function hardDeleteTrashItem(kind: 'folders' | 'files', id: number) {
  return request<void>(`/api/drive/trash/${kind}/${id}`, { method: 'DELETE' })
}
