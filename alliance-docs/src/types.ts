export type DocStatus = '草稿' | '评审中' | '已发布'

export interface WorkspaceFolder {
  id: string
  name: string
  createdAt: string
}

export interface TeamDocument {
  id: string
  title: string
  content: string
  tags: string[]
  status: DocStatus
  owner: string
  canEdit?: boolean
  folderId: string | null
  createdAt: string
  updatedAt: string
}

export interface WorkspaceState {
  folders: WorkspaceFolder[]
  documents: TeamDocument[]
  recycleBin: TeamDocument[]
  activeDocumentId: string
  updatedAt: string
}

export interface DocumentVersion {
  id: string
  documentId: string
  version: number
  title: string
  content: string
  tags: string[]
  status: DocStatus
  owner: string
  event: string
  createdBy: string
  createdAt: string
}

export interface Attachment {
  id: string
  documentId: string
  objectKey: string
  fileName: string
  contentType: string
  sizeBytes: number
  owner: string
  storage: string
  createdAt: string
}

export interface AuthUser {
  id: string
  username: string
  displayName: string
  isAdmin: boolean
  isDisabled: boolean
  mustChangePassword: boolean
}

export interface AuthSession {
  user: AuthUser
  accessToken: string
  refreshToken: string
}
