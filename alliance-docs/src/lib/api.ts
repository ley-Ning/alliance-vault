import { clearStoredSession, getStoredSession, refreshSession, setStoredSession } from './auth'
import { getActiveApiBase, requestApi } from './apiBase'
import type { Attachment, DocumentVersion, TeamDocument } from '../types'

interface PresignPayload {
  documentId: string
  fileName: string
  contentType: string
  sizeBytes: number
}

interface PresignResult {
  objectKey: string
  uploadUrl: string
  method: 'PUT'
  requiredHeaders: Record<string, string>
}

interface CompletePayload {
  documentId: string
  objectKey: string
  fileName: string
  contentType: string
  sizeBytes: number
  owner: string
}

interface DownloadURLResult {
  downloadUrl: string
  expiresInSeconds: number
  attachment: Attachment
}

interface DeleteAttachmentResult {
  deleted: boolean
  attachmentId: string
}

export interface AdminUserItem {
  id: string
  username: string
  displayName: string
  isAdmin: boolean
  isDisabled: boolean
  mustChangePassword: boolean
  disabledAt?: string
  createdAt: string
}

export type DocumentPermissionAccess = 'read' | 'edit'

export interface DocumentPermissionItem {
  documentId: string
  userId: string
  username: string
  displayName: string
  isAdmin: boolean
  isDisabled: boolean
  accessLevel: DocumentPermissionAccess
  createdAt: string
  updatedAt: string
}

interface CreateDocumentPayload {
  title?: string
  content?: string
  tags?: string[]
  status?: TeamDocument['status']
  owner?: string
}

type UpdateDocumentPayload = Partial<Pick<TeamDocument, 'title' | 'content' | 'tags' | 'status' | 'owner'>>

let refreshingPromise: Promise<string | null> | null = null

const resolveToken = async (): Promise<string | null> => {
  const session = getStoredSession()
  if (!session) {
    return null
  }

  if (!refreshingPromise) {
    refreshingPromise = refreshSession(session.refreshToken)
      .then((nextSession) => {
        setStoredSession(nextSession)
        return nextSession.accessToken
      })
      .catch(() => {
        clearStoredSession()
        return null
      })
      .finally(() => {
        refreshingPromise = null
      })
  }

  return refreshingPromise
}

const requestJSON = async <T>(path: string, init?: RequestInit): Promise<T> => {
  const request = async (token: string | null) => {
    const headers = new Headers(init?.headers)
    if (!headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json')
    }
    if (token) {
      headers.set('Authorization', `Bearer ${token}`)
    }

    return await requestApi(path, {
      ...init,
      headers,
    })
  }

  const session = getStoredSession()
  let response = await request(session?.accessToken ?? null)

  if (response.status === 401 && session?.refreshToken) {
    const renewedToken = await resolveToken()
    if (renewedToken) {
      response = await request(renewedToken)
    }
  }

  if (!response.ok) {
    let message = `请求失败（${response.status}）`
    try {
      const payload = (await response.json()) as { error?: string }
      if (payload?.error) {
        message = payload.error
      }
    } catch {
      // ignore
    }
    throw new Error(message)
  }

  return (await response.json()) as T
}

export const getApiBaseURL = () => getActiveApiBase()

export const listDocuments = async (): Promise<TeamDocument[]> => {
  const result = await requestJSON<{ items: TeamDocument[] }>('/api/v1/documents')
  return result.items ?? []
}

