import React, { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  Activity,
  ArrowRightLeft,
  Building2,
  CircleGauge,
  Cpu,
  LayoutDashboard,
  LockKeyhole,
  Package,
  Plus,
  RefreshCw,
  Search,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  Users,
  X,
} from "lucide-react";
import {
  ApiError,
  ConnectionSettings,
  Device,
  PermissionGrant,
  Tenant,
  TenantMember,
  TenantRole,
  loadConnectionSettings,
  platformApi,
  saveConnectionSettings,
  tenantApi,
} from "./api";
import "./styles.css";

type Mode = "admin" | "workspace";
type PageId = "dashboard" | "tenants" | "members" | "roles" | "devices";
type NavItem = {
  id: PageId | string;
  label: string;
  icon: React.ComponentType<{ size?: number }>;
  disabled?: boolean;
  note?: string;
};

type Tab = { mode: Mode; id: PageId; label: string };

const ADMIN_NAV: NavItem[] = [
  { id: "dashboard", label: "工作台", icon: LayoutDashboard },
  { id: "tenants", label: "租户管理", icon: Building2 },
  { id: "commercial", label: "套餐与额度", icon: Package, disabled: true, note: "待后端契约" },
  { id: "features", label: "功能模块", icon: SlidersHorizontal, disabled: true, note: "待产品层契约" },
  { id: "global-settings", label: "全局系统设置", icon: Settings, disabled: true, note: "待后端契约" },
];

const WORKSPACE_NAV: NavItem[] = [
  { id: "dashboard", label: "工作台", icon: LayoutDashboard },
  { id: "departments", label: "组织部门", icon: Building2, disabled: true, note: "待 Department 契约" },
  { id: "members", label: "成员管理", icon: Users },
  { id: "roles", label: "角色与数据权限", icon: ShieldCheck },
  { id: "devices", label: "设备运营", icon: Cpu },
  { id: "activities", label: "业务活动", icon: Activity, disabled: true, note: "待 Activity 契约" },
];

const PAGE_LABELS: Record<Mode, Partial<Record<PageId, string>>> = {
  admin: { dashboard: "平台工作台", tenants: "租户管理" },
  workspace: { dashboard: "租户工作台", members: "成员管理", roles: "角色与数据权限", devices: "设备运营" },
};

function statusLeaf(status: string) {
  const parts = status.split("_");
  return parts[parts.length - 1] || status;
}

function statusClass(status: string) {
  switch (statusLeaf(status)) {
    case "ACTIVE":
      return "status status-success";
    case "PENDING":
    case "INVITED":
      return "status status-pending";
    case "SUSPENDED":
      return "status status-warning";
    case "CLOSED":
    case "REMOVED":
    case "DISABLED":
      return "status status-muted";
    default:
      return "status status-muted";
  }
}

function friendlyError(error: unknown) {
  if (error instanceof ApiError) {
    if (error.status === 401 || error.status === 403) return "当前凭证没有执行此操作的权限，请检查连接配置与授权。";
    if (error.status === 409 || error.status === 412 || /version|stale|optimistic/i.test(error.message)) {
      return "数据已被其他操作更新。请刷新后基于最新版本重试，系统不会覆盖并发修改。";
    }
    if (/owner/i.test(error.message)) {
      return "该操作可能破坏租户最后一个 Owner 的保护约束。请先为另一位 Active 成员授予 Owner，再重试。";
    }
    return `${error.status}: ${error.message}`;
  }
  return error instanceof Error ? error.message : "请求失败";
}

