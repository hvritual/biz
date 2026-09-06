export type Tenant = {
  id: string;
  name: string;
  status: string;
  version: number;
};

export type TenantMember = {
  userId: string;
  email: string;
  status: string;
  version: number;
};

export type PermissionGrant = {
  permission: string;
  scope: string;
};

export type TenantRole = {
  id: string;
  name: string;
  status: string;
  permissions?: PermissionGrant[];
  version: number;
};

export type Device = {
  id: string;
  siteId: string;
  name: string;
  serial: string;
  createdBy: string;
  version: number;
};

export type ConnectionSettings = {
  baseUrl: string;
  platformToken: string;
  tenantToken: string;
};

const SETTINGS_KEY = "biz-console.connection.v1";
const DEFAULT_BASE = import.meta.env.VITE_BIZ_API_BASE || "http://127.0.0.1:8080";

export function loadConnectionSettings(): ConnectionSettings {
  const fallback: ConnectionSettings = {
    baseUrl: DEFAULT_BASE,
    platformToken: import.meta.env.VITE_BIZ_PLATFORM_TOKEN || "",
    tenantToken: import.meta.env.VITE_BIZ_TENANT_TOKEN || "",
  };
  const raw = sessionStorage.getItem(SETTINGS_KEY);
  if (!raw) return fallback;
  try {
    return { ...fallback, ...JSON.parse(raw) };
  } catch {
    return fallback;
  }
}

export function saveConnectionSettings(settings: ConnectionSettings) {
  sessionStorage.setItem(SETTINGS_KEY, JSON.stringify(settings));
}

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly body?: string,
  ) {
    super(message);
  }
}

type RequestOptions = {
  token: string;
  method?: string;
  body?: unknown;
  idempotent?: boolean;
};

