import { useMemo, useState, type DragEvent, type FormEvent } from 'react'
import dayjs from 'dayjs'
import { Button, Input, Popconfirm, Tag } from 'antd'
import type { TeamDocument, WorkspaceFolder } from '../types'

interface SidebarProps {
  folders: WorkspaceFolder[]
  documents: TeamDocument[]
  recycleCount: number
  isAdmin: boolean
  activeDocumentId: string
  onSelect: (id: string) => void
  onCreate: () => void
  onCreateFolder: (name: string) => void
  onMoveDocument: (documentId: string, folderId: string | null) => void
  onDelete: (id: string) => void
  onOpenRecycleBin: () => void
  onOpenPermissionCenter: () => void
}

const stripHtml = (value: string) => value.replace(/<[^>]+>/g, '').replace(/\s+/g, ' ').trim()

const statusClassMap = {
  草稿: 'draft',
  评审中: 'review',
  已发布: 'published',
} as const

interface FolderGroup {
  id: string
  name: string
  docs: TeamDocument[]
}

const folderNameFromDoc = (doc: TeamDocument, folders: WorkspaceFolder[]) => {
  const folder = folders.find((item) => item.id === doc.folderId)
  return folder ? folder.name : '未分组'
}

export const Sidebar = ({
  folders,
  documents,
  recycleCount,
  isAdmin,
  activeDocumentId,
  onSelect,
  onCreate,
  onCreateFolder,
  onMoveDocument,
  onDelete,
  onOpenRecycleBin,
  onOpenPermissionCenter,
}: SidebarProps) => {
  const [keyword, setKeyword] = useState('')
  const [folderName, setFolderName] = useState('')
  const [draggingDocId, setDraggingDocId] = useState<string | null>(null)
  const [dragTargetFolderId, setDragTargetFolderId] = useState<string | null | 'root'>(null)

  const filteredDocs = useMemo(() => {
    const q = keyword.trim().toLowerCase()
    if (!q) {
      return documents
    }

    return documents.filter((doc) => {
      return (
        doc.title.toLowerCase().includes(q) ||
        doc.tags.join(' ').toLowerCase().includes(q) ||
        stripHtml(doc.content).toLowerCase().includes(q) ||
        folderNameFromDoc(doc, folders).toLowerCase().includes(q)
      )
    })
  }, [documents, folders, keyword])

  const groupedDocs = useMemo(() => {
    const folderMap = new Map<string, TeamDocument[]>()
    folders.forEach((folder) => {
      folderMap.set(folder.id, [])
    })

    const rootDocs: TeamDocument[] = []

    filteredDocs.forEach((doc) => {
      if (doc.folderId && folderMap.has(doc.folderId)) {
        folderMap.get(doc.folderId)?.push(doc)
      } else {
        rootDocs.push(doc)
      }
    })

    const folderGroups: FolderGroup[] = folders.map((folder) => ({
      id: folder.id,
      name: folder.name,
      docs: folderMap.get(folder.id) ?? [],
    }))

    return {
      rootDocs,
      folderGroups,
    }
  }, [filteredDocs, folders])

  const submitFolder = (event: FormEvent) => {
    event.preventDefault()
    const trimmed = folderName.trim()
    if (!trimmed) {
      return
    }

    onCreateFolder(trimmed)
    setFolderName('')
  }

  const onDragStartDoc = (docId: string, event: DragEvent<HTMLElement>) => {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', docId)
    setDraggingDocId(docId)
  }

  const clearDragState = () => {
    setDraggingDocId(null)
    setDragTargetFolderId(null)
  }

  const onDropToFolder = (folderId: string | null, event: DragEvent<HTMLElement>) => {
    event.preventDefault()
    const docId = event.dataTransfer.getData('text/plain') || draggingDocId
    if (docId) {
      onMoveDocument(docId, folderId)
    }
    clearDragState()
  }

  const renderDocCard = (doc: TeamDocument) => {
    const active = doc.id === activeDocumentId
    const dragging = doc.id === draggingDocId

    return (
      <article
        key={doc.id}
        className={`doc-card ${active ? 'active' : ''} ${dragging ? 'dragging' : ''}`}
        onClick={() => onSelect(doc.id)}
        draggable
        onDragStart={(event) => onDragStartDoc(doc.id, event)}
        onDragEnd={clearDragState}
      >
        <div className="doc-card-top">
          <h2>{doc.title || '未命名文档'}</h2>
          <Popconfirm
            title="删除文档"
            description="删除后会进入回收站，可随时恢复。确认继续吗？"
            okText="移入回收站"
            cancelText="取消"
            onConfirm={(event) => {
              event?.stopPropagation()
              onDelete(doc.id)
            }}
            disabled={doc.canEdit === false}
          >
            <Button
              className="ghost-btn"
              onClick={(event) => {
                event.stopPropagation()
              }}
              disabled={doc.canEdit === false}
              title={doc.canEdit === false ? '当前文档仅有只读权限，无法删除' : '删除文档'}
              size="small"
            >
              删除
            </Button>
          </Popconfirm>
        </div>
        <p className="doc-preview">{stripHtml(doc.content) || '点击开始编写内容...'}</p>
        <div className="doc-meta-row">
          <Tag className={`status-badge ${statusClassMap[doc.status]}`}>{doc.status}</Tag>
          <span>{dayjs(doc.updatedAt).format('MM-DD HH:mm')}</span>
        </div>
      </article>
    )
  }

  return (
    <aside className="sidebar">
      <div className="sidebar-head">
        <div className="brand-wrap">
          <img className="brand-logo" src="/logo.svg" alt="联盟文舱 Logo" />
          <div>
            <p className="label">联盟文舱</p>
            <h1>协作文档工具</h1>
          </div>
        </div>
        <Button className="primary-btn" type="primary" onClick={onCreate}>
          新建文档
        </Button>
      </div>

      <form className="folder-create" onSubmit={submitFolder}>
        <Input
          value={folderName}
          placeholder="新增目录，比如：季度复盘"
          onChange={(event) => setFolderName(event.target.value)}
        />
        <Button className="toolbar-btn" htmlType="submit">
          新增目录
        </Button>
      </form>

      <div className="search-box">
        <Input
          value={keyword}
          placeholder="搜标题 / 标签 / 内容 / 目录"
          onChange={(event) => setKeyword(event.target.value)}
        />
      </div>

      <section className="quick-actions">
        <Button className="toolbar-btn" onClick={onOpenRecycleBin}>
          回收站（{recycleCount}）
        </Button>
      </section>

      {isAdmin ? (
        <section className="admin-menu">
          <p>管理员菜单</p>
          <Button className="toolbar-btn" onClick={onOpenPermissionCenter}>
            权限分配
          </Button>
        </section>
      ) : null}

      <div className="doc-list">
        <section
          className={`folder-block ${dragTargetFolderId === 'root' ? 'drag-over' : ''}`}
          onDragOver={(event) => {
            event.preventDefault()
            if (draggingDocId) {
              setDragTargetFolderId('root')
            }
          }}
          onDragLeave={() => {
            if (dragTargetFolderId === 'root') {
              setDragTargetFolderId(null)
            }
          }}
          onDrop={(event) => onDropToFolder(null, event)}
        >
          <header className="folder-header">
            <h2>未分组</h2>
            <span>{groupedDocs.rootDocs.length} 篇</span>
          </header>
          <div className="folder-docs">{groupedDocs.rootDocs.map((doc) => renderDocCard(doc))}</div>
        </section>

        {groupedDocs.folderGroups.map((folder) => (
          <section
            key={folder.id}
            className={`folder-block ${dragTargetFolderId === folder.id ? 'drag-over' : ''}`}
            onDragOver={(event) => {
              event.preventDefault()
              if (draggingDocId) {
                setDragTargetFolderId(folder.id)
              }
            }}
            onDragLeave={() => {
              if (dragTargetFolderId === folder.id) {
                setDragTargetFolderId(null)
              }
            }}
            onDrop={(event) => onDropToFolder(folder.id, event)}
          >
            <header className="folder-header">
              <h2>{folder.name}</h2>
              <span>{folder.docs.length} 篇</span>
            </header>
            <div className="folder-docs">{folder.docs.map((doc) => renderDocCard(doc))}</div>
          </section>
        ))}

        {!filteredDocs.length ? (
          <div className="empty-state">
            <p>没有找到匹配文档，试试换个关键词。</p>
          </div>
        ) : null}
      </div>
    </aside>
  )
}
