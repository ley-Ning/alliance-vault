import { useEffect, useMemo, useRef, useState } from 'react'
import {
  createDocumentRemote,
  deleteDocumentRemote,
  listDocumentVersions as listDocumentVersionsRemote,
  listDocuments,
  listRecycleBinDocuments,
  restoreDocumentRemote,
  rollbackDocumentVersionRemote,
  updateDocumentRemote,
} from '../lib/api'
import { createEmptyDocument, loadWorkspace, saveWorkspace } from '../lib/storage'
import type { DocumentVersion, TeamDocument, WorkspaceFolder, WorkspaceState } from '../types'

const SYNC_CHANNEL = 'alliance-vault-sync'

type RemoteDocumentPatch = Partial<Pick<TeamDocument, 'title' | 'content' | 'tags' | 'status' | 'owner'>>

const uid = () => {
  try {
    return crypto.randomUUID()
  } catch {
    return `${Date.now()}-${Math.random().toString(36).slice(2)}`
  }
}

interface SyncPayload {
  sourceId: string
  state: WorkspaceState
}

const now = () => new Date().toISOString()

const updateWorkspaceStamp = (state: WorkspaceState): WorkspaceState => ({
  ...state,
  updatedAt: now(),
})

const normalizeFolderId = (folderId: unknown, validFolderIds: Set<string>): string | null => {
  return typeof folderId === 'string' && validFolderIds.has(folderId) ? folderId : null
}

const normalizeFolders = (folders: WorkspaceFolder[] | undefined): WorkspaceFolder[] => {
  if (!Array.isArray(folders)) {
    return []
  }

  return folders
    .filter((folder): folder is WorkspaceFolder => Boolean(folder?.id && folder.name))
    .map((folder) => ({
      ...folder,
      createdAt: folder.createdAt || now(),
    }))
}

const normalizeDocument = (
  doc: Partial<TeamDocument>,
  owner: string,
  validFolderIds: Set<string>,
  preferredFolderId?: string | null,
): TeamDocument => {
  const stamp = now()
  const folderId = preferredFolderId !== undefined ? preferredFolderId : normalizeFolderId(doc.folderId, validFolderIds)

  return {
    id: doc.id || uid(),
    title: doc.title || '未命名文档',
    content: doc.content || '<p></p>',
    tags: Array.isArray(doc.tags) && doc.tags.length > 0 ? doc.tags : ['未分类'],
    status: doc.status || '草稿',
    owner: doc.owner || owner,
    canEdit: typeof doc.canEdit === 'boolean' ? doc.canEdit : true,
    folderId,
    createdAt: doc.createdAt || stamp,
    updatedAt: doc.updatedAt || stamp,
  }
}

const normalizeWorkspace = (state: WorkspaceState, owner: string): WorkspaceState => {
  const folders = normalizeFolders(state.folders)
  const validFolderIds = new Set(folders.map((folder) => folder.id))
  const documents = (state.documents ?? []).map((doc) => normalizeDocument(doc, owner, validFolderIds))
  const recycleBin = (state.recycleBin ?? []).map((doc) => normalizeDocument(doc, owner, validFolderIds))

  if (documents.length === 0) {
    return {
      folders,
      documents: [],
      recycleBin,
      activeDocumentId: '',
      updatedAt: state.updatedAt || now(),
    }
  }

  const activeExists = documents.some((doc) => doc.id === state.activeDocumentId)

  return {
    folders,
    documents,
    recycleBin,
    activeDocumentId: activeExists ? state.activeDocumentId : documents[0].id,
    updatedAt: state.updatedAt || now(),
  }
}

