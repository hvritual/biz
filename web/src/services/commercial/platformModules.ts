import {
  CommercialApiError,
  commercialRequestId,
  type ModuleDTO,
  type ModuleSalesStatus,
  type ModuleTechnicalStatus,
} from './platformCommercial'

const configuredBase = import.meta.env.VITE_API_BASE_URL ?? '/api'
const baseUrl = configuredBase.endsWith('/') ? configuredBase.slice(0, -1) : configuredBase

function encoded(value: string | number) {
  return encodeURIComponent(String(value).trim())
}

async function parseError(response: Response) {
  const fallback = `模块管理接口请求失败（HTTP ${response.status}）`
  try {
    const body = (await response.json()) as { message?: string; error?: string }
    return body.message || body.error || fallback
  } catch {
    return fallback
  }
}

async function parseResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const message = await parseError(response)
    if (response.status === 401) throw new CommercialApiError(message, 401, 'unauthenticated')
    if (response.status === 403) throw new CommercialApiError(message, 403, 'forbidden')
    if (response.status === 409) throw new CommercialApiError(message, 409, 'conflict')
    throw new CommercialApiError(message, response.status, 'http')
  }
  try {
    return (await response.json()) as T
  } catch {
    throw new CommercialApiError('模块管理接口返回了无法解析的数据', response.status, 'invalid-response')
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    credentials: 'include',
    headers,
  })
  return parseResponse<T>(response)
}

async function trustedCsrfToken() {
  const session = await request<{ authenticated?: boolean; csrf_token?: string }>('/auth/session')
  if (!session.authenticated || !session.csrf_token) {
    throw new CommercialApiError('当前可信会话不可执行平台模块写操作', 401, 'unauthenticated')
  }
  return session.csrf_token
}

async function mutate<T>(path: string, method: 'POST' | 'PATCH', body: Record<string, unknown>, requestId: string) {
  const csrf = await trustedCsrfToken()
  return request<T>(path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrf,
      'Idempotency-Key': requestId,
    },
    body: JSON.stringify(body),
  })
}

export function getPlatformModule(moduleCode: string) {
  return request<ModuleDTO>(`/v1/platform/modules/${encoded(moduleCode)}`)
}

export function updatePlatformModule(
  module: ModuleDTO,
  input: { name: string; category: string; salesScope: string[]; reason: string },
) {
  const requestId = commercialRequestId('ce13-module-update')
  return mutate<ModuleDTO>(
    `/v1/platform/modules/${encoded(module.moduleCode)}`,
    'PATCH',
    {
      requestId,
      moduleCode: module.moduleCode,
      name: input.name.trim(),
      category: input.category.trim(),
      salesScope: input.salesScope.map((value) => value.trim()).filter(Boolean),
      version: String(module.version),
      reason: input.reason.trim(),
    },
    requestId,
  )
}

export function setPlatformModuleSalesStatus(module: ModuleDTO, salesStatus: ModuleSalesStatus, reason: string) {
  const requestId = commercialRequestId('ce13-module-sales')
  return mutate<ModuleDTO>(
    `/v1/platform/modules/${encoded(module.moduleCode)}/sales-status`,
    'POST',
    {
      requestId,
      moduleCode: module.moduleCode,
      salesStatus,
      version: String(module.version),
      reason: reason.trim(),
    },
    requestId,
  )
}

export function setPlatformModuleTechnicalStatus(
  module: ModuleDTO,
  technicalStatus: ModuleTechnicalStatus,
  reason: string,
) {
  const requestId = commercialRequestId('ce13-module-technical')
  return mutate<ModuleDTO>(
    `/v1/platform/modules/${encoded(module.moduleCode)}/technical-status`,
    'POST',
    {
      requestId,
      moduleCode: module.moduleCode,
      technicalStatus,
      version: String(module.version),
      reason: reason.trim(),
    },
    requestId,
  )
}
