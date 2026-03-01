import { useEffect, useRef, useState, type ChangeEvent } from 'react'
import dayjs from 'dayjs'
import CharacterCount from '@tiptap/extension-character-count'
import Placeholder from '@tiptap/extension-placeholder'
import StarterKit from '@tiptap/starter-kit'
import { EditorContent, useEditor } from '@tiptap/react'
import { Button, Empty, Input, Modal, Popconfirm, Select, Spin, Tag } from 'antd'
import { htmlToMarkdown, markdownToHtml } from '../lib/markdown'
import type { DocStatus, DocumentVersion, TeamDocument } from '../types'

interface EditorPaneProps {
  document: TeamDocument
  onPatch: (id: string, patch: Partial<TeamDocument>) => void
  onLoadVersions: (documentId: string, limit?: number) => Promise<DocumentVersion[]>
  onRollbackVersion: (documentId: string, versionId: string) => Promise<void>
}

const statuses: DocStatus[] = ['草稿', '评审中', '已发布']

const splitTags = (raw: string) =>
  raw
    .split(/[，,]/)
    .map((item) => item.trim())
    .filter(Boolean)

const versionEventLabelMap: Record<string, string> = {
  create: '创建快照',
  update: '修改前快照',
  delete: '删除前快照',
  restore: '恢复快照',
  rollback_backup: '回滚前快照',
}