const createWorkspaceFromDocuments = (
  docs: TeamDocument[],
  owner: string,
  previousState: WorkspaceState,
  currentActiveId?: string,
): WorkspaceState => {
  const folders = normalizeFolders(previousState.folders)
  const validFolderIds = new Set(folders.map((folder) => folder.id))
  const previousFolderByDocId = new Map(
    [...previousState.documents, ...(previousState.recycleBin ?? [])].map((doc) => [
      doc.id,
      normalizeFolderId(doc.folderId, validFolderIds),
    ]),
  )

  const documents = docs.map((doc) =>
    normalizeDocument(doc, owner, validFolderIds, previousFolderByDocId.get(doc.id)),
  )
  const recycleBin = (previousState.recycleBin ?? []).map((doc) =>
    normalizeDocument(doc, owner, validFolderIds, previousFolderByDocId.get(doc.id)),
  )

  return {
    folders,
    documents,
    recycleBin,
    activeDocumentId:
      documents.length === 0
        ? ''
        : currentActiveId && documents.some((doc) => doc.id === currentActiveId)
          ? currentActiveId
          : documents[0].id,
    updatedAt: now(),
  }
}

const mergePatch = (origin: RemoteDocumentPatch, patch: RemoteDocumentPatch): RemoteDocumentPatch => ({
  ...origin,
  ...patch,
})

const toRemotePatch = (patch: Partial<TeamDocument>): RemoteDocumentPatch => {
  const remotePatch: RemoteDocumentPatch = {}

  if (patch.title !== undefined) {
    remotePatch.title = patch.title
  }
  if (patch.content !== undefined) {
    remotePatch.content = patch.content
  }
  if (patch.tags !== undefined) {
    remotePatch.tags = patch.tags
  }
  if (patch.status !== undefined) {
    remotePatch.status = patch.status
  }
  if (patch.owner !== undefined) {
    remotePatch.owner = patch.owner
  }

  return remotePatch
}

const bindRecycleBinDocuments = (incoming: TeamDocument[], prev: WorkspaceState, owner: string) => {
  const validFolderIds = new Set(prev.folders.map((folder) => folder.id))
  const folderByDocId = new Map(
    [...prev.documents, ...prev.recycleBin].map((doc) => [doc.id, normalizeFolderId(doc.folderId, validFolderIds)]),
  )

  return incoming.map((doc) => normalizeDocument(doc, owner, validFolderIds, folderByDocId.get(doc.id) ?? null))
}

