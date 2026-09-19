# 企业中心一期 Requirement → Issue → Contract → Acceptance 映射

本表覆盖 US-001～US-051、FR-1～FR-183、Q-001～Q-020。区间为闭区间；架构测试会展开区间并验证无遗漏、无重复和未知编号。

## User Stories

| Range | Owner issues | Contract / UI | Acceptance |
| --- | --- | --- | --- |
| US-001..US-001 | #169 | privacy consent + login precondition | 首次提示、拒绝、版本记录、浏览器 |
| US-002..US-004 | #170 #171 #172 | account/contact/password/OTP session entry | 密码/验证码、单/多企业、锁定 |
| US-005..US-006 | #171 | tenant switch/logout/session revocation | 越权切换、退出旧会话拒绝 |
| US-007..US-007 | #170 #173 | recovery OTP + one-time authorization + password rotate | 重放/过期/弱密码/旧会话 |
| US-008..US-008 | #174 | current-user authorization aggregate | user/tenant/module/button current facts |
| US-009..US-011 | #175 | canonical routes + menu/button/deep-link consumption | 401/403、无闪现、API反绕过 |
| US-012..US-014 | #168 #176 | protected identity/contact + member query/create/update | 服务端筛选分页、原子创建、脱敏 |
| US-015..US-018 | #173 #177 | member lifecycle/recovery/admin recovery | owner保护、软删除、重登 |
| US-019..US-020 | #170 #177 #180 | business scope + recovery appeal | 可分配范围、限流、通知 |
| US-021..US-023 | #173 #181 | self profile/avatar/password | self-only、脱敏、会话撤销 |
| US-024..US-027 | #170 #182 #183 | contact rotation/preferences/account exit | OTP一次消费、tenant隔离 |
| US-028..US-028 | #168 | field encryption/masking/log hygiene | DB/响应/日志/浏览器扫描 |
| US-029..US-032 | #178 | role query/profile/default protection/delete | 默认角色和有成员阻塞 |
| US-033..US-035 | #174 #179 | action catalog + role menu/button/member grants | 半选/CAS/即时撤权 |
| US-036..US-037 | #179 #180 | effective Data Policy ref + current authorization | 策略有效性、撤权下一请求拒绝 |
| US-038..US-039 | #185 #186 | in-app unread + mark-all-read | tenant/user隔离与并发新增 |
| US-040..US-044 | #184 | message type/channel/config | 过滤、唯一性、权限、事务 |
| US-045..US-045 | #183 #184 #185 | event→config→preference→delivery | outbox、重试、站内记录 |
| US-046..US-049 | #171 #174 | trusted session + request-time authorization | 401/403、停用、跨租户、恒通过反例 |
| US-050..US-050 | #187 | service request HMAC/replay boundary | timestamp/nonce/body digest |
| US-051..US-051 | #174 | tenant-bound resource authorization | request tenant cannot override principal |

## Functional Requirements

| Range | Owner issues | Notes |
| --- | --- | --- |
| FR-1..FR-4 | #169 | privacy/terms consent authority |
| FR-5..FR-14 | #168 #171 #172 | login identifier, credential, tenant selection |
| FR-15..FR-19 | #170 | OTP send/exchange/consume |
| FR-20..FR-24 | #171 | tenant switch/session/logout |
| FR-25..FR-33 | #170 #173 | password recovery |
| FR-34..FR-37 | #174 | current authorization aggregate |
| FR-38..FR-44 | #175 | menu/button/deep-link frontend enforcement |
| FR-45..FR-66 | #168 #176 | member query/create/edit |
| FR-67..FR-78 | #173 #177 | member state/reset/delete/recovery |
| FR-79..FR-83 | #170 #177 #180 | business scope/audit/recovery appeal |
| FR-84..FR-92 | #173 #181 | self profile/avatar/password |
| FR-93..FR-98 | #170 #182 | phone/email rotation |
| FR-99..FR-102 | #183 | personal notification preference |
| FR-103..FR-107 | #170 #171 #182 | tenant-account exit |
| FR-108..FR-112 | #168 | field encryption/password slow hash/log masking |
| FR-113..FR-122 | #178 | role query/profile/default/delete protection |
| FR-123..FR-127 | #174 #179 | module/button trees and grant differences |
| FR-128..FR-128 | #167 #174 #179 | SUPERSEDED: no Gateway allow index; request-time current Access facts |
| FR-129..FR-131 | #178 #180 | role members + effective Data Policy refs |
| FR-132..FR-132 | #167 #174 #179 | SUPERSEDED: no global Authorization Version; resource/policy CAS remains |
| FR-133..FR-133 | #179 | permission contraction effect |
| FR-134..FR-137 | #186 | in-app unread/mark-all-read |
| FR-138..FR-155 | #184 | message catalog/config/channels |
| FR-156..FR-158 | #185 | notification generation/preferences/in-app |
| FR-159..FR-173 | #171 #174 | token/session/current authorization boundary |
| FR-174..FR-177 | #187 | HMAC/timestamp/nonce/replay |
| FR-178..FR-180 | #174 #188 | tenant override rejection/trace/audit denial |
| FR-181..FR-183 | #184 | message configuration operation permissions |

## Open Questions

| Range | Decision authority | Implementation owners |
| --- | --- | --- |
| Q-001..Q-004 | `decisions.md` / Human | #168 #170 #171 #172 |
| Q-005..Q-008 | `decisions.md` / Human | #169 #173 #176 #177 #182 #183 |
| Q-009..Q-012 | `decisions.md` / Human | #171 #178 #179 #180 |
| Q-013..Q-016 | `decisions.md` / Human | #184 #185 #186 |
| Q-017..Q-019 | `decisions.md` / Human | #167 #187 |
| Q-020..Q-020 | actual acceptance owners | #192 |

## Cross-cutting qualification

- #188：扩展现有 server audit 覆盖新增身份/权限/消息事件。
- #189：US-001～US-045 的同一真实环境全流程联调、i18n、视觉与可访问性收口。
- #190：身份/隐私/授权/消息数据迁移、对账、重跑与回滚演练。
- #191：US-001～US-051、FR-1～FR-183、成功指标与安全反例的固定候选验收。
- #192：真实批准、密钥/渠道/监控/运行手册与生产发布资格。

## Existing commercial / tenant lifecycle boundaries

#112 及 #114～#137 的租赁多租户、客户额度、套餐购买、营销计费路线保持独立；本路线只消费已存在 entitlement/quota/session 能力，不复制商业事实、客户 Tenant 或支付状态机。品牌 #107、设计系统路线同理只消费不重建。