function App() {
  const initialMode: Mode = window.location.pathname.startsWith("/workspace") ? "workspace" : "admin";
  const [mode, setMode] = useState<Mode>(initialMode);
  const [page, setPage] = useState<PageId>("dashboard");
  const [refreshNonce, setRefreshNonce] = useState(0);
  const [settings, setSettings] = useState<ConnectionSettings>(() => loadConnectionSettings());
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [tabs, setTabs] = useState<Tab[]>([
    { mode: initialMode, id: "dashboard", label: PAGE_LABELS[initialMode].dashboard || "工作台" },
  ]);

  const navigate = useCallback(
    (nextPage: PageId) => {
      setPage(nextPage);
      const label = PAGE_LABELS[mode][nextPage] || nextPage;
      setTabs((current) => {
        if (current.some((tab) => tab.mode === mode && tab.id === nextPage)) return current;
        return [...current, { mode, id: nextPage, label }];
      });
    },
    [mode],
  );

  const switchMode = (nextMode: Mode) => {
    setMode(nextMode);
    setPage("dashboard");
    window.history.pushState({}, "", nextMode === "admin" ? "/admin" : "/workspace");
    setTabs((current) => {
      if (current.some((tab) => tab.mode === nextMode && tab.id === "dashboard")) return current;
      return [...current, { mode: nextMode, id: "dashboard", label: PAGE_LABELS[nextMode].dashboard || "工作台" }];
    });
  };

  const closeTab = (tab: Tab) => {
    setTabs((current) => current.filter((item) => !(item.mode === tab.mode && item.id === tab.id)));
    if (tab.mode === mode && tab.id === page) setPage("dashboard");
  };

  const nav = mode === "admin" ? ADMIN_NAV : WORKSPACE_NAV;
  const connected = mode === "admin" ? Boolean(settings.platformToken) : Boolean(settings.tenantToken);

  return (
    <div className="app-shell">
      <header className="topbar">
        <button className="brand" onClick={() => navigate("dashboard")} aria-label="Biz SaaS 首页">
          <span className="brand-mark">B</span>
          <span>Biz SaaS</span>
        </button>
        <nav className="product-switch" aria-label="系统切换">
          <button className={mode === "admin" ? "active" : ""} onClick={() => switchMode("admin")}>平台管理</button>
          <button className={mode === "workspace" ? "active" : ""} onClick={() => switchMode("workspace")}>租户工作台</button>
        </nav>
        <div className="topbar-spacer" />
        <div className={`connection-state ${connected ? "connected" : ""}`}>
          <span className="connection-dot" />
          {connected ? "已配置凭证" : "待配置凭证"}
        </div>
        <button className="icon-text-button" onClick={() => setSettingsOpen(true)}>
          <Settings size={17} />连接配置
        </button>
      </header>

      <aside className="sidebar">
        <div className="sidebar-context">
          <div className="sidebar-context-icon">{mode === "admin" ? <CircleGauge size={19} /> : <Building2 size={19} />}</div>
          <div>
            <strong>{mode === "admin" ? "Platform Console" : "Tenant Workspace"}</strong>
            <span>{mode === "admin" ? "Control Plane" : "Runtime Plane"}</span>
          </div>
        </div>
        <div className="sidebar-section-label">导航</div>
        <nav className="sidebar-nav">
          {nav.map((item) => {
            const Icon = item.icon;
            const active = !item.disabled && item.id === page;
            return (
              <button
                key={item.id}
                className={`sidebar-item ${active ? "active" : ""} ${item.disabled ? "disabled" : ""}`}
                onClick={() => !item.disabled && navigate(item.id as PageId)}
                title={item.note}
                disabled={item.disabled}
              >
                <Icon size={18} />
                <span>{item.label}</span>
                {item.disabled ? <LockKeyhole size={14} className="sidebar-lock" /> : null}
              </button>
            );
          })}
        </nav>
        <div className="contract-note">
          <ShieldCheck size={17} />
          <div>
            <strong>Contract-first</strong>
            <span>仅开放 biz 当前已有 REST 契约。</span>
          </div>
        </div>
      </aside>

      <main className="main-area">
        <div className="work-tabs">
          <div className="work-tabs-main">
            {tabs.filter((tab) => tab.mode === mode).map((tab) => {
              const active = tab.id === page;
              return (
                <button
                  key={`${tab.mode}-${tab.id}`}
                  className={`work-tab ${active ? "active" : ""}`}
                  onClick={() => setPage(tab.id)}
                >
                  <span className="work-tab-icon">{tab.id === "dashboard" ? "⌂" : "·"}</span>
                  <span className="work-tab-label">{tab.label}</span>
                  {tab.id !== "dashboard" ? (
                    <span
                      role="button"
                      tabIndex={0}
                      aria-label={`关闭 ${tab.label}`}
                      className="work-tab-close"
                      onClick={(event) => {
                        event.stopPropagation();
                        closeTab(tab);
                      }}
                    >
                      <X size={13} />
                    </span>
                  ) : null}
                </button>
              );
            })}
            <button className="work-tab-add" onClick={() => setPage("dashboard")} aria-label="回到工作台"><Plus size={16} /></button>
          </div>
          <div className="work-tabs-tools">
            <button className="work-tabs-tool" onClick={() => setRefreshNonce((value) => value + 1)} aria-label="刷新当前页面">
              <RefreshCw size={16} />
            </button>
          </div>
        </div>

        <div className="page-body">
          {!connected ? (
            <ConnectionRequired mode={mode} onOpen={() => setSettingsOpen(true)} />
          ) : mode === "admin" ? (
            page === "tenants" ? <TenantsPage refreshNonce={refreshNonce} /> : <AdminDashboard refreshNonce={refreshNonce} onOpenTenants={() => navigate("tenants")} />
          ) : page === "members" ? (
            <MembersPage refreshNonce={refreshNonce} />
          ) : page === "roles" ? (
            <RolesPage refreshNonce={refreshNonce} />
          ) : page === "devices" ? (
            <DevicesPage refreshNonce={refreshNonce} />
          ) : (
            <WorkspaceDashboard refreshNonce={refreshNonce} onNavigate={navigate} />
          )}
        </div>
      </main>

      {settingsOpen ? (
        <ConnectionDialog
          initial={settings}
          onClose={() => setSettingsOpen(false)}
          onSave={(next) => {
            saveConnectionSettings(next);
            setSettings(next);
            setSettingsOpen(false);
            setRefreshNonce((value) => value + 1);
          }}
        />
      ) : null}
    </div>
  );
}

function ConnectionRequired({ mode, onOpen }: { mode: Mode; onOpen: () => void }) {
  return (
    <div className="center-state">
      <div className="center-state-icon"><LockKeyhole size={24} /></div>
      <h2>{mode === "admin" ? "需要 Platform Token" : "需要 Tenant Token"}</h2>
      <p>当前页面不会使用模拟数据。配置 Biz Runtime 地址与真实 Bearer Token 后开始读取 `/v1/*`。</p>
      <button className="btn btn-primary" onClick={onOpen}>配置连接</button>
    </div>
  );
}

