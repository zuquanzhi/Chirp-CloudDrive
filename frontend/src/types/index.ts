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
}

export interface ItemsResponse {
  folders: Folder[]
  files: DriveFile[]
}

export interface QuotaResponse {
  quota: number
  used: number
}
