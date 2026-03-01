const normalizeBaseURL = (value: string) => value.trim().replace(/\/+$/, '')

const parseConfiguredBases = (raw: string | undefined): string[] => {
  if (!raw) {
    return []
  }

  return raw
    .split(',')
    .map((item) => normalizeBaseURL(item))
    .filter(Boolean)
}

const fallbackBases = [
  'http://127.0.0.1:9090',
  'http://localhost:9090',
  'http://127.0.0.1:9091',
  'http://localhost:9091',
  'http://127.0.0.1:8088',
  'http://localhost:8088',
]

const configuredBases = parseConfiguredBases(import.meta.env.VITE_API_BASE_URL as string | undefined)

const apiBaseCandidates = Array.from(new Set([...configuredBases, ...fallbackBases]))

let activeApiBase = apiBaseCandidates[0] || 'http://localhost:9090'
let discoveryDone = false
let discoveryPromise: Promise<void> | null = null

const orderedBases = () => {
  const index = apiBaseCandidates.indexOf(activeApiBase)
  if (index <= 0) {
    return apiBaseCandidates
  }
  return [activeApiBase, ...apiBaseCandidates.slice(0, index), ...apiBaseCandidates.slice(index + 1)]
}

const asConnectionError = (error: unknown) =>
  error instanceof TypeError || (error instanceof Error && /network|fetch/i.test(error.message))

const probeBase = async (base: string): Promise<boolean> => {
  try {
    const response = await fetch(`${base}/api/v1/health`)
    return response.ok
  } catch {
    return false
  }
}

const ensureApiBase = async () => {
  if (discoveryDone) {
    return
  }

  if (!discoveryPromise) {
    discoveryPromise = (async () => {
      for (const base of orderedBases()) {
        const available = await probeBase(base)
        if (available) {
          activeApiBase = base
          discoveryDone = true
          return
        }
      }
    })().finally(() => {
      discoveryPromise = null
    })
  }

  await discoveryPromise
}

export const requestApi = async (path: string, init?: RequestInit): Promise<Response> => {
  await ensureApiBase()

  let lastError: Error | null = null

  for (const base of orderedBases()) {
    try {
      const response = await fetch(`${base}${path}`, init)
      activeApiBase = base
      discoveryDone = true
      return response
    } catch (error) {
      if (asConnectionError(error)) {
        lastError = error instanceof Error ? error : new Error('网络请求失败')
        continue
      }
      throw error
    }
  }

  throw lastError ?? new Error('无法连接后端服务，请确认后端已启动')
}

export const getActiveApiBase = () => activeApiBase