function AdminDashboard({ refreshNonce, onOpenTenants }: { refreshNonce: number; onOpenTenants: () => void }) {
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try { setTenants(await platformApi.listTenants()); } catch (err) { setError(friendlyError(err)); } finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load, refreshNonce]);

  const count = (leaf: string) => tenants.filter((tenant) => statusLeaf(tenant.status) === leaf).length;
  return (
    <>
      <PageHeader title="平台工作台" description="基于真实 Tenant Lifecycle 数据查看平台租户运行状态。" action={<button className="btn btn-secondary" onClick={onOpenTenants}>进入租户管理</button>} />
      {error ? <ErrorBanner text={error} retry={load} /> : null}
      <div className="metric-grid four">
        <MetricCard label="租户总数" value={loading ? "—" : tenants.length} icon={<Building2 size={23} />} />
        <MetricCard label="Active" value={loading ? "—" : count("ACTIVE")} icon={<ShieldCheck size={23} />} tone="success" />
        <MetricCard label="Pending" value={loading ? "—" : count("PENDING")} icon={<CircleGauge size={23} />} tone="orange" />
        <MetricCard label="Suspended" value={loading ? "—" : count("SUSPENDED")} icon={<LockKeyhole size={23} />} tone="warning" />
      </div>
      <section className="card section-card">
        <div className="section-heading"><div><h2>租户状态</h2><p>当前 API 未提供创建时间与活跃度，页面不伪造这些指标。</p></div></div>
        <DataTable headers={["租户", "状态", "版本"]} loading={loading} empty={tenants.length === 0}>
          {tenants.slice(0, 8).map((tenant) => (
            <tr key={tenant.id}><td><strong>{tenant.name}</strong><small>{tenant.id}</small></td><td><span className={statusClass(tenant.status)}>{statusLeaf(tenant.status)}</span></td><td className="mono">v{tenant.version}</td></tr>
          ))}
        </DataTable>
      </section>
    </>
  );
}

function TenantsPage({ refreshNonce }: { refreshNonce: number }) {
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("ALL");
  const [createOpen, setCreateOpen] = useState(false);
  const [selected, setSelected] = useState<Tenant | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true); setError("");
    try { setTenants(await platformApi.listTenants()); } catch (err) { setError(friendlyError(err)); } finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load, refreshNonce]);

  const filtered = useMemo(() => tenants.filter((tenant) => {
    const matchesQuery = !query || tenant.name.toLowerCase().includes(query.toLowerCase()) || tenant.id.toLowerCase().includes(query.toLowerCase());
    const matchesStatus = status === "ALL" || statusLeaf(tenant.status) === status;
    return matchesQuery && matchesStatus;
  }), [tenants, query, status]);

  const replaceTenant = (next: Tenant) => {
    setTenants((current) => current.map((item) => item.id === next.id ? next : item));
    setSelected(next);
  };
  const mutate = async (action: () => Promise<Tenant>) => {
    setBusy(true); setError("");
    try { replaceTenant(await action()); } catch (err) { setError(friendlyError(err)); } finally { setBusy(false); }
  };

  return (
    <>
      <PageHeader title="租户管理" description="管理 Tenant Lifecycle；状态严格遵循 PENDING / ACTIVE / SUSPENDED / CLOSED。" action={<button className="btn btn-primary" onClick={() => setCreateOpen(true)}><Plus size={16} />创建租户</button>} />
      {error ? <ErrorBanner text={error} retry={load} /> : null}
      <section className="card section-card">
        <div className="table-toolbar">
          <label className="search-control"><Search size={16} /><input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="搜索租户名称 / ID" /></label>
          <select className="select-control" value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="ALL">全部状态</option><option value="PENDING">Pending</option><option value="ACTIVE">Active</option><option value="SUSPENDED">Suspended</option><option value="CLOSED">Closed</option>
          </select>
        </div>
        <DataTable headers={["租户", "状态", "版本", "操作"]} loading={loading} empty={filtered.length === 0}>
          {filtered.map((tenant) => (
            <tr key={tenant.id} className="clickable-row" onClick={() => setSelected(tenant)}>
              <td><strong>{tenant.name}</strong><small>{tenant.id}</small></td>
              <td><span className={statusClass(tenant.status)}>{statusLeaf(tenant.status)}</span></td>
              <td className="mono">v{tenant.version}</td>
              <td><button className="text-button" onClick={(event) => { event.stopPropagation(); setSelected(tenant); }}>查看详情</button></td>
            </tr>
          ))}
        </DataTable>
      </section>
      {createOpen ? <CreateTenantDialog onClose={() => setCreateOpen(false)} onCreated={(tenant) => { setTenants((current) => [tenant, ...current]); setCreateOpen(false); setSelected(tenant); }} /> : null}
      {selected ? <TenantDrawer tenant={selected} busy={busy} onClose={() => setSelected(null)} onUpdate={(name) => mutate(() => platformApi.updateTenant(selected, name))} onActivate={() => mutate(() => platformApi.activateTenant(selected))} onSuspend={() => mutate(() => platformApi.suspendTenant(selected))} onCloseTenant={() => mutate(() => platformApi.closeTenant(selected))} /> : null}
    </>
  );
}

