import { useState, type FormEvent } from 'react'
import { Alert, Button, Input } from 'antd'

interface PasswordResetGateProps {
  username: string
  onSubmit: (newPassword: string) => Promise<void>
  onLogout: () => Promise<void>
}

export const PasswordResetGate = ({ username, onSubmit, onLogout }: PasswordResetGateProps) => {
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError('')

    if (newPassword !== confirmPassword) {
      setError('两次输入的新密码不一致')
      return
    }

    setSubmitting(true)
    try {
      await onSubmit(newPassword)
      setNewPassword('')
      setConfirmPassword('')
    } catch (err) {
      const message = err instanceof Error ? err.message : '修改密码失败'
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="auth-shell">
      <section className="auth-card">
        <p className="label">安全校验</p>
        <h1>首次登录请先修改密码</h1>
        <p className="auth-subtitle">账号：{username}</p>
        <p className="auth-tip">修改成功后，方可进入文档工作台。</p>

        <form className="auth-form" onSubmit={submit}>
          <label>
            新密码
            <Input.Password
              value={newPassword}
              onChange={(event) => setNewPassword(event.target.value)}
              placeholder="至少 8 位"
              autoComplete="new-password"
              required
            />
          </label>

          <label>
            确认新密码
            <Input.Password
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
              placeholder="请再次输入新密码"
              autoComplete="new-password"
              required
            />
          </label>

          {error ? <Alert className="auth-alert" type="error" showIcon message={error} /> : null}

          <div className="auth-action-row">
            <Button className="primary-btn auth-submit" type="primary" htmlType="submit" loading={submitting}>
              {submitting ? '处理中...' : '确认修改'}
            </Button>
            <Button className="ghost-btn" onClick={() => void onLogout()}>
              退出登录
            </Button>
          </div>
        </form>
      </section>
    </div>
  )
}
