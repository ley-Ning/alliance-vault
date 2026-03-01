import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import 'antd/dist/reset.css'
import './index.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ConfigProvider
      locale={zhCN}
      theme={{
        token: {
          colorPrimary: '#1d6d64',
          colorInfo: '#1d6d64',
          colorSuccess: '#1d6d64',
          colorWarning: '#c17e2b',
          colorError: '#a53a31',
          borderRadius: 12,
          fontFamily: "'Songti SC', 'STSong', 'Hiragino Sans GB', 'PingFang SC', 'Microsoft YaHei', sans-serif",
        },
      }}
    >
      <App />
    </ConfigProvider>
  </StrictMode>,
)