function CreateTenantDialog({ onClose, onCreated }: { onClose: () => void; onCreated: (tenant: Tenant) => void }) {
  const [name, setName] = useState(""); const [ownerUserId, setOwnerUserId] = useState(""); const [ownerEmail, setOwnerEmail] = useState("");
  const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  const submit = async (event: FormEvent) => {
    event.preventDefault(); setBusy(true); setError("");
    try { onCreated(await platformApi.createTenant({ name, ownerUserId, ownerEmail })); } catch (err) { setError(friendlyError(err)); } finally { setBusy(false); }
  };
  return <Modal title="创建租户" description="CreateTenant 会在同一根执行范围内初始化 Owner Member 与 Owner Role。" onClose={onClose}>
    <form onSubmit={submit} className="form-stack">
      {error ? <ErrorBanner text={error} /> : null}
      <Field label="租户名称"><input value={name} onChange={(e) => setName(e.target.value)} required /></Field>
      <Field label="Owner User ID"><input value={ownerUserId} onChange={(e) => setOwnerUserId(e.target.value)} required /></Field>
      <Field label="Owner Email"><input type="email" value={ownerEmail} onChange={(e) => setOwnerEmail(e.target.value)} required /></Field>
      <div className="inline-note"><ShieldCheck size={17} /><span>Owner 初始化由后端原子组合保证，前端不提供关闭选项。</span></div>
      <FormActions onCancel={onClose} busy={busy} submitLabel="创建租户" />
    </form>
  </Modal>;
}

function TenantDrawer({ tenant, busy, onClose, onUpdate, onActivate, onSuspend, onCloseTenant }: { tenant: Tenant; busy: boolean; onClose: () => void; onUpdate: (name: string) => void; onActivate: () => void; onSuspend: () => void; onCloseTenant: () => void }) {
  const [name, setName] = useState(tenant.name);
  useEffect(() => setName(tenant.name), [tenant.name]);
  const leaf = statusLeaf(tenant.status);
  return <Drawer title={tenant.name} subtitle={tenant.id} onClose={onClose}>
    <div className="detail-hero"><span className={statusClass(tenant.status)}>{leaf}</span><span className="mono">Version {tenant.version}</span></div>
    <section className="drawer-section"><h3>基础信息</h3><Field label="租户名称"><input value={name} onChange={(e) => setName(e.target.value)} /></Field><button className="btn btn-secondary" disabled={busy || name === tenant.name} onClick={() => onUpdate(name)}>保存名称</button></section>
    <section className="drawer-section"><h3>生命周期操作</h3><p className="section-copy">动作由当前状态决定，写操作携带最新 version 与 Idempotency-Key。</p><div className="action-cluster">
      {leaf === "PENDING" ? <button className="btn btn-primary" disabled={busy} onClick={onActivate}>激活租户</button> : null}
      {leaf === "ACTIVE" ? <button className="btn btn-secondary" disabled={busy} onClick={onSuspend}>暂停租户</button> : null}
      {leaf === "SUSPENDED" ? <button className="btn btn-primary" disabled={busy} onClick={onActivate}>重新激活</button> : null}
      {leaf !== "CLOSED" ? <button className="btn btn-danger" disabled={busy} onClick={() => window.confirm("关闭租户后将进入 CLOSED 状态，确认继续？") && onCloseTenant()}>关闭租户</button> : null}
    </div></section>
  </Drawer>;
}

function WorkspaceDashboard({ refreshNonce, onNavigate }: { refreshNonce: number; onNavigate: (page: PageId) => void }) {
  const [counts, setCounts] = useState<{ members: number | null; roles: number | null; devices: number | null }>({ members: null, roles: null, devices: null });
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    void Promise.allSettled([tenantApi.listMembers(), tenantApi.listRoles(), tenantApi.listDevices()]).then((results) => {
      if (!active) return;
      const next = {
        members: results[0].status === "fulfilled" ? results[0].value.length : null,
        roles: results[1].status === "fulfilled" ? results[1].value.length : null,
        devices: results[2].status === "fulfilled" ? results[2].value.length : null,
      };
      setCounts(next);
      const failures = results.filter((result) => result.status === "rejected");
      setError(failures.length === results.length ? friendlyError((failures[0] as PromiseRejectedResult).reason) : "");
    });
    return () => { active = false; };
  }, [refreshNonce]);
  return <>
    <PageHeader title="租户工作台" description="当前 Bearer Token 绑定可信租户上下文；Runtime 不接受前端任意指定 tenant_id。" />
    {error ? <ErrorBanner text={error} /> : null}
    <div className="metric-grid three"><MetricCard label="成员" value={counts.members ?? "—"} icon={<Users size={23} />} /><MetricCard label="角色" value={counts.roles ?? "—"} icon={<ShieldCheck size={23} />} tone="orange" /><MetricCard label="设备" value={counts.devices ?? "—"} icon={<Cpu size={23} />} tone="success" /></div>
    <div className="quick-grid">
      <button className="quick-action card" onClick={() => onNavigate("members")}><Users size={21} /><div><strong>成员生命周期</strong><span>Invite / Activate / Suspend / Remove</span></div></button>
      <button className="quick-action card" onClick={() => onNavigate("roles")}><ShieldCheck size={21} /><div><strong>角色与 DataScope</strong><span>Permission Grant 与数据范围一体配置</span></div></button>
      <button className="quick-action card" onClick={() => onNavigate("devices")}><ArrowRightLeft size={21} /><div><strong>DeviceOps</strong><span>设备 CRUD 与跨 Site 转移</span></div></button>
    </div>
  </>;
}