export const EditorPane = ({ document, onPatch, onLoadVersions, onRollbackVersion }: EditorPaneProps) => {
  const debounceRef = useRef<number | null>(null)
  const markdownImportRef = useRef<HTMLInputElement | null>(null)
  const markdownEditorRef = useRef<HTMLTextAreaElement | null>(null)
  const markdownPreviewRef = useRef<HTMLElement | null>(null)
  const syncingScrollRef = useRef<'editor' | 'preview' | null>(null)
  const modeRef = useRef<'rich' | 'markdown'>('rich')
  const isReadOnlyRef = useRef(false)
  const loadVersionsRef = useRef(onLoadVersions)

  const [mode, setMode] = useState<'rich' | 'markdown'>('rich')
  const [markdownDraft, setMarkdownDraft] = useState(() => htmlToMarkdown(document.content))
  const [historyOpen, setHistoryOpen] = useState(false)
  const [historyLoading, setHistoryLoading] = useState(false)
  const [historyError, setHistoryError] = useState('')
  const [historyItems, setHistoryItems] = useState<DocumentVersion[]>([])
  const [topHistoryLoading, setTopHistoryLoading] = useState(false)
  const [topHistoryItems, setTopHistoryItems] = useState<DocumentVersion[]>([])
  const [rollingVersionId, setRollingVersionId] = useState('')
  const isReadOnly = document.canEdit === false

  const editor = useEditor({
    extensions: [
      StarterKit,
      Placeholder.configure({
        placeholder: '请在这里编写团队知识、会议纪要或方案，支持实时保存。',
      }),
      CharacterCount.configure({ limit: 20000 }),
    ],
    editorProps: {
      attributes: {
        class: 'editor-body',
      },
    },
    editable: !isReadOnly,
    content: document.content,
    onUpdate({ editor: currentEditor }) {
      if (modeRef.current !== 'rich' || isReadOnlyRef.current) {
        return
      }
      if (debounceRef.current) {
        window.clearTimeout(debounceRef.current)
      }
      const html = currentEditor.getHTML()
      debounceRef.current = window.setTimeout(() => {
        onPatch(document.id, { content: html })
      }, 150)
    },
  })

  useEffect(() => {
    if (!editor) {
      return
    }
    if (editor.getHTML() !== document.content) {
      editor.commands.setContent(document.content, { emitUpdate: false })
    }
  }, [document.id, document.content, editor])

  useEffect(() => {
    modeRef.current = mode
  }, [mode])

  useEffect(() => {
    isReadOnlyRef.current = isReadOnly
  }, [isReadOnly])

  useEffect(() => {
    loadVersionsRef.current = onLoadVersions
  }, [onLoadVersions])

  useEffect(() => {
    if (!editor) {
      return
    }
    editor.setEditable(!isReadOnly)
  }, [editor, isReadOnly])

  useEffect(() => {
    return () => {
      if (debounceRef.current) {
        window.clearTimeout(debounceRef.current)
      }
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    const loadTopHistory = async () => {
      setTopHistoryLoading(true)
      try {
        const versions = await loadVersionsRef.current(document.id, 6)
        if (cancelled) {
          return
        }
        setTopHistoryItems(versions.slice(0, 4))
      } catch {
        if (cancelled) {
          return
        }
        setTopHistoryItems([])
      } finally {
        if (!cancelled) {
          setTopHistoryLoading(false)
        }
      }
    }

    void loadTopHistory()
    return () => {
      cancelled = true
    }
  }, [document.id])

  const characters = mode === 'markdown' ? markdownDraft.length : editor ? editor.storage.characterCount.characters() : 0

  const switchToMode = (nextMode: 'rich' | 'markdown') => {
    if (nextMode === 'markdown') {
      setMarkdownDraft(htmlToMarkdown(document.content))
    }
    setMode(nextMode)
  }

  const onChangeMarkdown = (event: ChangeEvent<HTMLTextAreaElement>) => {
    if (isReadOnly) {
      return
    }
    const nextMarkdown = event.target.value
    setMarkdownDraft(nextMarkdown)
    onPatch(document.id, { content: markdownToHtml(nextMarkdown) })
  }

  const syncScrollPosition = (source: 'editor' | 'preview') => {
    const editorEl = markdownEditorRef.current
    const previewEl = markdownPreviewRef.current
    if (!editorEl || !previewEl) {
      return
    }

    if (syncingScrollRef.current && syncingScrollRef.current !== source) {
      return
    }

    const sourceEl = source === 'editor' ? editorEl : previewEl
    const targetEl = source === 'editor' ? previewEl : editorEl
    const sourceScrollable = sourceEl.scrollHeight - sourceEl.clientHeight
    const targetScrollable = targetEl.scrollHeight - targetEl.clientHeight
    const ratio = sourceScrollable > 0 ? sourceEl.scrollTop / sourceScrollable : 0

    syncingScrollRef.current = source
    targetEl.scrollTop = targetScrollable > 0 ? ratio * targetScrollable : 0
    window.requestAnimationFrame(() => {
      syncingScrollRef.current = null
    })
  }

  const exportMarkdown = () => {
    const markdown = htmlToMarkdown(document.content)
    const blob = new Blob([markdown], { type: 'text/markdown;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const anchor = window.document.createElement('a')
    const safeTitle = (document.title || '文档').replace(/[\\/:*?"<>|]+/g, '-')
    anchor.href = url
    anchor.download = `${safeTitle}.md`
    anchor.click()
    URL.revokeObjectURL(url)
  }

  const importMarkdown = () => {
    markdownImportRef.current?.click()
  }

  const onImportFile = async (event: ChangeEvent<HTMLInputElement>) => {
    if (isReadOnly) {
      event.target.value = ''
      return
    }
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) {
      return
    }

    const markdown = await file.text()
    setMode('markdown')
    setMarkdownDraft(markdown)
    onPatch(document.id, {
      title: file.name.replace(/\.md$/i, '') || document.title,
      content: markdownToHtml(markdown),
    })
  }

  const loadHistory = async () => {
    setHistoryLoading(true)
    setHistoryError('')
    try {
      const versions = await onLoadVersions(document.id, 80)
      setHistoryItems(versions)
      setTopHistoryItems(versions.slice(0, 4))
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载历史版本失败'
      setHistoryError(message)
    } finally {
      setHistoryLoading(false)
    }
  }

  const openHistory = async () => {
    setHistoryOpen(true)
    await loadHistory()
  }

  const rollbackVersion = async (versionId: string) => {
    setRollingVersionId(versionId)
    setHistoryError('')
    try {
      await onRollbackVersion(document.id, versionId)
      await loadHistory()
    } catch (err) {
      const message = err instanceof Error ? err.message : '回滚失败，请稍后重试'
      setHistoryError(message)
    } finally {
      setRollingVersionId('')
    }
  }

  return (
    <section className="editor-panel">
      <header className="editor-head">
        <Input
          className="title-input"
          value={document.title}
          onChange={(event) => onPatch(document.id, { title: event.target.value || '未命名文档' })}
          placeholder="文档标题"
          readOnly={isReadOnly}
        />

        <section className="version-strip">
          <div className="version-strip-head">
            <p className="label">历史版本</p>
            <Button className="toolbar-btn" htmlType="button" size="small" onClick={() => void openHistory()}>
              查看全部
            </Button>
          </div>
          {topHistoryLoading ? (
            <div className="version-strip-loading">
              <Spin size="small" />
            </div>
          ) : topHistoryItems.length === 0 ? (
            <p className="version-strip-empty">暂无历史版本，编辑后会自动生成快照。</p>
          ) : (
            <div className="version-strip-list">
              {topHistoryItems.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className="version-chip"
                  onClick={() => void openHistory()}
                  title={`${versionEventLabelMap[item.event] || '变更快照'} · ${dayjs(item.createdAt).format('YYYY-MM-DD HH:mm:ss')}`}
                >
                  V{item.version} · {versionEventLabelMap[item.event] || '快照'} ·{' '}
                  {dayjs(item.createdAt).format('MM-DD HH:mm')}
                </button>
              ))}
            </div>
          )}
        </section>

        <div className="meta-grid">
          <label>
            状态
            <Select
              value={document.status}
              options={statuses.map((status) => ({ label: status, value: status }))}
              onChange={(value) => onPatch(document.id, { status: value as DocStatus })}
              disabled={isReadOnly}
            />
          </label>

          <label className="tags-field">
            标签
            <Input
              value={document.tags.join('，')}
              onChange={(event) => onPatch(document.id, { tags: splitTags(event.target.value) })}
              placeholder="多个标签用逗号分隔"
              readOnly={isReadOnly}
            />
          </label>
        </div>

        <div className="editor-toolbar">
          <Button
            className={mode === 'rich' ? 'toolbar-btn active' : 'toolbar-btn'}
            onClick={() => switchToMode('rich')}
            htmlType="button"
          >
            富文本
          </Button>
          <Button
            className={mode === 'markdown' ? 'toolbar-btn active' : 'toolbar-btn'}
            onClick={() => switchToMode('markdown')}
            htmlType="button"
          >
            Markdown
          </Button>
          <Button className="toolbar-btn" onClick={importMarkdown} htmlType="button" disabled={isReadOnly}>
            导入 .md
          </Button>
          <Button className="toolbar-btn" onClick={exportMarkdown} htmlType="button">
            导出 .md
          </Button>
          <Button className="toolbar-btn" onClick={() => void openHistory()} htmlType="button">
            历史版本
          </Button>
          <Button
            className={editor?.isActive('bold') ? 'toolbar-btn active' : 'toolbar-btn'}
            onClick={() => editor?.chain().focus().toggleBold().run()}
            htmlType="button"
            disabled={mode !== 'rich' || isReadOnly}
          >
            加粗
          </Button>
          <Button
            className={editor?.isActive('italic') ? 'toolbar-btn active' : 'toolbar-btn'}
            onClick={() => editor?.chain().focus().toggleItalic().run()}
            htmlType="button"
            disabled={mode !== 'rich' || isReadOnly}
          >
            斜体
          </Button>
          <Button
            className={editor?.isActive('bulletList') ? 'toolbar-btn active' : 'toolbar-btn'}
            onClick={() => editor?.chain().focus().toggleBulletList().run()}
            htmlType="button"
            disabled={mode !== 'rich' || isReadOnly}
          >
            列表
          </Button>
          <Button
            className={editor?.isActive('blockquote') ? 'toolbar-btn active' : 'toolbar-btn'}
            onClick={() => editor?.chain().focus().toggleBlockquote().run()}
            htmlType="button"
            disabled={mode !== 'rich' || isReadOnly}
          >
            引用
          </Button>
        </div>
        {isReadOnly ? <p className="readonly-hint">当前文档仅有只读权限，可预览/导出，不可编辑。</p> : null}
      </header>

      <input
        ref={markdownImportRef}
        className="file-input"
        type="file"
        accept=".md,text/markdown"
        onChange={onImportFile}
        disabled={isReadOnly}
      />

      {mode === 'rich' ? (
        <EditorContent className="editor-content-wrap" editor={editor} />
      ) : (
        <div className="markdown-workbench">
          <textarea
            ref={markdownEditorRef}
            className="markdown-editor"
            value={markdownDraft}
            onChange={onChangeMarkdown}
            onScroll={() => syncScrollPosition('editor')}
            placeholder="在这里直接编写 Markdown，右侧会实时预览效果。"
            readOnly={isReadOnly}
          />
          <article
            ref={markdownPreviewRef}
            className="markdown-preview"
            onScroll={() => syncScrollPosition('preview')}
            dangerouslySetInnerHTML={{ __html: markdownToHtml(markdownDraft) }}
          />
        </div>
      )}

      <Modal
        open={historyOpen}
        title={`历史版本 · ${document.title || '未命名文档'}`}
        onCancel={() => setHistoryOpen(false)}
        footer={null}
        width={820}
        destroyOnClose
      >
        {historyError ? <p className="version-error">{historyError}</p> : null}
        {historyLoading ? (
          <div className="version-loading">
            <Spin />
          </div>
        ) : historyItems.length === 0 ? (
          <Empty description="暂无历史版本，开始编辑后会自动生成。" />
        ) : (
          <div className="version-list">
            {historyItems.map((item, index) => (
              <article key={item.id} className="version-item">
                <div>
                  <h4>
                    V{item.version}
                    <Tag className="version-event">{versionEventLabelMap[item.event] || item.event || '变更快照'}</Tag>
                    {index === 0 ? <Tag className="version-latest">最近</Tag> : null}
                  </h4>
                  <p>
                    操作人：{item.createdBy} · {dayjs(item.createdAt).format('YYYY-MM-DD HH:mm:ss')}
                  </p>
                  <p className="version-title">标题：{item.title || '未命名文档'}</p>
                </div>
                <Popconfirm
                  title="回滚确认"
                  description="将当前文档恢复到这个历史快照，当前内容会自动备份。"
                  okText="确认回滚"
                  cancelText="取消"
                  onConfirm={() => rollbackVersion(item.id)}
                  disabled={isReadOnly}
                >
                  <Button
                    className="toolbar-btn"
                    loading={rollingVersionId === item.id}
                    disabled={isReadOnly}
                  >
                    回滚到此版本
                  </Button>
                </Popconfirm>
              </article>
            ))}
          </div>
        )}
      </Modal>

      <footer className="editor-foot">
        <span>字数：{characters}</span>
        <span>最后更新：{dayjs(document.updatedAt).format('YYYY-MM-DD HH:mm:ss')}</span>
      </footer>
    </section>
  )
}