export const createDocumentRemote = async (payload: CreateDocumentPayload): Promise<TeamDocument> => {
  const result = await requestJSON<{ document: TeamDocument }>('/api/v1/documents', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
  return result.document
}

export const updateDocumentRemote = async (id: string, patch: UpdateDocumentPayload): Promise<TeamDocument> => {
  const result = await requestJSON<{ document: TeamDocument }>(`/api/v1/documents/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(patch),
  })
  return result.document
}

export const deleteDocumentRemote = async (id: string): Promise<void> => {
  await requestJSON<{ deleted: boolean }>(`/api/v1/documents/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

export const listRecycleBinDocuments = async (): Promise<TeamDocument[]> => {
  const result = await requestJSON<{ items: TeamDocument[] }>('/api/v1/documents/recycle-bin')
  return result.items ?? []
}

export const restoreDocumentRemote = async (id: string): Promise<TeamDocument> => {
  const result = await requestJSON<{ document: TeamDocument }>(`/api/v1/documents/${encodeURIComponent(id)}/restore`, {
    method: 'POST',
  })
  return result.document
}

export const listDocumentVersions = async (documentId: string, limit = 40): Promise<DocumentVersion[]> => {
  const result = await requestJSON<{ items: DocumentVersion[] }>(
    `/api/v1/documents/${encodeURIComponent(documentId)}/versions?limit=${encodeURIComponent(String(limit))}`,
  )
  return result.items ?? []
}

export const rollbackDocumentVersionRemote = async (
  documentId: string,
  versionId: string,
): Promise<TeamDocument> => {
  const result = await requestJSON<{ document: TeamDocument }>(
    `/api/v1/documents/${encodeURIComponent(documentId)}/versions/${encodeURIComponent(versionId)}/rollback`,
    {
      method: 'POST',
    },
  )
  return result.document
}

export const listAttachments = async (documentId: string): Promise<Attachment[]> => {
  const result = await requestJSON<{ items: Attachment[] }>(
    `/api/v1/documents/${encodeURIComponent(documentId)}/attachments`,
  )
  return result.items ?? []
}

export const presignUpload = async (payload: PresignPayload): Promise<PresignResult> => {
  return await requestJSON<PresignResult>('/api/v1/uploads/presign', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export const completeUpload = async (payload: CompletePayload): Promise<Attachment> => {
  const result = await requestJSON<{ attachment: Attachment }>('/api/v1/uploads/complete', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
  return result.attachment
}

export const getDownloadURL = async (attachmentId: string): Promise<DownloadURLResult> => {
  return await requestJSON<DownloadURLResult>(`/api/v1/attachments/${encodeURIComponent(attachmentId)}/download-url`)
}

export const deleteAttachment = async (attachmentId: string): Promise<void> => {
  await requestJSON<DeleteAttachmentResult>(`/api/v1/attachments/${encodeURIComponent(attachmentId)}`, {
    method: 'DELETE',
  })
}

export const listAdminUsers = async (): Promise<AdminUserItem[]> => {
  const result = await requestJSON<{ items: AdminUserItem[] }>('/api/v1/admin/users')
  return result.items ?? []
}

export const updateAdminRole = async (userId: string, isAdmin: boolean): Promise<AdminUserItem> => {
  const result = await requestJSON<{ user: AdminUserItem }>(`/api/v1/admin/users/${encodeURIComponent(userId)}/role`, {
    method: 'PATCH',
    body: JSON.stringify({ isAdmin }),
  })
  return result.user
}

export const updateAdminUserDisabled = async (userId: string, isDisabled: boolean): Promise<AdminUserItem> => {
  const result = await requestJSON<{ user: AdminUserItem }>(`/api/v1/admin/users/${encodeURIComponent(userId)}/disabled`, {
    method: 'PATCH',
    body: JSON.stringify({ isDisabled }),
  })
  return result.user
}

export const deleteAdminUser = async (userId: string): Promise<void> => {
  await requestJSON<{ deleted: boolean }>(`/api/v1/admin/users/${encodeURIComponent(userId)}`, {
    method: 'DELETE',
  })
}

export const listDocumentPermissions = async (documentId: string): Promise<DocumentPermissionItem[]> => {
  const result = await requestJSON<{ items: DocumentPermissionItem[] }>(
    `/api/v1/admin/documents/${encodeURIComponent(documentId)}/permissions`,
  )
  return result.items ?? []
}

export const upsertDocumentPermission = async (
  documentId: string,
  userId: string,
  accessLevel: DocumentPermissionAccess,
): Promise<DocumentPermissionItem> => {
  const result = await requestJSON<{ permission: DocumentPermissionItem }>(
    `/api/v1/admin/documents/${encodeURIComponent(documentId)}/permissions`,
    {
      method: 'PUT',
      body: JSON.stringify({ userId, accessLevel }),
    },
  )
  return result.permission
}

export const deleteDocumentPermission = async (documentId: string, userId: string): Promise<void> => {
  await requestJSON<{ deleted: boolean }>(
    `/api/v1/admin/documents/${encodeURIComponent(documentId)}/permissions/${encodeURIComponent(userId)}`,
    {
      method: 'DELETE',
    },
  )
}