export const useWorkspace = (defaultOwner: string) => {
  const [workspace, setWorkspace] = useState<WorkspaceState>(() => loadWorkspace())
  const [syncMessage, setSyncMessage] = useState('正在连接云端...')

  const sourceIdRef = useRef(uid())
  const channelRef = useRef<BroadcastChannel | null>(null)
  const queueRef = useRef<Record<string, RemoteDocumentPatch>>({})
  const timerRef = useRef<Record<string, number>>({})
  const remoteReadyRef = useRef(false)

  useEffect(() => {
    let cancelled = false

    const bootstrap = async () => {
      setSyncMessage('正在拉取云端文档...')
      try {
        const [remoteDocs, remoteRecycleDocs] = await Promise.all([
          listDocuments(),
          listRecycleBinDocuments().catch(() => [] as TeamDocument[]),
        ])
        if (cancelled) {
          return
        }

        if (remoteDocs.length > 0) {
          remoteReadyRef.current = true
          setWorkspace((prev) => {
            const next = createWorkspaceFromDocuments(remoteDocs, defaultOwner, prev, prev.activeDocumentId)
            return updateWorkspaceStamp({
              ...next,
              recycleBin: bindRecycleBinDocuments(remoteRecycleDocs, next, defaultOwner),
            })
          })
          setSyncMessage('云端已同步')
          return
        }

        const localState = loadWorkspace()
        const localDocs = localState.documents
        if (localDocs.length > 0) {
          const created = await Promise.all(
            localDocs.map((doc) =>
              createDocumentRemote({
                title: doc.title,
                content: doc.content,
                tags: doc.tags,
                status: doc.status,
                owner: doc.owner,
              }),
            ),
          )

          if (cancelled) {
            return
          }

          remoteReadyRef.current = true

          const withFolderBinding = created.map((doc, index) => ({
            ...doc,
            folderId: localDocs[index]?.folderId ?? null,
          }))

          setWorkspace((prev) => {
            const next = createWorkspaceFromDocuments(withFolderBinding, defaultOwner, prev, prev.activeDocumentId)
            return updateWorkspaceStamp({
              ...next,
              recycleBin: bindRecycleBinDocuments(localState.recycleBin ?? [], next, defaultOwner),
            })
          })
          setSyncMessage('云端初始化完成')
          return
        }

        remoteReadyRef.current = true
        setWorkspace((prev) => {
          const next = createWorkspaceFromDocuments([], defaultOwner, loadWorkspace())
          return updateWorkspaceStamp({
            ...next,
            recycleBin: bindRecycleBinDocuments(remoteRecycleDocs, next, defaultOwner),
            activeDocumentId: prev.activeDocumentId && next.documents.some((doc) => doc.id === prev.activeDocumentId)
              ? prev.activeDocumentId
              : next.activeDocumentId,
          })
        })
        setSyncMessage('云端初始化完成')
      } catch {
        if (cancelled) {
          return
        }
        remoteReadyRef.current = false
        setSyncMessage('云端不可用，当前离线模式（已本地保存）')
      }
    }

    bootstrap()

    return () => {
      cancelled = true
    }
  }, [defaultOwner])

  useEffect(() => {
    saveWorkspace(workspace)
    channelRef.current?.postMessage({
      sourceId: sourceIdRef.current,
      state: workspace,
    } satisfies SyncPayload)
  }, [workspace])

  useEffect(() => {
    if (!('BroadcastChannel' in window)) {
      return
    }

    const channel = new BroadcastChannel(SYNC_CHANNEL)
    channelRef.current = channel

    channel.onmessage = (event: MessageEvent<SyncPayload>) => {
      const payload = event.data
      if (!payload || payload.sourceId === sourceIdRef.current) {
        return
      }
      setWorkspace(normalizeWorkspace(payload.state, defaultOwner))
    }

    return () => {
      channel.close()
      channelRef.current = null
    }
  }, [defaultOwner])

  const activeDocument = useMemo(
    () => workspace.documents.find((doc) => doc.id === workspace.activeDocumentId) ?? workspace.documents[0],
    [workspace],
  )

  const flushPatch = async (id: string) => {
    const patch = queueRef.current[id]
    if (!patch) {
      return
    }

    delete queueRef.current[id]
    const timer = timerRef.current[id]
    if (timer) {
      window.clearTimeout(timer)
      delete timerRef.current[id]
    }

    if (!remoteReadyRef.current) {
      return
    }

    setSyncMessage('同步文档中...')
    try {
      const updated = await updateDocumentRemote(id, patch)
      setWorkspace((prev) => {
        const validFolderIds = new Set(prev.folders.map((folder) => folder.id))
        const current = prev.documents.find((doc) => doc.id === id)
        const normalized = normalizeDocument(updated, defaultOwner, validFolderIds, current?.folderId ?? null)
        return updateWorkspaceStamp({
          ...prev,
          documents: prev.documents.map((doc) => (doc.id === id ? normalized : doc)),
        })
      })
      setSyncMessage('云端已同步')
    } catch {
      setSyncMessage('同步失败，当前变更已保存在本地')
    }
  }

  const schedulePatch = (id: string, patch: RemoteDocumentPatch) => {
    queueRef.current[id] = mergePatch(queueRef.current[id] ?? {}, patch)

    const existingTimer = timerRef.current[id]
    if (existingTimer) {
      window.clearTimeout(existingTimer)
    }

    timerRef.current[id] = window.setTimeout(() => {
      void flushPatch(id)
    }, 700)
  }

  const setActiveDocument = (id: string) => {
    setWorkspace((prev) => updateWorkspaceStamp({ ...prev, activeDocumentId: id }))
  }

  const createDocument = async () => {
    if (remoteReadyRef.current) {
      try {
        setSyncMessage('创建文档中...')
        const saved = await createDocumentRemote({
          title: '新建文档',
          content: '<p></p>',
          tags: ['未分类'],
          status: '草稿',
          owner: defaultOwner,
        })

        const nextDoc = {
          ...saved,
          folderId: null,
        }

        setWorkspace((prev) =>
          updateWorkspaceStamp({
            ...prev,
            documents: [nextDoc, ...prev.documents],
            activeDocumentId: nextDoc.id,
          }),
        )
        setSyncMessage('云端已同步')
        return
      } catch {
        setSyncMessage('云端不可用，创建本地文档')
      }
    }

    const newDoc = createEmptyDocument(defaultOwner)
    setWorkspace((prev) =>
      updateWorkspaceStamp({
        ...prev,
        documents: [newDoc, ...prev.documents],
        activeDocumentId: newDoc.id,
      }),
    )
  }

  const createFolder = (name: string) => {
    const trimmed = name.trim()
    if (!trimmed) {
      return
    }

    setWorkspace((prev) => {
      if (prev.folders.some((folder) => folder.name === trimmed)) {
        return prev
      }

      const folder: WorkspaceFolder = {
        id: uid(),
        name: trimmed,
        createdAt: now(),
      }

      return updateWorkspaceStamp({
        ...prev,
        folders: [folder, ...prev.folders],
      })
    })
  }

  const moveDocumentToFolder = (documentId: string, folderId: string | null) => {
    setWorkspace((prev) => {
      const exists = prev.documents.some((doc) => doc.id === documentId)
      if (!exists) {
        return prev
      }

      const validFolderId =
        typeof folderId === 'string' && prev.folders.some((folder) => folder.id === folderId) ? folderId : null

      return updateWorkspaceStamp({
        ...prev,
        documents: prev.documents.map((doc) =>
          doc.id === documentId
            ? {
                ...doc,
                folderId: validFolderId,
                updatedAt: now(),
              }
            : doc,
        ),
      })
    })
  }

  const removeDocument = async (id: string) => {
    const previousState = workspace
    const target = workspace.documents.find((doc) => doc.id === id)
    if (!target) {
      return
    }
    clearQueuedPatch(id)
    const filtered = workspace.documents.filter((doc) => doc.id !== id)
    const nextActive =
      filtered.length === 0 ? '' : workspace.activeDocumentId === id ? filtered[0].id : workspace.activeDocumentId

    setWorkspace(
      updateWorkspaceStamp({
        ...workspace,
        documents: filtered,
        recycleBin: [{ ...target, updatedAt: now() }, ...workspace.recycleBin.filter((doc) => doc.id !== id)],
        activeDocumentId: nextActive,
      }),
    )

    if (!remoteReadyRef.current) {
      setSyncMessage('本地已移动到回收站')
      return
    }

    try {
      setSyncMessage('移动到回收站中...')
      await deleteDocumentRemote(id)
      const deletedDocs = await listRecycleBinDocuments().catch(() => [] as TeamDocument[])
      setWorkspace((prev) => {
        return updateWorkspaceStamp({
          ...prev,
          recycleBin: bindRecycleBinDocuments(deletedDocs, prev, defaultOwner),
        })
      })
      setSyncMessage('已移入回收站')
    } catch {
      setWorkspace(previousState)
      setSyncMessage('删除失败，已回滚到本地状态')
    }
  }

  const refreshRecycleBin = async () => {
    if (!remoteReadyRef.current) {
      return
    }

    const deletedDocs = await listRecycleBinDocuments()
    setWorkspace((prev) => {
      return updateWorkspaceStamp({
        ...prev,
        recycleBin: bindRecycleBinDocuments(deletedDocs, prev, defaultOwner),
      })
    })
  }

  const restoreDocument = async (id: string) => {
    const previousState = workspace
    const recycleDoc = workspace.recycleBin.find((doc) => doc.id === id)
    if (!recycleDoc) {
      return
    }

    setWorkspace(
      updateWorkspaceStamp({
        ...workspace,
        documents: [recycleDoc, ...workspace.documents],
        recycleBin: workspace.recycleBin.filter((doc) => doc.id !== id),
        activeDocumentId: recycleDoc.id,
      }),
    )

    if (!remoteReadyRef.current) {
      setSyncMessage('本地已从回收站恢复')
      return
    }

    try {
      setSyncMessage('恢复文档中...')
      const restored = await restoreDocumentRemote(id)
      setWorkspace((prev) => {
        const validFolderIds = new Set(prev.folders.map((folder) => folder.id))
        const normalized = normalizeDocument(
          restored,
          defaultOwner,
          validFolderIds,
          recycleDoc.folderId ?? restored.folderId ?? null,
        )
        return updateWorkspaceStamp({
          ...prev,
          documents: [normalized, ...prev.documents.filter((doc) => doc.id !== id)],
          recycleBin: prev.recycleBin.filter((doc) => doc.id !== id),
          activeDocumentId: normalized.id,
        })
      })
      setSyncMessage('文档已恢复')
    } catch {
      setWorkspace(previousState)
      setSyncMessage('恢复失败，已回滚到本地状态')
    }
  }

  const listDocumentVersions = async (documentId: string, limit = 40): Promise<DocumentVersion[]> => {
    if (!remoteReadyRef.current) {
      throw new Error('离线模式下暂不支持历史版本，请连接云端后重试')
    }
    return await listDocumentVersionsRemote(documentId, limit)
  }

  const rollbackDocumentVersion = async (documentId: string, versionId: string) => {
    if (!remoteReadyRef.current) {
      throw new Error('离线模式下暂不支持回滚，请连接云端后重试')
    }

    setSyncMessage('回滚历史版本中...')
    const updated = await rollbackDocumentVersionRemote(documentId, versionId)
    setWorkspace((prev) => {
      const validFolderIds = new Set(prev.folders.map((folder) => folder.id))
      const current = prev.documents.find((doc) => doc.id === documentId)
      const normalized = normalizeDocument(updated, defaultOwner, validFolderIds, current?.folderId ?? null)
      return updateWorkspaceStamp({
        ...prev,
        documents: prev.documents.map((doc) => (doc.id === documentId ? normalized : doc)),
        activeDocumentId: normalized.id,
      })
    })
    setSyncMessage('已回滚到历史版本')
  }

  const clearQueuedPatch = (id: string) => {
    delete queueRef.current[id]
    const timer = timerRef.current[id]
    if (timer) {
      window.clearTimeout(timer)
      delete timerRef.current[id]
    }
  }

  const updateDocument = (id: string, patch: Partial<TeamDocument>) => {
    setWorkspace((prev) => {
      const validFolderIds = new Set(prev.folders.map((folder) => folder.id))
      return updateWorkspaceStamp({
        ...prev,
        documents: prev.documents.map((doc) => {
          if (doc.id !== id) {
            return doc
          }

          const nextFolderId =
            patch.folderId !== undefined ? normalizeFolderId(patch.folderId, validFolderIds) : doc.folderId

          return {
            ...doc,
            ...patch,
            folderId: nextFolderId,
            updatedAt: now(),
          }
        }),
      })
    })

    if (remoteReadyRef.current) {
      const remotePatch = toRemotePatch(patch)
      if (Object.keys(remotePatch).length > 0) {
        schedulePatch(id, remotePatch)
      }
    }
  }

  useEffect(() => {
    return () => {
      Object.values(timerRef.current).forEach((timer) => window.clearTimeout(timer))
      timerRef.current = {}
      queueRef.current = {}
    }
  }, [])

  return {
    workspace,
    activeDocument,
    syncMessage,
    setActiveDocument,
    createDocument,
    createFolder,
    moveDocumentToFolder,
    removeDocument,
    restoreDocument,
    refreshRecycleBin,
    listDocumentVersions,
    rollbackDocumentVersion,
    updateDocument,
  }
}