async function request<T>(path: string, options: RequestOptions): Promise<T> {
  const settings = loadConnectionSettings();
  const headers = new Headers({ Accept: "application/json" });
  if (options.token) headers.set("Authorization", `Bearer ${options.token}`);
  if (options.body !== undefined) headers.set("Content-Type", "application/json");
  if (options.idempotent) headers.set("Idempotency-Key", crypto.randomUUID());

  const response = await fetch(`${settings.baseUrl.replace(/\/$/, "")}${path}`, {
    method: options.method || "GET",
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  const text = await response.text();
  if (!response.ok) {
    let detail = text || response.statusText;
    try {
      const parsed = JSON.parse(text) as { message?: string; error?: string; detail?: string };
      detail = parsed.message || parsed.error || parsed.detail || detail;
    } catch {
      // Keep raw text when the gateway does not return JSON.
    }
    throw new ApiError(response.status, detail, text);
  }
  if (!text) return undefined as T;
  return JSON.parse(text) as T;
}

function platformToken() {
  return loadConnectionSettings().platformToken;
}

function tenantToken() {
  return loadConnectionSettings().tenantToken;
}

export const platformApi = {
  async listTenants() {
    const result = await request<{ tenants?: Tenant[] }>("/v1/tenants", { token: platformToken() });
    return result.tenants || [];
  },
  createTenant(input: { name: string; ownerUserId: string; ownerEmail: string }) {
    return request<Tenant>("/v1/tenants", {
      token: platformToken(),
      method: "POST",
      body: input,
      idempotent: true,
    });
  },
  updateTenant(tenant: Tenant, name: string) {
    return request<Tenant>(`/v1/tenants/${encodeURIComponent(tenant.id)}`, {
      token: platformToken(),
      method: "PATCH",
      body: { id: tenant.id, name, version: tenant.version },
      idempotent: true,
    });
  },
  activateTenant(tenant: Tenant) {
    return request<Tenant>(`/v1/tenants/${encodeURIComponent(tenant.id)}/activate`, {
      token: platformToken(),
      method: "POST",
      body: { id: tenant.id, version: tenant.version },
      idempotent: true,
    });
  },
  suspendTenant(tenant: Tenant) {
    return request<Tenant>(`/v1/tenants/${encodeURIComponent(tenant.id)}/suspend`, {
      token: platformToken(),
      method: "POST",
      body: { id: tenant.id, version: tenant.version },
      idempotent: true,
    });
  },
  closeTenant(tenant: Tenant) {
    return request<Tenant>(`/v1/tenants/${encodeURIComponent(tenant.id)}/close`, {
      token: platformToken(),
      method: "POST",
      body: { id: tenant.id, version: tenant.version },
      idempotent: true,
    });
  },
};

export const tenantApi = {
  async listMembers() {
    const result = await request<{ members?: TenantMember[] }>("/v1/tenant/members", { token: tenantToken() });
    return result.members || [];
  },
  inviteMember(email: string) {
    return request<TenantMember>("/v1/tenant/members", {
      token: tenantToken(),
      method: "POST",
      body: { email },
      idempotent: true,
    });
  },
  activateMember(member: TenantMember) {
    return request<TenantMember>(`/v1/tenant/members/${encodeURIComponent(member.userId)}/activate`, {
      token: tenantToken(),
      method: "POST",
      body: { userId: member.userId, version: member.version },
      idempotent: true,
    });
  },
  suspendMember(member: TenantMember) {
    return request<TenantMember>(`/v1/tenant/members/${encodeURIComponent(member.userId)}/suspend`, {
      token: tenantToken(),
      method: "POST",
      body: { userId: member.userId, version: member.version },
      idempotent: true,
    });
  },
  removeMember(member: TenantMember) {
    return request<TenantMember>(`/v1/tenant/members/${encodeURIComponent(member.userId)}/remove`, {
      token: tenantToken(),
      method: "POST",
      body: { userId: member.userId, version: member.version },
      idempotent: true,
    });
  },
  async listRoles() {
    const result = await request<{ roles?: TenantRole[] }>("/v1/tenant/roles", { token: tenantToken() });
    return result.roles || [];
  },
  createRole(name: string) {
    return request<TenantRole>("/v1/tenant/roles", {
      token: tenantToken(),
      method: "POST",
      body: { name },
      idempotent: true,
    });
  },
  updateRole(role: TenantRole, name: string) {
    return request<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(role.id)}`, {
      token: tenantToken(),
      method: "PATCH",
      body: { roleId: role.id, name, version: role.version },
      idempotent: true,
    });
  },
  enableRole(role: TenantRole) {
    return request<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(role.id)}/enable`, {
      token: tenantToken(),
      method: "POST",
      body: { roleId: role.id, version: role.version },
      idempotent: true,
    });
  },
  disableRole(role: TenantRole) {
    return request<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(role.id)}/disable`, {
      token: tenantToken(),
      method: "POST",
      body: { roleId: role.id, version: role.version },
      idempotent: true,
    });
  },
  setRolePermissions(role: TenantRole, permissions: PermissionGrant[]) {
    return request<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(role.id)}/permissions`, {
      token: tenantToken(),
      method: "PUT",
      body: { roleId: role.id, permissions, version: role.version },
      idempotent: true,
    });
  },
  assignRoleMember(roleId: string, userId: string) {
    return request<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(roleId)}/members`, {
      token: tenantToken(),
      method: "POST",
      body: { roleId, userId },
      idempotent: true,
    });
  },
  revokeRoleMember(roleId: string, userId: string) {
    return request<TenantRole>(`/v1/tenant/roles/${encodeURIComponent(roleId)}/members/${encodeURIComponent(userId)}/revoke`, {
      token: tenantToken(),
      method: "POST",
      body: { roleId, userId },
      idempotent: true,
    });
  },
  async listDevices() {
    const result = await request<{ devices?: Device[] }>("/v1/devices", { token: tenantToken() });
    return result.devices || [];
  },
  createDevice(input: { siteId: string; name: string; serial: string }) {
    return request<Device>("/v1/devices", {
      token: tenantToken(),
      method: "POST",
      body: input,
      idempotent: true,
    });
  },
  updateDevice(device: Device, input: { siteId: string; name: string }) {
    return request<Device>(`/v1/devices/${encodeURIComponent(device.id)}`, {
      token: tenantToken(),
      method: "PATCH",
      body: { id: device.id, ...input, version: device.version },
      idempotent: true,
    });
  },
  deleteDevice(device: Device) {
    return request<void>(`/v1/devices/${encodeURIComponent(device.id)}`, {
      token: tenantToken(),
      method: "DELETE",
      body: { id: device.id, version: device.version },
      idempotent: true,
    });
  },
  transferDevice(device: Device, targetSiteId: string) {
    return request<Device>(`/v1/devices/${encodeURIComponent(device.id)}/transfer`, {
      token: tenantToken(),
      method: "PATCH",
      body: { id: device.id, targetSiteId, version: device.version },
      idempotent: true,
    });
  },
};