function MembersPage({ refreshNonce }: { refreshNonce: number }) {
  const [members, setMembers] = useState<TenantMember[]>([]); const [loading, setLoading] = useState(true); const [error, setError] = useState("");
  const [query, setQuery] = useState(""); const [filter, setFilter] = useState("ALL"); const [inviteOpen, setInviteOpen] = useState(false); const [busyId, setBusyId] = useState("");
  const load = useCallback(async () => { setLoading(true); setError(""); try { setMembers(await tenantApi.listMembers()); } catch (err) { setError(friendlyError(err)); } finally { setLoading(false); } }, []);
  useEffect(() => { void load(); }, [load, refreshNonce]);
  const filtered = useMemo(() => members.filter((member) => (!query || member.email.toLowerCase().includes(query.toLowerCase()) || member.userId.toLowerCase().includes(query.toLowerCase())) && (filter === "ALL" || statusLeaf(member.status) === filter)), [members, query, filter]);
  const mutate = async (member: TenantMember, action: () => Promise<TenantMember>) => { setBusyId(member.userId); setError(""); try { const next = await action(); setMembers((current) => current.map((item) => item.userId === next.userId ? next : item)); } catch (err) { setError(friendlyError(err)); } finally { setBusyId(""); } };
  return <>
    <PageHeader title="成员管理" description="成员状态来自 Tenant Member Lifecycle；最后 Owner 保护由后端不变量强制执行。" action={<button className="btn btn-primary" onClick={() => setInviteOpen(true)}><Plus size={16} />邀请成员</button>} />
    {error ? <ErrorBanner text={error} retry={load} /> : null}
    <section className="card section-card"><div className="table-toolbar"><label className="search-control"><Search size={16} /><input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="搜索 Email / User ID" /></label><select className="select-control" value={filter} onChange={(e) => setFilter(e.target.value)}><option value="ALL">全部状态</option><option value="INVITED">Invited</option><option value="ACTIVE">Active</option><option value="SUSPENDED">Suspended</option><option value="REMOVED">Removed</option></select></div>
      <DataTable headers={["成员", "状态", "版本", "生命周期操作"]} loading={loading} empty={filtered.length === 0}>{filtered.map((member) => { const leaf = statusLeaf(member.status); const busy = busyId === member.userId; return <tr key={member.userId}><td><strong>{member.email}</strong><small>{member.userId}</small></td><td><span className={statusClass(member.status)}>{leaf}</span></td><td className="mono">v{member.version}</td><td><div className="row-actions">{leaf !== "ACTIVE" && leaf !== "REMOVED" ? <button className="text-button" disabled={busy} onClick={() => mutate(member, () => tenantApi.activateMember(member))}>激活</button> : null}{leaf === "ACTIVE" ? <button className="text-button" disabled={busy} onClick={() => mutate(member, () => tenantApi.suspendMember(member))}>暂停</button> : null}{leaf !== "REMOVED" ? <button className="text-button danger" disabled={busy} onClick={() => window.confirm("确认移除该成员？") && mutate(member, () => tenantApi.removeMember(member))}>移除</button> : null}</div></td></tr>; })}</DataTable>
    </section>
    {inviteOpen ? <InviteMemberDialog onClose={() => setInviteOpen(false)} onInvited={(member) => { setMembers((current) => [member, ...current]); setInviteOpen(false); }} /> : null}
  </>;
}

function InviteMemberDialog({ onClose, onInvited }: { onClose: () => void; onInvited: (member: TenantMember) => void }) {
  const [email, setEmail] = useState(""); const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  const submit = async (event: FormEvent) => { event.preventDefault(); setBusy(true); setError(""); try { onInvited(await tenantApi.inviteMember(email)); } catch (err) { setError(friendlyError(err)); } finally { setBusy(false); } };
  return <Modal title="邀请成员" description="成员会进入 INVITED 状态；同一全局身份可在不同租户拥有独立 Membership。" onClose={onClose}><form onSubmit={submit} className="form-stack">{error ? <ErrorBanner text={error} /> : null}<Field label="Email"><input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoFocus /></Field><FormActions onCancel={onClose} busy={busy} submitLabel="发送邀请" /></form></Modal>;
}

function RolesPage({ refreshNonce }: { refreshNonce: number }) {
  const [roles, setRoles] = useState<TenantRole[]>([]); const [selected, setSelected] = useState<TenantRole | null>(null); const [loading, setLoading] = useState(true); const [error, setError] = useState(""); const [createOpen, setCreateOpen] = useState(false); const [busy, setBusy] = useState(false);
  const load = useCallback(async () => { setLoading(true); setError(""); try { const next = await tenantApi.listRoles(); setRoles(next); setSelected((current) => next.find((role) => role.id === current?.id) || next[0] || null); } catch (err) { setError(friendlyError(err)); } finally { setLoading(false); } }, []);
  useEffect(() => { void load(); }, [load, refreshNonce]);
  const replaceRole = (next: TenantRole) => { setRoles((current) => current.map((role) => role.id === next.id ? next : role)); setSelected(next); };
  const mutate = async (action: () => Promise<TenantRole>) => { setBusy(true); setError(""); try { replaceRole(await action()); } catch (err) { setError(friendlyError(err)); } finally { setBusy(false); } };
  return <>
    <PageHeader title="角色与数据权限" description="Permission 与 DataScope 作为同一 PermissionGrant 管理；当前 Scope：NONE / SELF / SITES / ALL。" action={<button className="btn btn-primary" onClick={() => setCreateOpen(true)}><Plus size={16} />创建角色</button>} />
    {error ? <ErrorBanner text={error} retry={load} /> : null}
    <div className="roles-layout card">
      <aside className="role-list"><div className="role-list-title">角色</div>{loading ? <div className="skeleton-lines" /> : roles.map((role) => <button key={role.id} className={`role-list-item ${selected?.id === role.id ? "active" : ""}`} onClick={() => setSelected(role)}><span><strong>{role.name}</strong><small>{role.id}</small></span><span className={statusClass(role.status)}>{statusLeaf(role.status)}</span></button>)}</aside>
      <div className="role-editor">{selected ? <RoleEditor key={`${selected.id}-${selected.version}`} role={selected} busy={busy} onSaveName={(name) => mutate(() => tenantApi.updateRole(selected, name))} onToggle={() => mutate(() => statusLeaf(selected.status) === "ACTIVE" ? tenantApi.disableRole(selected) : tenantApi.enableRole(selected))} onSavePermissions={(permissions) => mutate(() => tenantApi.setRolePermissions(selected, permissions))} onAssign={(userId) => mutate(() => tenantApi.assignRoleMember(selected.id, userId))} onRevoke={(userId) => mutate(() => tenantApi.revokeRoleMember(selected.id, userId))} /> : <EmptyState title="暂无角色" description="创建角色后配置权限与数据范围。" />}</div>
    </div>
    {createOpen ? <CreateRoleDialog onClose={() => setCreateOpen(false)} onCreated={(role) => { setRoles((current) => [...current, role]); setSelected(role); setCreateOpen(false); }} /> : null}
  </>;
}

