import dayjs from 'dayjs'
import { Tag } from 'antd'
import type { TeamDocument } from '../types'

interface InsightsPanelProps {
  documents: TeamDocument[]
  activeDocument: TeamDocument
}

const statusCount = (documents: TeamDocument[], status: TeamDocument['status']) =>
  documents.filter((doc) => doc.status === status).length

export const InsightsPanel = ({ documents, activeDocument }: InsightsPanelProps) => {
  return (
    <aside className="insights-panel">
      <section className="insight-card">
        <p className="label">全局进度</p>
        <h3>{documents.length} 篇协作文档</h3>
        <div className="progress-list">
          <p>
            草稿：<strong>{statusCount(documents, '草稿')}</strong>
          </p>
          <p>
            评审中：<strong>{statusCount(documents, '评审中')}</strong>
          </p>
          <p>
            已发布：<strong>{statusCount(documents, '已发布')}</strong>
          </p>
        </div>
      </section>

      <section className="insight-card">
        <p className="label">当前文档标签</p>
        <div className="tag-list">
          {activeDocument.tags.map((tag) => (
            <Tag key={tag}>{tag}</Tag>
          ))}
        </div>
      </section>

      <section className="insight-card">
        <p className="label">实时状态</p>
        <ul>
          <li>文档负责人：{activeDocument.owner}</li>
          <li>文档状态：{activeDocument.status}</li>
          <li>创建时间：{dayjs(activeDocument.createdAt).format('MM-DD HH:mm')}</li>
          <li>更新时间：{dayjs(activeDocument.updatedAt).format('MM-DD HH:mm')}</li>
        </ul>
      </section>
    </aside>
  )
}
