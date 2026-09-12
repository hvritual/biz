export type ModuleTechnicalStatus =
  | 'MODULE_TECHNICAL_STATUS_UNSPECIFIED'
  | 'MODULE_TECHNICAL_STATUS_NOT_READY'
  | 'MODULE_TECHNICAL_STATUS_READY'
  | 'MODULE_TECHNICAL_STATUS_DISABLED'

export type ModuleSalesStatus =
  | 'MODULE_SALES_STATUS_UNSPECIFIED'
  | 'MODULE_SALES_STATUS_SELLABLE'
  | 'MODULE_SALES_STATUS_RETIRED'

export interface ModuleDTO {
  moduleCode: string
  name: string
  category: string
  salesScope: string[]
  technicalStatus: ModuleTechnicalStatus
  salesStatus: ModuleSalesStatus
  capabilityCodes: string[]
  quotaSchemaKeys: string[]
  fieldPolicySchemaKeys: string[]
  dependencies: string[]
  version: string | number
}

interface ListModulesResponse {
  modules?: ModuleDTO[]
}

export class CommercialApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code: 'unauthenticated' | 'forbidden' | 'http' | 'invalid-response',
  ) {
    super(message)
  }
}

const configuredBase = import.meta.env.VITE_API_BASE_URL ?? '/api'
const baseUrl = configuredBase.endsWith('/') ? configuredBase.slice(0, -1) : configuredBase

async function parseError(response: Response) {
  const fallback = `商业平台接口请求失败（HTTP ${response.status}）`
  try {
    const body = (await response.json()) as { message?: string; error?: string }
    return body.message || body.error || fallback
  } catch {
    return fallback
  }
}

async function request<T>(path: string): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`, {
    method: 'GET',
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })

  if (!response.ok) {
    const message = await parseError(response)
    if (response.status === 401) throw new CommercialApiError(message, 401, 'unauthenticated')
    if (response.status === 403) throw new CommercialApiError(message, 403, 'forbidden')
    throw new CommercialApiError(message, response.status, 'http')
  }

  try {
    return (await response.json()) as T
  } catch {
    throw new CommercialApiError('商业平台接口返回了无法解析的数据', response.status, 'invalid-response')
  }
}

export async function listPlatformModules(): Promise<ModuleDTO[]> {
  const result = await request<ListModulesResponse>('/v1/platform/modules')
  return Array.isArray(result.modules) ? result.modules : []
}