function RoleEditor({ role, busy, onSaveName, onToggle, onSavePermissions, onAssign, onRevoke }: { role: TenantRole; busy: boolean; onSaveName: (name: string) => void; onToggle: () => void; onSavePermissions: (permissions: PermissionGrant[]) => void; onAssign: (userId: string) => void; onRevoke: (userId: string) => void }) {
  const [name, setName] = useState(role.name); const [permissions, setPermissions] = useState<PermissionGrant[]>(role.permissions || []); const [userId, setUserId] = useState("");
  const updateGrant = (index: number, patch: Partial<PermissionGrant>) => setPermissions((current) => current.map((grant, i) => i === index ? { ...grant, ...patch } : grant));
  return <div className="role-editor-inner">
    <div className="editor-title"><div><h2>{role.name}</h2><span className={statusClass(role.status)}>{statusLeaf(role.status)}</span></div><button className="btn btn-secondary" disabled={busy} onClick={onToggle}>{statusLeaf(role.status) === "ACTIVE" ? "停用角色" : "启用角色"}</button></div>
    <div className="editor-block"><h3>角色信息</h3><div className="inline-form"><input value={name} onChange={(e) => setName(e.target.value)} /><button className="btn btn-secondary" disabled={busy || name === role.name} onClick={() => onSaveName(name)}>保存名称</button></div></div>
    <div className="editor-block"><div className="editor-block-heading"><div><h3>Permission Grants</h3><p>功能权限与数据范围不可拆离。</p></div><button className="btn btn-secondary" onClick={() => setPermissions((current) => [...current, { permission: "", scope: "DATA_SCOPE_NONE" }])}><Plus size={15} />添加授权</button></div>
      <div className="grant-list">{permissions.length === 0 ? <EmptyState title="暂无权限" description="为角色添加 permission 与 DataScope。" compact /> : permissions.map((grant, index) => <div className="grant-row" key={`${index}-${grant.permission}`}><input value={grant.permission} onChange={(e) => updateGrant(index, { permission: e.target.value })} placeholder="例如 device.read" /><select value={grant.scope} onChange={(e) => updateGrant(index, { scope: e.target.value })}><option value="DATA_SCOPE_NONE">NONE</option><option value="DATA_SCOPE_SELF">SELF</option><option value="DATA_SCOPE_SITES">SITES</option><option value="DATA_SCOPE_ALL">ALL</option></select><button className="icon-button danger" onClick={() => setPermissions((current) => current.filter((_, i) => i !== index))}><X size={16} /></button></div>)}</div>
      <button className="btn btn-primary" disabled={busy || permissions.some((grant) => !grant.permission.trim())} onClick={() => onSavePermissions(permissions)}>保存权限</button>
    </div>
    <div className="editor-block"><h3>角色成员操作</h3><p className="section-copy">当前 TenantRoleDTO 不返回成员列表，因此这里严格按契约提供 User ID 分配 / 撤销操作，不伪造成员关联查询。</p><div className="inline-form"><input value={userId} onChange={(e) => setUserId(e.target.value)} placeholder="User ID" /><button className="btn btn-secondary" disabled={busy || !userId} onClick={() => onAssign(userId)}>分配角色</button><button className="btn btn-danger" disabled={busy || !userId} onClick={() => onRevoke(userId)}>撤销角色</button></div></div>
  </div>;
}

function CreateRoleDialog({ onClose, onCreated }: { onClose: () => void; onCreated: (role: TenantRole) => void }) {
  const [name, setName] = useState(""); const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  const submit = async (event: FormEvent) => { event.preventDefault(); setBusy(true); setError(""); try { onCreated(await tenantApi.createRole(name)); } catch (err) { setError(friendlyError(err)); } finally { setBusy(false); } };
  return <Modal title="创建角色" description="创建后再配置 PermissionGrant。" onClose={onClose}><form onSubmit={submit} className="form-stack">{error ? <ErrorBanner text={error} /> : null}<Field label="角色名称"><input value={name} onChange={(e) => setName(e.target.value)} required autoFocus /></Field><FormActions onCancel={onClose} busy={busy} submitLabel="创建角色" /></form></Modal>;
}

