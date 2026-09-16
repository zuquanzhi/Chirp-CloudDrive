export interface User {
  id: number
  name: string
  email: string
  role: string
  created_at: string
  phone_number?: string
  school?: string
  student_id?: string
  birthdate?: string
  address?: string
  gender?: string
  quota: number
  used: number
}

export interface Folder {
  id: number
  owner_id: number
  parent_id: number | null
  name: string
  created_at: string
  deleted_at?: string
}

export interface DriveFile {
  id: number
  owner_id: number | null
  folder_id: number | null
  title: string
  description: string
  filename: string
  original_name: string
  size: number
  file_hash: string
  status: string
  created_at: string
  deleted_at?: string
  url?: string
  version_group?: string
  version: number
  is_latest: boolean
}

export interface ItemsResponse {
  folders: Folder[]
  files: DriveFile[]
}

export interface QuotaResponse {
  quota: number
  used: number
}

export interface Share {
  id: number
  token: string
  resource_id: number
  owner_id: number
  has_password: boolean
  expires_at?: string
  downloads: number
  created_at: string
  file_name?: string
  file_size?: number
}

export interface Activity {
  id: number
  user_id: number
  action: string
  target_kind: 'file' | 'folder' | 'share'
  target_id: number
  target_name: string
  detail?: string
  created_at: string
}

export interface UploadSession {
  id: string
  owner_id: number
  folder_id: number | null
  filename: string
  size: number
  chunk_size: number
  total_chunks: number
  file_hash: string
  created_at: string
  uploaded_chunks: number[]
}

export interface InitUploadResult {
  instant: boolean
  resource?: DriveFile
  session?: UploadSession
}
