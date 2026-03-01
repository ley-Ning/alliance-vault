import { useEffect, useMemo, useState } from 'react'
import dayjs from 'dayjs'
import { Button, Empty, Modal, Spin, Tag } from 'antd'
import type { TeamDocument } from '../types'

interface RecycleBinModalProps {
  open: boolean
  items: TeamDocument[]
  onClose: () => void
  onRestore: (id: string) => Promise<void>
  onRefresh: () => Promise<void>
}

const stripHtml = (value: string) => value.replace(/<[^>]+>/g, '').replace(/\s+/g, ' ').trim()

export const RecycleBinModal = ({ open, items, onClose, onRestore, onRefresh }: RecycleBinModalProps) => {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [restoringId, setRestoringId] = useState('')

  useEffect(() => {
    if (!open) {
      return
    }

    setLoading(true)
    setError('')
    void onRefresh()
      .catch((err) => {
        const message = err instanceof Error ? err.message : '回收站加载失败'
        setError(message)
      })
      .finally(() => {
        setLoading(false)
      })
  }, [open, onRefresh])

  const sortedItems = useMemo(
    () => [...items].sort((left, right) => new Date(right.updatedAt).getTime() - new Date(left.updatedAt).getTime()),
    [items],
  )

  const restore = async (id: string) => {
    setError('')
    setRestoringId(id)
    try {
      await onRestore(id)
    } catch (err) {
      const message = err instanceof Error ? err.message : '恢复失败，请稍后重试'
      setError(message)
    } finally {
      setRestoringId('')
    }
  }

  return (
    <Modal
      open={open}
      title="文档回收站"
      onCancel={onClose}
      width={760}
      footer={null}
      destroyOnClose
    >
      {error ? <p className="recycle-error">{error}</p> : null}
      {loading ? (
        <div className="recycle-loading">
          <Spin />
        </div>
      ) : sortedItems.length === 0 ? (
        <Empty description="回收站是空的，暂无可恢复文档。" />
      ) : (
        <div className="recycle-list">
          {sortedItems.map((doc) => (
            <article key={doc.id} className="recycle-item">
              <div>
                <h4>{doc.title || '未命名文档'}</h4>
                <p>{stripHtml(doc.content) || '该文档暂无正文'}</p>
                <div className="recycle-meta">
                  <Tag className="status-badge">{doc.status}</Tag>
                  <span>删除时间：{dayjs(doc.updatedAt).format('YYYY-MM-DD HH:mm')}</span>
                </div>
              </div>
              <Button
                className="toolbar-btn"
                onClick={() => void restore(doc.id)}
                loading={restoringId === doc.id}
                disabled={doc.canEdit === false}
              >
                恢复
              </Button>
            </article>
          ))}
        </div>
      )}
    </Modal>
  )
}
