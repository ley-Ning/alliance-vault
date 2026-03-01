import dayjs from 'dayjs'
import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent } from 'react'
import { Alert, Button, Empty, Modal, Popconfirm, Spin } from 'antd'
import { completeUpload, deleteAttachment, getDownloadURL, listAttachments, presignUpload } from '../lib/api'
import type { Attachment, TeamDocument } from '../types'

interface AttachmentPanelProps {
  document: TeamDocument
  compact?: boolean
}

const parsedMaxUploadSizeMB = Number(import.meta.env.VITE_MAX_UPLOAD_SIZE_MB || 20)
const maxUploadSizeMB = Number.isFinite(parsedMaxUploadSizeMB) && parsedMaxUploadSizeMB > 0 ? parsedMaxUploadSizeMB : 20
const maxUploadSizeBytes = maxUploadSizeMB * 1024 * 1024

const formatSize = (bytes: number) => {
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

const lower = (value: string) => value.toLowerCase()

const extensionOf = (fileName: string) => {
  const index = fileName.lastIndexOf('.')
  if (index < 0) {
    return ''
  }
  return lower(fileName.slice(index + 1))
}

const isImageAttachment = (attachment: Attachment) => {
  const contentType = lower(attachment.contentType || '')
  const ext = extensionOf(attachment.fileName)
  return (
    contentType.startsWith('image/') ||
    ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico', 'avif'].includes(ext)
  )
}

const isVideoAttachment = (attachment: Attachment) => {
  const contentType = lower(attachment.contentType || '')
  const ext = extensionOf(attachment.fileName)
  return contentType.startsWith('video/') || ['mp4', 'webm', 'ogg', 'mov'].includes(ext)
}

const isAudioAttachment = (attachment: Attachment) => {
  const contentType = lower(attachment.contentType || '')
  const ext = extensionOf(attachment.fileName)
  return contentType.startsWith('audio/') || ['mp3', 'wav', 'ogg', 'aac', 'm4a', 'flac'].includes(ext)
}

const isPdfAttachment = (attachment: Attachment) => {
  const contentType = lower(attachment.contentType || '')
  const ext = extensionOf(attachment.fileName)
  return contentType.includes('application/pdf') || ext === 'pdf'
}

const isTextAttachment = (attachment: Attachment) => {
  const contentType = lower(attachment.contentType || '')
  const ext = extensionOf(attachment.fileName)
  if (contentType.startsWith('text/')) {
    return true
  }
  return [
    'txt',
    'md',
    'markdown',
    'json',
    'csv',
    'log',
    'xml',
    'yml',
    'yaml',
    'toml',
    'ini',
    'conf',
    'sql',
    'ts',
    'tsx',
    'js',
    'jsx',
    'css',
    'html',
    'htm',
    'go',
    'java',
    'py',
    'sh',
    'bash',
  ].includes(ext)
}

type PreviewKind = 'image' | 'video' | 'audio' | 'pdf' | 'text' | 'unsupported'

const detectPreviewKind = (attachment: Attachment): PreviewKind => {
  if (isImageAttachment(attachment)) {
    return 'image'
  }
  if (isVideoAttachment(attachment)) {
    return 'video'
  }
  if (isAudioAttachment(attachment)) {
    return 'audio'
  }
  if (isPdfAttachment(attachment)) {
    return 'pdf'
  }
  if (isTextAttachment(attachment)) {
    return 'text'
  }
  return 'unsupported'
}

const charsetFromContentType = (contentType: string) => {
  const match = contentType.match(/charset\s*=\s*([^;]+)/i)
  return match ? match[1].trim().toLowerCase() : ''
}

const decodeWith = (bytes: Uint8Array, encoding: string) => {
  try {
    return new TextDecoder(encoding, { fatal: true }).decode(bytes)
  } catch {
    return null
  }
}

const replacementCount = (value: string) => (value.match(/\ufffd/g) || []).length

const decodeTextBuffer = (buffer: ArrayBuffer, contentType: string) => {
  const bytes = new Uint8Array(buffer)
  const declaredCharset = charsetFromContentType(contentType)

  if (declaredCharset) {
    const decoded = decodeWith(bytes, declaredCharset)
    if (decoded !== null) {
      return decoded
    }
  }

  const candidates = ['utf-8', 'gb18030', 'gbk', 'big5', 'utf-16le']
  let best = ''
  let bestScore = Number.POSITIVE_INFINITY

  for (const encoding of candidates) {
    const decoded = decodeWith(bytes, encoding)
    if (decoded === null) {
      continue
    }
    const score = replacementCount(decoded)
    if (score < bestScore) {
      best = decoded
      bestScore = score
    }
    if (score === 0) {
      return decoded
    }
  }

  if (best) {
    return best
  }

  return new TextDecoder().decode(bytes)
}

export const AttachmentPanel = ({ document, compact = false }: AttachmentPanelProps) => {
  const [items, setItems] = useState<Attachment[]>([])
  const [loading, setLoading] = useState(false)
  const [uploadingCount, setUploadingCount] = useState(0)
  const [uploadingName, setUploadingName] = useState('')
  const [downloadingId, setDownloadingId] = useState<string | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [previewingId, setPreviewingId] = useState<string | null>(null)
  const [previewOpen, setPreviewOpen] = useState(false)
  const [previewAttachment, setPreviewAttachment] = useState<Attachment | null>(null)
  const [previewKind, setPreviewKind] = useState<PreviewKind>('unsupported')
  const [previewURL, setPreviewURL] = useState('')
  const [previewText, setPreviewText] = useState('')
  const [previewError, setPreviewError] = useState('')
  const [error, setError] = useState('')
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const uploading = uploadingCount > 0
  const previewLoading = previewingId !== null
  const isReadOnly = document.canEdit === false

  const loadItems = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const result = await listAttachments(document.id)
      setItems(result)
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载附件失败'
      setError(message)
      setItems([])
    } finally {
      setLoading(false)
    }
  }, [document.id])

  useEffect(() => {
    void loadItems()
  }, [loadItems])

  useEffect(() => {
    setPreviewOpen(false)
    setPreviewAttachment(null)
    setPreviewKind('unsupported')
    setPreviewURL('')
    setPreviewText('')
    setPreviewError('')
    setPreviewingId(null)
  }, [document.id])

  const hasItems = useMemo(() => items.length > 0, [items])

  const openPicker = () => {
    if (isReadOnly) {
      return
    }
    fileInputRef.current?.click()
  }

  const validateFile = (file: File) => {
    if (file.size <= 0) {
      return `文件「${file.name}」大小为 0，无法上传。`
    }
    if (file.size > maxUploadSizeBytes) {
      return `文件「${file.name}」超过 ${maxUploadSizeMB} MB 限制。`
    }
    return ''
  }

  const uploadSingleFile = async (file: File) => {
    const contentType = file.type || 'application/octet-stream'
    const sign = await presignUpload({
      documentId: document.id,
      fileName: file.name,
      contentType,
      sizeBytes: file.size,
    })

    const uploadHeaders = new Headers(sign.requiredHeaders)
    if (!uploadHeaders.has('Content-Type')) {
      uploadHeaders.set('Content-Type', contentType)
    }

    const uploadResponse = await fetch(sign.uploadUrl, {
      method: sign.method,
      headers: uploadHeaders,
      body: file,
    })

    if (!uploadResponse.ok) {
      throw new Error(`上传到对象存储失败（${uploadResponse.status}）`)
    }

    return await completeUpload({
      documentId: document.id,
      objectKey: sign.objectKey,
      fileName: file.name,
      contentType,
      sizeBytes: file.size,
      owner: document.owner,
    })
  }

  const onPickFile = async (event: ChangeEvent<HTMLInputElement>) => {
    if (isReadOnly) {
      event.target.value = ''
      return
    }
    const files = Array.from(event.target.files ?? [])
    event.target.value = ''
    if (files.length === 0) {
      return
    }

    setError('')

    const validFiles: File[] = []
    for (const file of files) {
      const validationMessage = validateFile(file)
      if (validationMessage) {
        setError(validationMessage)
        continue
      }
      validFiles.push(file)
    }

    if (validFiles.length === 0) {
      return
    }

    setUploadingCount(validFiles.length)

    const uploadedItems: Attachment[] = []
    const failedFiles: string[] = []

    for (const file of validFiles) {
      setUploadingName(file.name)
      try {
        const saved = await uploadSingleFile(file)
        uploadedItems.push(saved)
      } catch (err) {
        const rawMessage = err instanceof Error ? err.message : '上传失败'
        const message =
          /failed to fetch/i.test(rawMessage) || /network/i.test(rawMessage)
            ? '上传失败：无法连接文件服务，请检查后端与 RustFS 是否已启动。'
            : rawMessage
        failedFiles.push(`${file.name}（${message}）`)
      } finally {
        setUploadingCount((count) => Math.max(count - 1, 0))
      }
    }

    setUploadingName('')

    if (uploadedItems.length > 0) {
      setItems((prev) => [...uploadedItems, ...prev])
      void loadItems()
    }

    if (failedFiles.length > 0) {
      setError(`以下文件上传失败：${failedFiles.join('；')}`)
    }
  }

  const onDownload = async (attachment: Attachment) => {
    setDownloadingId(attachment.id)
    setError('')
    try {
      const result = await getDownloadURL(attachment.id)
      window.open(result.downloadUrl, '_blank', 'noopener,noreferrer')
    } catch (err) {
      const message = err instanceof Error ? err.message : '生成下载链接失败'
      setError(message)
    } finally {
      setDownloadingId(null)
    }
  }

  const onDelete = async (attachment: Attachment) => {
    if (isReadOnly) {
      return
    }
    setDeletingId(attachment.id)
    setError('')
    try {
      await deleteAttachment(attachment.id)
      setItems((prev) => prev.filter((item) => item.id !== attachment.id))
    } catch (err) {
      const message = err instanceof Error ? err.message : '删除附件失败'
      setError(message)
    } finally {
      setDeletingId(null)
    }
  }

  const onPreview = async (attachment: Attachment) => {
    const kind = detectPreviewKind(attachment)
    setPreviewOpen(true)
    setPreviewAttachment(attachment)
    setPreviewKind(kind)
    setPreviewURL('')
    setPreviewText('')
    setPreviewError('')
    setPreviewingId(attachment.id)

    try {
      const result = await getDownloadURL(attachment.id)
      setPreviewURL(result.downloadUrl)

      if (kind === 'text') {
        const response = await fetch(result.downloadUrl)
        if (!response.ok) {
          throw new Error(`拉取预览内容失败（${response.status}）`)
        }
        const buffer = await response.arrayBuffer()
        const contentType = response.headers.get('content-type') || attachment.contentType || ''
        const decoded = decodeTextBuffer(buffer, contentType)
        setPreviewText(decoded)
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : '生成预览链接失败'
      setPreviewError(message)
    } finally {
      setPreviewingId(null)
    }
  }

  const renderPreviewContent = () => {
    if (previewLoading) {
      return (
        <div className="attachment-preview-loading">
          <Spin tip="正在生成在线预览..." />
        </div>
      )
    }

    if (previewError) {
      return <Alert type="error" showIcon message={previewError} />
    }

    if (!previewAttachment || !previewURL) {
      return <Empty description="暂时无法预览该文件" />
    }

    if (previewKind === 'image') {
      return <img className="attachment-preview-image" src={previewURL} alt={previewAttachment.fileName} />
    }

    if (previewKind === 'video') {
      return <video className="attachment-preview-video" src={previewURL} controls />
    }

    if (previewKind === 'audio') {
      return (
        <div className="attachment-preview-audio-wrap">
          <audio className="attachment-preview-audio" src={previewURL} controls />
        </div>
      )
    }

    if (previewKind === 'text') {
      return <pre className="attachment-preview-text">{previewText || '（空文件）'}</pre>
    }

    if (previewKind === 'pdf') {
      return <iframe className="attachment-preview-frame" src={previewURL} title={previewAttachment.fileName} />
    }

    return <Empty description="该格式暂不支持在线预览，请点击下载查看。" />
  }

  return (
    <section className={compact ? 'attachment-panel compact' : 'attachment-panel'}>
      <div className="attachment-head">
        <div>
          <p className="label">文档附件</p>
          <h3>{compact ? '附件区' : '上传与共享'}</h3>
        </div>
        <div className="attachment-actions">
          <Button className="toolbar-btn" onClick={() => void loadItems()} disabled={loading || uploading}>
            刷新
          </Button>
          <Button
            className="primary-btn"
            type="primary"
            onClick={openPicker}
            loading={uploading}
            disabled={isReadOnly}
          >
            {uploading ? `上传中（剩余 ${uploadingCount}）` : '上传文件'}
          </Button>
        </div>

        <input
          ref={fileInputRef}
          className="file-input"
          type="file"
          onChange={onPickFile}
          disabled={uploading || isReadOnly}
          multiple
        />
      </div>

      {isReadOnly ? <p className="attachment-uploading-hint">当前文档是只读权限，附件仅支持预览和下载。</p> : null}

      {uploading && uploadingName ? <p className="attachment-uploading-hint">正在上传：{uploadingName}</p> : null}

      {error ? <Alert className="attachment-error" type="error" showIcon message={error} /> : null}

      <div className="attachment-list">
        {loading ? (
          <div className="attachment-loading">
            <Spin tip="正在同步附件..." />
          </div>
        ) : null}

        {!loading && !hasItems ? <Empty className="attachment-empty" description="暂无附件，点击右上角先上传一份。" /> : null}

        {items.map((item) => (
          <article key={item.id} className="attachment-item">
            <div>
              <button
                className="attachment-name-btn"
                type="button"
                onClick={() => void onPreview(item)}
                disabled={previewLoading || deletingId === item.id}
                title="点击在线预览"
              >
                {item.fileName}
              </button>
              <p>
                {formatSize(item.sizeBytes)} · {item.owner} · {dayjs(item.createdAt).format('MM-DD HH:mm')}
              </p>
            </div>
            <div className="attachment-item-actions">
              <Button
                className="toolbar-btn"
                onClick={() => void onPreview(item)}
                disabled={previewLoading || deletingId === item.id}
              >
                预览
              </Button>
              <Button
                className="toolbar-btn"
                onClick={() => onDownload(item)}
                disabled={downloadingId === item.id || deletingId === item.id || previewLoading}
              >
                {downloadingId === item.id ? '生成中...' : '下载'}
              </Button>
              <Popconfirm
                title="删除附件"
                description="删除后不可恢复，确认继续吗？"
                okText="删除"
                cancelText="取消"
                onConfirm={() => onDelete(item)}
              >
                <Button
                  className="ghost-btn"
                  danger
                  loading={deletingId === item.id}
                  disabled={downloadingId === item.id || previewLoading || isReadOnly}
                >
                  删除
                </Button>
              </Popconfirm>
            </div>
          </article>
        ))}
      </div>

      <Modal
        className="attachment-preview-modal"
        title={previewAttachment ? `在线预览 · ${previewAttachment.fileName}` : '在线预览'}
        open={previewOpen}
        onCancel={() => setPreviewOpen(false)}
        width={980}
        footer={[
          <Button key="close" onClick={() => setPreviewOpen(false)}>
            关闭
          </Button>,
          <Button
            key="open"
            type="primary"
            onClick={() => {
              if (previewURL) {
                window.open(previewURL, '_blank', 'noopener,noreferrer')
              }
            }}
            disabled={!previewURL}
          >
            新窗口打开
          </Button>,
        ]}
        destroyOnClose
      >
        <div className="attachment-preview-content">{renderPreviewContent()}</div>
      </Modal>
    </section>
  )
}
