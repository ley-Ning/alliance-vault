import { useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  Button,
  Empty,
  Input,
  Modal,
  Popconfirm,
  Select,
  Spin,
  Switch,
  Tabs,
  Tag,
} from 'antd'
import dayjs from 'dayjs'
import {
  deleteAdminUser,
  deleteDocumentPermission,
  listAdminUsers,
  listDocumentPermissions,
  updateAdminRole,
  updateAdminUserDisabled,
  upsertDocumentPermission,
  type AdminUserItem,
  type DocumentPermissionAccess,
  type DocumentPermissionItem,
} from '../lib/api'
import type { TeamDocument, WorkspaceFolder } from '../types'

const ROOT_FOLDER_VALUE = '__ungrouped__'

type PermissionTabKey = 'users' | 'documents'
type AssignmentScope = 'document' | 'folder'

interface PermissionCenterProps {
  open: boolean
  currentUserId: string
  documents: TeamDocument[]
  folders: WorkspaceFolder[]
  activeDocumentId: string
  onClose: () => void
  onCreateMember: (username: string, password: string, displayName?: string) => Promise<void>
}

export const PermissionCenter = ({
  open,
  currentUserId,
  documents,
  folders,
  activeDocumentId,
  onClose,
  onCreateMember,
}: PermissionCenterProps) => {
  const wasOpenRef = useRef(false)
  const [activeTab, setActiveTab] = useState<PermissionTabKey>('users')
  const [items, setItems] = useState<AdminUserItem[]>([])
  const [loading, setLoading] = useState(false)
  const [savingId, setSavingId] = useState<string | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')

  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')

  const [assignmentScope, setAssignmentScope] = useState<AssignmentScope>('document')
  const [selectedDocumentId, setSelectedDocumentId] = useState('')
  const [selectedFolderId, setSelectedFolderId] = useState('')
  const [permissionViewDocumentId, setPermissionViewDocumentId] = useState('')
  const [selectedUserId, setSelectedUserId] = useState('')
  const [selectedAccess, setSelectedAccess] = useState<DocumentPermissionAccess>('read')
  const [permissionItems, setPermissionItems] = useState<DocumentPermissionItem[]>([])
  const [permissionLoading, setPermissionLoading] = useState(false)
  const [permissionSaving, setPermissionSaving] = useState(false)

  const loadUsers = async () => {
    setLoading(true)
    setError('')
    try {
      const users = await listAdminUsers()
      setItems(users)
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载成员权限失败'
      setError(message)
      setItems([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!open) {
      return
    }
    void loadUsers()
  }, [open])

  const defaultDocumentId = useMemo(() => {
    if (documents.length === 0) {
      return ''
    }
    const activeExists = documents.some((doc) => doc.id === activeDocumentId)
    if (activeExists) {
      return activeDocumentId
    }
    return documents[0].id
  }, [activeDocumentId, documents])

  const effectiveDocumentId = useMemo(() => {
    if (!selectedDocumentId) {
      return defaultDocumentId
    }
    const exists = documents.some((doc) => doc.id === selectedDocumentId)
    return exists ? selectedDocumentId : defaultDocumentId
  }, [defaultDocumentId, documents, selectedDocumentId])

  const effectivePermissionViewDocumentId = useMemo(() => {
    if (!permissionViewDocumentId) {
      return defaultDocumentId
    }
    const exists = documents.some((doc) => doc.id === permissionViewDocumentId)
    return exists ? permissionViewDocumentId : defaultDocumentId
  }, [defaultDocumentId, documents, permissionViewDocumentId])

  const rootDocumentCount = useMemo(() => documents.filter((doc) => !doc.folderId).length, [documents])

  const folderOptions = useMemo(() => {
    const options = [
      {
        label: `未分组（${rootDocumentCount} 篇）`,
        value: ROOT_FOLDER_VALUE,
      },
    ]

    folders.forEach((folder) => {
      const count = documents.filter((doc) => doc.folderId === folder.id).length
      options.push({
        label: `${folder.name}（${count} 篇）`,
        value: folder.id,
      })
    })

    return options
  }, [documents, folders, rootDocumentCount])

  const effectiveFolderId = useMemo(() => {
    if (!selectedFolderId) {
      return folderOptions[0]?.value ?? ''
    }
    const exists = folderOptions.some((item) => item.value === selectedFolderId)
    return exists ? selectedFolderId : folderOptions[0]?.value ?? ''
  }, [folderOptions, selectedFolderId])

  const documentsInFolder = useMemo(() => {
    if (!effectiveFolderId) {
      return []
    }

    if (effectiveFolderId === ROOT_FOLDER_VALUE) {
      return documents.filter((doc) => !doc.folderId)
    }

    return documents.filter((doc) => doc.folderId === effectiveFolderId)
  }, [documents, effectiveFolderId])

  const assignmentTargetDocumentIds = useMemo(() => {
    if (assignmentScope === 'document') {
      return effectiveDocumentId ? [effectiveDocumentId] : []
    }
    return documentsInFolder.map((doc) => doc.id)
  }, [assignmentScope, documentsInFolder, effectiveDocumentId])

  useEffect(() => {
    if (open && !wasOpenRef.current) {
      setActiveTab('users')
      setAssignmentScope('document')
      if (defaultDocumentId) {
        setSelectedDocumentId(defaultDocumentId)
        setPermissionViewDocumentId(defaultDocumentId)
      } else {
        setSelectedDocumentId('')
        setPermissionViewDocumentId('')
      }
      if (folderOptions.length > 0) {
        setSelectedFolderId(folderOptions[0].value)
      } else {
        setSelectedFolderId('')
      }
    }

    wasOpenRef.current = open
  }, [defaultDocumentId, folderOptions, open])

  const loadDocumentPermissionsFor = async (documentId: string) => {
    setPermissionLoading(true)
    setError('')
    try {
      const permissions = await listDocumentPermissions(documentId)
      setPermissionItems(permissions)
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载文档权限失败'
      setError(message)
      setPermissionItems([])
    } finally {
      setPermissionLoading(false)
    }
  }

  useEffect(() => {
    if (!open || !effectivePermissionViewDocumentId) {
      return
    }
    void loadDocumentPermissionsFor(effectivePermissionViewDocumentId)
  }, [effectivePermissionViewDocumentId, open])

  const canSubmit = useMemo(() => {
    return username.trim().length >= 3 && password.trim().length >= 8
  }, [password, username])

  const submitCreateMember = async () => {
    if (!canSubmit) {
      return
    }

    setCreating(true)
    setError('')
    try {
      await onCreateMember(username, password, displayName || undefined)
      setUsername('')
      setDisplayName('')
      setPassword('')
      await loadUsers()
    } catch (err) {
      const message = err instanceof Error ? err.message : '新增成员失败'
      setError(message)
    } finally {
      setCreating(false)
    }
  }

  const toggleAdmin = async (user: AdminUserItem, nextValue: boolean) => {
    setSavingId(user.id)
    setError('')
    try {
      const updated = await updateAdminRole(user.id, nextValue)
      setItems((prev) => prev.map((item) => (item.id === updated.id ? updated : item)))
      setPermissionItems((prev) =>
        prev.map((item) => (item.userId === updated.id ? { ...item, isAdmin: updated.isAdmin } : item)),
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : '更新权限失败'
      setError(message)
    } finally {
      setSavingId(null)
    }
  }

  const toggleDisabled = async (user: AdminUserItem, nextValue: boolean) => {
    setSavingId(user.id)
    setError('')
    try {
      const updated = await updateAdminUserDisabled(user.id, nextValue)
      setItems((prev) => prev.map((item) => (item.id === updated.id ? updated : item)))
      setPermissionItems((prev) =>
        prev.map((item) => (item.userId === updated.id ? { ...item, isDisabled: updated.isDisabled } : item)),
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : '更新账号状态失败'
      setError(message)
    } finally {
      setSavingId(null)
    }
  }

  const removeUser = async (user: AdminUserItem) => {
    setDeletingId(user.id)
    setError('')
    try {
      await deleteAdminUser(user.id)
      setItems((prev) => prev.filter((item) => item.id !== user.id))
      setPermissionItems((prev) => prev.filter((item) => item.userId !== user.id))
      if (selectedUserId === user.id) {
        setSelectedUserId('')
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : '删除账号失败'
      setError(message)
    } finally {
      setDeletingId(null)
    }
  }

  const assignDocumentPermission = async () => {
    if (!selectedUserId) {
      return
    }

    const targets = assignmentTargetDocumentIds
    if (targets.length === 0) {
      setError('当前没有可分配的目标文档')
      return
    }

    setPermissionSaving(true)
    setError('')
    try {
      if (assignmentScope === 'document') {
        const updated = await upsertDocumentPermission(targets[0], selectedUserId, selectedAccess)
        if (targets[0] === effectivePermissionViewDocumentId) {
          setPermissionItems((prev) => {
            const exists = prev.some((item) => item.userId === updated.userId)
            if (exists) {
              return prev.map((item) => (item.userId === updated.userId ? updated : item))
            }
            return [updated, ...prev]
          })
        }
      } else {
        const results = await Promise.allSettled(
          targets.map((documentId) => upsertDocumentPermission(documentId, selectedUserId, selectedAccess)),
        )

        const failedCount = results.filter((result) => result.status === 'rejected').length
        if (failedCount > 0) {
          const successCount = results.length - failedCount
          setError(`目录批量分配已完成，但有部分失败：成功 ${successCount}，失败 ${failedCount}`)
        }
      }

      if (effectivePermissionViewDocumentId && targets.includes(effectivePermissionViewDocumentId)) {
        await loadDocumentPermissionsFor(effectivePermissionViewDocumentId)
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : '分配文档权限失败'
      setError(message)
    } finally {
      setPermissionSaving(false)
    }
  }

  const removeDocumentPermissionForUser = async (userId: string) => {
    if (!effectivePermissionViewDocumentId) {
      return
    }

    setPermissionSaving(true)
    setError('')
    try {
      await deleteDocumentPermission(effectivePermissionViewDocumentId, userId)
      setPermissionItems((prev) => prev.filter((item) => item.userId !== userId))
    } catch (err) {
      const message = err instanceof Error ? err.message : '移除文档权限失败'
      setError(message)
    } finally {
      setPermissionSaving(false)
    }
  }

  const documentOptions = useMemo(
    () => documents.map((doc) => ({ label: doc.title || '未命名文档', value: doc.id })),
    [documents],
  )

  const memberOptions = useMemo(
    () =>
      items.map((user) => ({
        label: `${user.displayName || user.username}（${user.username}${user.isDisabled ? '，已禁用' : ''}）`,
        value: user.id,
      })),
    [items],
  )

  const selectedFolderLabel = useMemo(() => {
    const matched = folderOptions.find((item) => item.value === effectiveFolderId)
    return matched?.label ?? '未分组'
  }, [effectiveFolderId, folderOptions])

  const handleRefresh = async () => {
    if (activeTab === 'users') {
      await loadUsers()
      return
    }
    if (effectivePermissionViewDocumentId) {
      await loadDocumentPermissionsFor(effectivePermissionViewDocumentId)
    }
  }

  const usersTab = (
    <>
      <section className="permission-create">
        <p className="label">新增成员</p>
        <div className="permission-create-grid">
          <Input
            value={username}
            onChange={(event) => setUsername(event.target.value.trim())}
            placeholder="用户名（如 guanyu）"
          />
          <Input
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            placeholder="显示名（可选）"
          />
          <Input.Password value={password} onChange={(event) => setPassword(event.target.value)} placeholder="初始密码（至少 8 位）" />
          <Button className="primary-btn" type="primary" loading={creating} onClick={() => void submitCreateMember()} disabled={!canSubmit}>
            新增成员
          </Button>
        </div>
      </section>

      <section className="permission-list">
        <p className="label">账号状态与角色</p>
        {loading ? (
          <div className="permission-loading">
            <Spin tip="正在拉取成员权限..." />
          </div>
        ) : (
          items.map((user) => (
            <article key={user.id} className="permission-item">
              <div>
                <h4>{user.displayName || user.username}</h4>
                <p>
                  {user.username} · 创建于 {dayjs(user.createdAt).format('YYYY-MM-DD HH:mm')}
                </p>
                {user.mustChangePassword ? <Tag color="orange">首次登录需改密码</Tag> : null}
                {user.isDisabled ? <Tag color="red">账号已禁用</Tag> : <Tag color="green">账号可用</Tag>}
              </div>
              <div className="permission-item-action">
                <span>{user.isAdmin ? '管理员' : '普通成员'}</span>
                <Switch
                  checked={user.isAdmin}
                  checkedChildren="管理员"
                  unCheckedChildren="成员"
                  loading={savingId === user.id}
                  disabled={savingId !== null || deletingId !== null || user.id === currentUserId || user.isDisabled}
                  onChange={(next) => void toggleAdmin(user, next)}
                />
                <Switch
                  checked={!user.isDisabled}
                  checkedChildren="启用"
                  unCheckedChildren="禁用"
                  loading={savingId === user.id}
                  disabled={savingId !== null || deletingId !== null || user.id === currentUserId}
                  onChange={(nextEnabled) => void toggleDisabled(user, !nextEnabled)}
                />
                <Popconfirm
                  title="确认删除该账号？"
                  description="删除后将移除其登录能力和文档专属权限。"
                  onConfirm={() => void removeUser(user)}
                  okText="删除"
                  cancelText="取消"
                  disabled={savingId !== null || deletingId !== null || user.id === currentUserId}
                >
                  <Button
                    className="ghost-btn"
                    danger
                    loading={deletingId === user.id}
                    disabled={savingId !== null || deletingId !== null || user.id === currentUserId}
                  >
                    删除账号
                  </Button>
                </Popconfirm>
              </div>
            </article>
          ))
        )}
      </section>
    </>
  )

  const documentsTab = (
    <>
      <section className="doc-permission-panel">
        <p className="label">批量/单文档分配</p>
        <div className="doc-permission-form">
          <Select
            value={assignmentScope}
            options={[
              { label: '按单文档分配', value: 'document' },
              { label: '按目录分组分配', value: 'folder' },
            ]}
            onChange={(value) => setAssignmentScope(value)}
          />

          {assignmentScope === 'document' ? (
            <Select
              value={effectiveDocumentId || undefined}
              options={documentOptions}
              placeholder="选择文档"
              onChange={(value) => setSelectedDocumentId(value)}
            />
          ) : (
            <Select
              value={effectiveFolderId || undefined}
              options={folderOptions}
              placeholder="选择目录"
              onChange={(value) => setSelectedFolderId(value)}
            />
          )}

          <Select
            value={selectedUserId || undefined}
            options={memberOptions}
            placeholder="选择成员"
            onChange={(value) => setSelectedUserId(value)}
          />
          <Select
            value={selectedAccess}
            options={[
              { label: '只读（read）', value: 'read' },
              { label: '可编辑（edit）', value: 'edit' },
            ]}
            onChange={(value) => setSelectedAccess(value)}
          />
          <Button
            className="primary-btn"
            type="primary"
            loading={permissionSaving}
            disabled={assignmentTargetDocumentIds.length === 0 || !selectedUserId}
            onClick={() => void assignDocumentPermission()}
          >
            {assignmentScope === 'document' ? '分配权限' : '批量分配'}
          </Button>
        </div>

        {assignmentScope === 'folder' ? (
          <p className="doc-permission-hint">
            将对「{selectedFolderLabel}」下 {assignmentTargetDocumentIds.length} 篇文档统一分配 {selectedAccess} 权限。
          </p>
        ) : null}
      </section>

      <section className="doc-permission-view">
        <p className="label">文档权限明细</p>
        <Select
          className="doc-permission-view-select"
          value={effectivePermissionViewDocumentId || undefined}
          options={documentOptions}
          placeholder="选择要查看权限明细的文档"
          onChange={(value) => setPermissionViewDocumentId(value)}
        />

        <div className="doc-permission-list">
          {permissionLoading ? (
            <div className="permission-loading">
              <Spin tip="正在拉取文档权限..." />
            </div>
          ) : null}

          {!permissionLoading && !effectivePermissionViewDocumentId ? <Empty description="暂无文档可查看权限" /> : null}

          {!permissionLoading && effectivePermissionViewDocumentId && permissionItems.length === 0 ? (
            <Empty description="当前文档尚未分配专属权限，默认全员可见可编辑" />
          ) : null}

          {!permissionLoading &&
            permissionItems.map((permission) => (
              <article key={permission.userId} className="doc-permission-item">
                <div>
                  <h4>{permission.displayName || permission.username}</h4>
                  <p>
                    {permission.username} · 权限 {permission.accessLevel === 'edit' ? '可编辑' : '只读'} · 更新于{' '}
                    {dayjs(permission.updatedAt).format('YYYY-MM-DD HH:mm')}
                  </p>
                </div>
                <Popconfirm
                  title="移除该成员在当前文档的专属权限？"
                  onConfirm={() => void removeDocumentPermissionForUser(permission.userId)}
                  okText="移除"
                  cancelText="取消"
                  disabled={permissionSaving}
                >
                  <Button className="ghost-btn" disabled={permissionSaving}>
                    移除
                  </Button>
                </Popconfirm>
              </article>
            ))}
        </div>
      </section>
    </>
  )

  return (
    <Modal
      title="权限分配中心（仅管理员）"
      open={open}
      onCancel={onClose}
      width={980}
      footer={[
        <Button key="refresh" className="toolbar-btn" onClick={() => void handleRefresh()} disabled={loading || creating}>
          刷新
        </Button>,
        <Button key="close" onClick={onClose}>
          关闭
        </Button>,
      ]}
      destroyOnClose
    >
      <Tabs
        className="permission-tabs"
        activeKey={activeTab}
        onChange={(value) => setActiveTab(value as PermissionTabKey)}
        items={[
          { key: 'users', label: '用户管理', children: usersTab },
          { key: 'documents', label: '文档权限分配', children: documentsTab },
        ]}
      />

      {error ? <Alert className="permission-error" type="error" showIcon message={error} /> : null}
    </Modal>
  )
}
