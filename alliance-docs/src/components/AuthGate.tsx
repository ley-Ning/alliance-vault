import { useState, type FormEvent } from 'react'
import { Alert, Button, Input } from 'antd'

interface AuthGateProps {
  onLogin: (username: string, password: string) => Promise<void>
}

export const AuthGate = ({ onLogin }: AuthGateProps) => {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError('')
    setSubmitting(true)

    try {
      await onLogin(username, password)
      setPassword('')
    } catch (err) {
      const message = err instanceof Error ? err.message : '登录失败'
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="auth-shell">
      <section className="auth-card">
        <p className="label">联盟文舱</p>
        <h1>登录协作文档</h1>
        <p className="auth-subtitle">仅管理员添加团队账号。首次登录会强制修改密码。</p>
        <p className="auth-tip">默认管理员：admin / 12345678</p>

        <form className="auth-form" onSubmit={submit}>
          <label>
            用户名
            <Input
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              placeholder="如：zhaoyun"
              autoComplete="username"
              required
            />
          </label>

          <label>
            密码
            <Input.Password
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="至少 8 位"
              autoComplete="current-password"
              required
            />
          </label>

          {error ? <Alert className="auth-alert" type="error" showIcon message={error} /> : null}

          <Button className="primary-btn auth-submit" type="primary" htmlType="submit" loading={submitting}>
            {submitting ? '处理中...' : '进入文档台'}
          </Button>
        </form>
      </section>
    </div>
  )
}