function DevicesPage({ refreshNonce }: { refreshNonce: number }) {
  const [devices, setDevices] = useState<Device[]>([]); const [loading, setLoading] = useState(true); const [error, setError] = useState(""); const [query, setQuery] = useState(""); const [createOpen, setCreateOpen] = useState(false); const [selected, setSelected] = useState<Device | null>(null); const [busy, setBusy] = useState(false);
  const load = useCallback(async () => { setLoading(true); setError(""); try { setDevices(await tenantApi.listDevices()); } catch (err) { setError(friendlyError(err)); } finally { setLoading(false); } }, []);
  useEffect(() => { void load(); }, [load, refreshNonce]);
  const filtered = useMemo(() => devices.filter((device) => !query || device.name.toLowerCase().includes(query.toLowerCase()) || device.serial.toLowerCase().includes(query.toLowerCase()) || device.siteId.toLowerCase().includes(query.toLowerCase())), [devices, query]);
  const replace = (next: Device) => { setDevices((current) => current.map((device) => device.id === next.id ? next : device)); setSelected(next); };
  const mutate = async (action: () => Promise<Device>) => { setBusy(true); setError(""); try { replace(await action()); } catch (err) { setError(friendlyError(err)); } finally { setBusy(false); } };
  const remove = async (device: Device) => { setBusy(true); setError(""); try { await tenantApi.deleteDevice(device); setDevices((current) => current.filter((item) => item.id !== device.id)); setSelected(null); } catch (err) { setError(friendlyError(err)); } finally { setBusy(false); } };
  return <>
    <PageHeader title="设备运营" description="DeviceOps 当前支持 List / Get / Create / Update / Delete / Transfer。" action={<button className="btn btn-primary" onClick={() => setCreateOpen(true)}><Plus size={16} />添加设备</button>} />
    {error ? <ErrorBanner text={error} retry={load} /> : null}
    <section className="card section-card"><div className="table-toolbar"><label className="search-control wide"><Search size={16} /><input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="搜索设备名称 / SN / Site ID" /></label></div><DataTable headers={["设备", "Serial", "Site", "版本", "操作"]} loading={loading} empty={filtered.length === 0}>{filtered.map((device) => <tr key={device.id} className="clickable-row" onClick={() => setSelected(device)}><td><strong>{device.name}</strong><small>{device.id}</small></td><td className="mono">{device.serial}</td><td>{device.siteId}</td><td className="mono">v{device.version}</td><td><button className="text-button" onClick={(event) => { event.stopPropagation(); setSelected(device); }}>查看详情</button></td></tr>)}</DataTable></section>
    {createOpen ? <CreateDeviceDialog onClose={() => setCreateOpen(false)} onCreated={(device) => { setDevices((current) => [device, ...current]); setCreateOpen(false); setSelected(device); }} /> : null}
    {selected ? <DeviceDrawer device={selected} busy={busy} onClose={() => setSelected(null)} onUpdate={(input) => mutate(() => tenantApi.updateDevice(selected, input))} onTransfer={(siteId) => mutate(() => tenantApi.transferDevice(selected, siteId))} onDelete={() => remove(selected)} /> : null}
  </>;
}

function CreateDeviceDialog({ onClose, onCreated }: { onClose: () => void; onCreated: (device: Device) => void }) {
  const [name, setName] = useState(""); const [serial, setSerial] = useState(""); const [siteId, setSiteId] = useState(""); const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  const submit = async (event: FormEvent) => { event.preventDefault(); setBusy(true); setError(""); try { onCreated(await tenantApi.createDevice({ name, serial, siteId })); } catch (err) { setError(friendlyError(err)); } finally { setBusy(false); } };
  return <Modal title="添加设备" description="直接调用 device.create，不生成任何演示设备。" onClose={onClose}><form onSubmit={submit} className="form-stack">{error ? <ErrorBanner text={error} /> : null}<Field label="设备名称"><input value={name} onChange={(e) => setName(e.target.value)} required /></Field><Field label="Serial"><input value={serial} onChange={(e) => setSerial(e.target.value)} required /></Field><Field label="Site ID"><input value={siteId} onChange={(e) => setSiteId(e.target.value)} required /></Field><FormActions onCancel={onClose} busy={busy} submitLabel="添加设备" /></form></Modal>;
}

function DeviceDrawer({ device, busy, onClose, onUpdate, onTransfer, onDelete }: { device: Device; busy: boolean; onClose: () => void; onUpdate: (input: { name: string; siteId: string }) => void; onTransfer: (siteId: string) => void; onDelete: () => void }) {
  const [name, setName] = useState(device.name); const [siteId, setSiteId] = useState(device.siteId); const [targetSiteId, setTargetSiteId] = useState("");
  useEffect(() => { setName(device.name); setSiteId(device.siteId); }, [device.name, device.siteId]);
  return <Drawer title={device.name} subtitle={device.serial} onClose={onClose}><div className="detail-grid"><div><span>Device ID</span><strong className="mono">{device.id}</strong></div><div><span>Created By</span><strong className="mono">{device.createdBy || "—"}</strong></div><div><span>Current Site</span><strong>{device.siteId}</strong></div><div><span>Version</span><strong className="mono">v{device.version}</strong></div></div>
    <section className="drawer-section"><h3>编辑设备</h3><Field label="名称"><input value={name} onChange={(e) => setName(e.target.value)} /></Field><Field label="Site ID"><input value={siteId} onChange={(e) => setSiteId(e.target.value)} /></Field><button className="btn btn-secondary" disabled={busy || (name === device.name && siteId === device.siteId)} onClick={() => onUpdate({ name, siteId })}>保存修改</button></section>
    <section className="drawer-section"><h3>转移设备</h3><p className="section-copy">当前 SiteApplication 没有公开 Site List RPC，因此目标点位使用真实 target_site_id 输入，不伪造下拉数据。</p><div className="inline-form"><input value={targetSiteId} onChange={(e) => setTargetSiteId(e.target.value)} placeholder="Target Site ID" /><button className="btn btn-primary" disabled={busy || !targetSiteId || targetSiteId === device.siteId} onClick={() => onTransfer(targetSiteId)}><ArrowRightLeft size={15} />确认转移</button></div></section>
    <section className="drawer-section danger-zone"><h3>危险操作</h3><button className="btn btn-danger" disabled={busy} onClick={() => window.confirm("确认删除该设备？") && onDelete()}>删除设备</button></section>
  </Drawer>;
}

