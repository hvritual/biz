# CE-13 平台商业 Web Session 边界

## 决策

CE-13 平台管理控制台使用 CE-12 已建立的 first-party OIDC + BFF durable web session，不在浏览器保存、生成或转发平台 API key。

平台商业 HTTP operation 只有在同时满足以下条件时允许 `AUTHENTICATION_WEB_SESSION`：

1. operation 是 CE-13 人工平台控制台的直接交互入口；
2. operation 继续使用既有 `platform.*` 权限做服务端授权；
3. `tenant_required=false` 的平台主体仍必须由 durable session 的 platform identity 产生，不能从请求头推断；
4. unsafe HTTP 请求继续经过 CE-12 CSRF 校验；
5. 已有 API-key 认证继续保留，供受控平台自动化使用。

内部 composition、provisioning worker、默认订阅规则和非 CE-13 交互入口不因本决策获得 web-session。

## 安全不变量

- Tenant user session 不能因为知道平台 URL 或伪造 tenant/principal/platform header 成为 platform principal。
- Platform session 每次授权继续回读当前 platform credential / permission grant；权限撤销后不能依赖会话中的历史权限继续访问。
- Web session provenance 保持 `web-session`，不得重分类为 `api-key`。
- 浏览器代码不得包含平台 API key、API key fallback 或失败后回退为模拟授权数据。
- 新增 web-session 的 operation 集由 `TestCE13PlatformCommercialWebSessionContract` 严格白名单锁定。

## 非目标

本决策不实现 CE-13 Vue 页面、不完成 CE-14 tenant self-service、不改变商业领域事务语义，也不扩大 provisioning worker 的浏览器访问能力。
