import type { TeamDocument, WorkspaceFolder, WorkspaceState } from '../types'

const STORAGE_KEY = 'alliance-vault-workspace-v1'

const now = () => new Date().toISOString()

const initialFolders: WorkspaceFolder[] = [
  {
    id: 'folder-product',
    name: '产品规划',
    createdAt: now(),
  },
  {
    id: 'folder-onboarding',
    name: '新人培训',
    createdAt: now(),
  },
]

const initialDocs: TeamDocument[] = [
  {
    id: 'doc-roadmap',
    title: '协作平台 90 天路线图',
    content:
      '<h2>阶段目标</h2><p>第一阶段完成团队文档归档，第二阶段接入任务流与评审机制，第三阶段接入知识检索。</p><ul><li>第 1-2 周：文档模板与权限模型</li><li>第 3-6 周：实时协同编辑与评论</li><li>第 7-12 周：看板与统计面板</li></ul>',
    tags: ['规划', '协同'],
    status: '评审中',
    owner: '产品组',
    canEdit: true,
    folderId: initialFolders[0].id,
    createdAt: now(),
    updatedAt: now(),
  },
  {
    id: 'doc-onboarding',
    title: '新人入组作战手册',
    content:
      '<h2>开工清单</h2><p>加入团队后请先完成账号开通，再阅读开发规范，最后跟着导师完成一次发布演练。</p><ol><li>完成账号申请</li><li>阅读代码规范与提测流程</li><li>参与一次真实需求评审</li></ol>',
    tags: ['培训', '流程'],
    status: '草稿',
    owner: '研发效能组',
    canEdit: true,
    folderId: initialFolders[1].id,
    createdAt: now(),
    updatedAt: now(),
  },
]

const fallbackState: WorkspaceState = {
  folders: initialFolders,
  documents: initialDocs,
  recycleBin: [],
  activeDocumentId: initialDocs[0].id,
  updatedAt: now(),
}

type LegacyWorkspaceState = {
  folders?: WorkspaceFolder[]
  documents?: Array<Partial<TeamDocument>>
  recycleBin?: Array<Partial<TeamDocument>>
  activeDocumentId?: string
  updatedAt?: string
}

const ensureDocumentShape = (doc: Partial<TeamDocument>): TeamDocument => {
  const stamp = now()
  return {
    id: doc.id || crypto.randomUUID(),
    title: doc.title || '未命名文档',
    content: doc.content || '<p></p>',
    tags: Array.isArray(doc.tags) && doc.tags.length > 0 ? doc.tags : ['未分类'],
    status: doc.status || '草稿',
    owner: doc.owner || '未分配',
    canEdit: typeof doc.canEdit === 'boolean' ? doc.canEdit : true,
    folderId: typeof doc.folderId === 'string' ? doc.folderId : null,
    createdAt: doc.createdAt || stamp,
    updatedAt: doc.updatedAt || stamp,
  }
}

export const loadWorkspace = (): WorkspaceState => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      return fallbackState
    }

    const parsed = JSON.parse(raw) as LegacyWorkspaceState
    const documents = (parsed.documents ?? []).map(ensureDocumentShape)
    const recycleBin = (parsed.recycleBin ?? []).map(ensureDocumentShape)
    const folders = Array.isArray(parsed.folders)
      ? parsed.folders.filter((folder): folder is WorkspaceFolder => Boolean(folder?.id && folder.name))
      : []

    if (documents.length === 0) {
      return {
        folders,
        documents: [],
        recycleBin,
        activeDocumentId: '',
        updatedAt: parsed.updatedAt || now(),
      }
    }

    const validFolderIds = new Set(folders.map((folder) => folder.id))
    const normalizedDocuments = documents.map((doc) => ({
      ...doc,
      folderId: doc.folderId && validFolderIds.has(doc.folderId) ? doc.folderId : null,
    }))

    const activeDocumentId = parsed.activeDocumentId
    const hasActive = activeDocumentId
      ? normalizedDocuments.some((doc) => doc.id === activeDocumentId)
      : false

    return {
      folders,
      documents: normalizedDocuments,
      recycleBin: recycleBin.map((doc) => ({
        ...doc,
        folderId: doc.folderId && validFolderIds.has(doc.folderId) ? doc.folderId : null,
      })),
      activeDocumentId: hasActive && activeDocumentId ? activeDocumentId : normalizedDocuments[0].id,
      updatedAt: parsed.updatedAt || now(),
    }
  } catch {
    return fallbackState
  }
}

export const saveWorkspace = (state: WorkspaceState) => {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
}

export const createEmptyDocument = (owner = '未分配'): TeamDocument => {
  const stamp = now()
  return {
    id: crypto.randomUUID(),
    title: '新建文档',
    content: '<p></p>',
    tags: ['未分类'],
    status: '草稿',
    owner,
    canEdit: true,
    folderId: null,
    createdAt: stamp,
    updatedAt: stamp,
  }
}

export const workspaceStorageKey = STORAGE_KEY
