import {
  abortUpload,
  completeUpload,
  getUploadSession,
  initUpload,
  instantUpload,
  uploadChunk,
  uploadFile,
} from '@/lib/api'
import type { DriveFile } from '@/types'

/** Files larger than this go through the chunked (resumable) pipeline. */
export const CHUNK_THRESHOLD = 8 * 1024 * 1024 // 8 MiB
/** Files larger than this skip the pre-upload hash (too expensive in one shot). */
const HASH_LIMIT = 512 * 1024 * 1024 // 512 MiB

const CHUNK_SIZE = 8 * 1024 * 1024

export interface UploadProgress {
  phase: 'hashing' | 'instant' | 'uploading' | 'merging' | 'done'
  /** 0–100, only meaningful while uploading */
  percent: number
}

export type ProgressFn = (p: UploadProgress) => void

/** SHA-256 (hex) of a Blob via WebCrypto. */
export async function sha256Hex(blob: Blob): Promise<string> {
  const buf = await blob.arrayBuffer()
  const digest = await crypto.subtle.digest('SHA-256', buf)
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}

/**
 * Smart upload pipeline:
 *  1. hash the file and try instant upload (秒传)
 *  2. large files → chunked resumable upload
 *  3. small files → classic multipart upload
 */
export async function smartUpload(
  file: File,
  folderId: number | null,
  onProgress: ProgressFn = () => {},
): Promise<{ file: DriveFile; instant: boolean }> {
  // Step 1: hash + instant attempt
  let hash = ''
  if (file.size <= HASH_LIMIT) {
    onProgress({ phase: 'hashing', percent: 0 })
    hash = await sha256Hex(file)
    try {
      const res = await instantUpload(file.name, hash, file.size, folderId)
      onProgress({ phase: 'done', percent: 100 })
      return { file: res, instant: true }
    } catch {
      // hash unknown on the server → continue with a real upload
    }
  }

  // Step 2: chunked upload for large files
  if (file.size > CHUNK_THRESHOLD) {
    const res = await chunkedUpload(file, folderId, hash, onProgress)
    return { file: res, instant: false }
  }

  // Step 3: classic multipart
  onProgress({ phase: 'uploading', percent: 0 })
  const res = await uploadFile(file, folderId)
  onProgress({ phase: 'done', percent: 100 })
  return { file: res, instant: false }
}

/**
 * Chunked upload with resume support: the server reports which chunk indexes it
 * already has, and only the missing ones are sent.
 */
export async function chunkedUpload(
  file: File,
  folderId: number | null,
  hash: string,
  onProgress: ProgressFn = () => {},
): Promise<DriveFile> {
  const init = await initUpload(file.name, file.size, folderId, hash, CHUNK_SIZE)
  if (init.instant && init.resource) {
    onProgress({ phase: 'done', percent: 100 })
    return init.resource
  }
  const session = init.session
  if (!session) throw new Error('upload session not created')

  try {
    // Resume: ask the server which chunks are already stored.
    const state = await getUploadSession(session.id)
    const done = new Set(state.uploaded_chunks)

    for (let i = 0; i < session.total_chunks; i++) {
      if (done.has(i)) continue
      const start = i * session.chunk_size
      const end = Math.min(file.size, start + session.chunk_size)
      await uploadChunk(session.id, i, file.slice(start, end))
      onProgress({
        phase: 'uploading',
        percent: Math.round(((i + 1) / session.total_chunks) * 100),
      })
    }

    onProgress({ phase: 'merging', percent: 100 })
    const res = await completeUpload(session.id)
    onProgress({ phase: 'done', percent: 100 })
    return res
  } catch (err) {
    // Keep the session for resume on transient network errors; abort on server rejections.
    if (err instanceof Error && /hash mismatch|size mismatch|quota/.test(err.message)) {
      await abortUpload(session.id).catch(() => {})
    }
    throw err
  }
}
