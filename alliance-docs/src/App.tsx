import { useEffect, useState } from 'react'
import './App.css'
import { Button, Spin, Tag } from 'antd'
import { AuthGate } from './components/AuthGate'
import { Sidebar } from './components/Sidebar'
import { EditorPane } from './components/EditorPane'
import { InsightsPanel } from './components/InsightsPanel'
import { AttachmentPanel } from './components/AttachmentPanel'
import { PasswordResetGate } from './components/PasswordResetGate'
import { PermissionCenter } from './components/PermissionCenter'
import { RecycleBinModal } from './components/RecycleBinModal'
import { useAuth } from './hooks/useAuth'
import { useWorkspace } from './hooks/useWorkspace'
import type { AuthSession } from './types'

interface WorkspaceScreenProps {
  session: AuthSession
  onCreateTeamMember: (username: string, password: string, displayName?: string) => Promise<void>
  onLogout: () => Promise<void>
}

function WorkspaceScreen({ session, onCreateTeamMember, onLogout }: WorkspaceScreenProps) {
  const [mainFullscreen, setMainFullscreen] = useState(false)
  const [permissionOpen, setPermissionOpen] = useState(false)
  const [recycleOpen, setRecycleOpen] = useState(false)
  const userDisplayName = session.user.displayName || session.user.username

  const {
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
  } = useWorkspace(userDisplayName)

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setMainFullscreen(false)
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [])

  return (
    <div className={`app-shell ${mainFullscreen ? 'main-fullscreen' : ''}`}>
      <Sidebar
        folders={workspace.folders}
        documents={workspace.documents}
        recycleCount={workspace.recycleBin.length}
        isAdmin={session.user.isAdmin}
        activeDocumentId={workspace.activeDocumentId}
        onSelect={setActiveDocument}
        onCreate={createDocument}
        onCreateFolder={createFolder}
        onMoveDocument={moveDocumentToFolder}
        onDelete={removeDocument}
        onOpenRecycleBin={() => setRecycleOpen(true)}
        onOpenPermissionCenter={() => setPermissionOpen(true)}
      />

      <main className="main-pane">
        <div className="main-header">
          <div className="session-box">
            <Tag className="sync-hint">{syncMessage}</Tag>
            <Button className="toolbar-btn fullscreen-btn" onClick={() => setMainFullscreen((prev) => !prev)}>
              {mainFullscreen ? '退出全屏' : '中间全屏'}
            </Button>
            <Button className="ghost-btn logout-btn" onClick={() => void onLogout()}>
              退出登录
            </Button>
          </div>
        </div>

        {activeDocument ? (
          <EditorPane
            key={activeDocument.id}
            document={activeDocument}
            onPatch={updateDocument}
            onLoadVersions={listDocumentVersions}
            onRollbackVersion={rollbackDocumentVersion}
          />
        ) : (
          <section className="editor-empty">
            <h3>当前没有文档</h3>
            <p>主公已清空文档库，可随时新建一篇继续写作。</p>
            <Button className="primary-btn" type="primary" onClick={() => void createDocument()}>
              新建第一篇文档
            </Button>
          </section>
        )}
      </main>

      <aside className="right-pane">
        {activeDocument ? (
          <>
            <InsightsPanel documents={workspace.documents} activeDocument={activeDocument} />
            <AttachmentPanel document={activeDocument} compact />
          </>
        ) : (
          <section className="insights-panel">
            <article className="insight-card">
              <p className="label">协作状态</p>
              <h3>暂无活跃文档</h3>
              <p>右侧统计和附件区会在新建文档后自动恢复。</p>
            </article>
          </section>
        )}
      </aside>

      <PermissionCenter
        open={permissionOpen}
        currentUserId={session.user.id}
        documents={workspace.documents}
        folders={workspace.folders}
        activeDocumentId={workspace.activeDocumentId}
        onClose={() => setPermissionOpen(false)}
        onCreateMember={onCreateTeamMember}
      />

      <RecycleBinModal
        open={recycleOpen}
        items={workspace.recycleBin}
        onClose={() => setRecycleOpen(false)}
        onRefresh={refreshRecycleBin}
        onRestore={restoreDocument}
      />
    </div>
  )
}

function App() {
  const { session, booting, login, changePassword, createTeamMember, logout } = useAuth()

  if (booting) {
    return (
      <div className="auth-shell">
        <section className="auth-card">
          <p className="label">联盟文舱</p>
          <h1>正在恢复会话...</h1>
          <p className="auth-subtitle">请稍候，小赵正在连接主公的云端文档空间。</p>
          <div className="booting-wrap">
            <Spin />
          </div>
        </section>
      </div>
    )
  }

  if (!session) {
    return <AuthGate onLogin={login} />
  }

  if (session.user.mustChangePassword) {
    return (
      <PasswordResetGate
        username={session.user.username}
        onSubmit={changePassword}
        onLogout={logout}
      />
    )
  }

  return <WorkspaceScreen session={session} onCreateTeamMember={createTeamMember} onLogout={logout} />
}

export default App