function ConnectionDialog({ initial, onClose, onSave }: { initial: ConnectionSettings; onClose: () => void; onSave: (settings: ConnectionSettings) => void }) {
  const [settings, setSettings] = useState(initial);
  return <Modal title="连接配置" description="开发阶段凭证仅保存在当前浏览器 Session；生产环境应切换为受控会话 / BFF。" onClose={onClose}><form className="form-stack" onSubmit={(event) => { event.preventDefault(); onSave(settings); }}><Field label="Biz Runtime API Base"><input value={settings.baseUrl} onChange={(e) => setSettings((current) => ({ ...current, baseUrl: e.target.value }))} placeholder="http://127.0.0.1:8080" /></Field><Field label="Platform Token"><input type="password" value={settings.platformToken} onChange={(e) => setSettings((current) => ({ ...current, platformToken: e.target.value }))} placeholder="tenantless platform principal token" /></Field><Field label="Tenant Token"><input type="password" value={settings.tenantToken} onChange={(e) => setSettings((current) => ({ ...current, tenantToken: e.target.value }))} placeholder="tenant-bound runtime token" /></Field><div className="inline-note"><LockKeyhole size={17} /><span>Tenant Context 由 tenant-bound token 建立，不允许前端任意传 tenant_id 作为授权依据。</span></div><FormActions onCancel={onClose} submitLabel="保存连接" /></form></Modal>;
}

function PageHeader({ title, description, action }: { title: string; description: string; action?: React.ReactNode }) {
  return <div className="page-header"><div><h1>{title}</h1><p>{description}</p></div>{action ? <div className="page-actions">{action}</div> : null}</div>;
}

function MetricCard({ label, value, icon, tone = "neutral" }: { label: string; value: string | number; icon: React.ReactNode; tone?: "neutral" | "orange" | "success" | "warning" }) {
  return <div className="metric-card card"><div className={`metric-icon ${tone}`}>{icon}</div><div><span className="metric-label">{label}</span><strong className="metric-value">{value}</strong></div></div>;
}

function DataTable({ headers, children, loading, empty }: { headers: string[]; children: React.ReactNode; loading?: boolean; empty?: boolean }) {
  return <div className="table-scroll"><table className="data-table"><thead><tr>{headers.map((header) => <th key={header}>{header}</th>)}</tr></thead><tbody>{loading ? <tr><td colSpan={headers.length}><div className="table-loading">正在读取真实 API…</div></td></tr> : empty ? <tr><td colSpan={headers.length}><EmptyState title="暂无数据" description="当前接口没有返回记录。" compact /></td></tr> : children}</tbody></table></div>;
}

function ErrorBanner({ text, retry }: { text: string; retry?: () => void | Promise<void> }) {
  return <div className="error-banner"><div><strong>操作未完成</strong><span>{text}</span></div>{retry ? <button className="text-button" onClick={() => void retry()}>重新读取</button> : null}</div>;
}

function EmptyState({ title, description, compact = false }: { title: string; description: string; compact?: boolean }) {
  return <div className={`empty-state ${compact ? "compact" : ""}`}><div className="empty-dot" /><strong>{title}</strong><span>{description}</span></div>;
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="field"><span>{label}</span>{children}</label>;
}

function FormActions({ onCancel, busy = false, submitLabel }: { onCancel: () => void; busy?: boolean; submitLabel: string }) {
  return <div className="form-actions"><button type="button" className="btn btn-secondary" onClick={onCancel}>取消</button><button type="submit" className="btn btn-primary" disabled={busy}>{busy ? "处理中…" : submitLabel}</button></div>;
}

function Modal({ title, description, onClose, children }: { title: string; description?: string; onClose: () => void; children: React.ReactNode }) {
  return <div className="overlay" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}><div className="modal" role="dialog" aria-modal="true"><div className="modal-header"><div><h2>{title}</h2>{description ? <p>{description}</p> : null}</div><button className="icon-button" onClick={onClose} aria-label="关闭"><X size={18} /></button></div><div className="modal-body">{children}</div></div></div>;
}

function Drawer({ title, subtitle, onClose, children }: { title: string; subtitle?: string; onClose: () => void; children: React.ReactNode }) {
  return <div className="overlay drawer-overlay" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}><aside className="drawer" role="dialog" aria-modal="true"><div className="modal-header"><div><h2>{title}</h2>{subtitle ? <p>{subtitle}</p> : null}</div><button className="icon-button" onClick={onClose} aria-label="关闭"><X size={18} /></button></div><div className="drawer-body">{children}</div></aside></div>;
}

const root = document.getElementById("root");
if (!root) throw new Error("#root not found");
createRoot(root).render(<React.StrictMode><App /></React.StrictMode>);
