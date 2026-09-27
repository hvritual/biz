# 身份生命周期：真实 HTTP 入口到可靠投递

关联 #185 / PR #277。开工候选 `3be0d116125f9cebddbd4d0c5532cd39bc664b4a`，源码树 `381ad52d963dfec061e1064c64645b5ca004555c`。

## 本轮边界

在既有生成 HTTP、IdP、BFF 入口发起真实请求，经应用/根事务、受保护 outbox 和 #185 SecurityDeliveryWorker 核验结果。数据库使用既有 CI 串行 MySQL；最末端仍为明确标注的 MemorySender，既不声称真实邮件/SMS供应商已接通，也不把其 DELIVERED 当作外部供应商的送达证明。

本轮不新增公共 API、第二 Executor 或新身份来源；不修改生成合同、CI Runner/Guard/Workflow、框架锁或 #186 UI。测试中的用户、角色和会话初始化属于 fixture；测试中的业务操作通过实际 HTTP 发起，不能用 fixture 写入的生命周期通知代替业务入口。

## 已定位的误触发

`UpdateTenantMemberProfile` 调用 `mutate(..., lifecycleEvent=false, ...)`，但原实现忽略这个标记，无条件调用 `notifyTenantMemberLifecycle`。所以普通资料编辑也会成为安全生命周期消息。本轮只在 `lifecycleEvent=true` 时调用通知；真实 Activate/Suspend 与独立 Remove/Restore 路径保留通知及原事务。

## 入口与验收映射

| 测试入口 | 验证链与反例 |
|---|---|
| PATCH /v1/tenant/members/{id}/profile | 资料实际落库，不生成安全生命周期 outbox，不调用 provider |
| POST /v1/tenant/members/{id}/suspend /activate /remove /restore | 真实 Action、版本变更、tenant/user/status/version 稳定事件、worker 投递、重放不增加事件；A 操作不改变共享账号在 B 的关系 |
| 只读角色 POST 成员四种生命周期动作 | 真实读取目标成功，但所有写动作 403；成员状态/版本不变，outbox 与 provider 均无副作用 |
| Suspend 的 outbox insert 故障 | HTTP 失败，版本和状态及事件均回滚；移除注入后可继续正常操作 |
| POST /v1/tenant/members/create | 激活链接及短信初始凭据分别真实创建；返回 PENDING 不含凭据；worker 取得正确身份和保护材料；投递完成销毁密文 |
| POST /idp/member/activate | 通过 test transport 收到的真实激活链接完成 HTTP 激活，重放拒绝；SMS 初始密码仍要求强制更改 |
| GET /idp/authorize → POST /idp/login | 真实浏览器 cookie/CSRF 与密码失败达到锁定；一个锁定事件，请求路径零 provider 调用，worker 后续投递 |
| GET /idp/password/recovery → request → complete | 真实 cookie、表单 nonce、worker 收到的 OTP；缺 cookie 拒绝；新旧凭据结果核验；完成事件不含 Secret；重放不再次通知 |
| POST /auth/personal/contact-change/request /complete | 真实 BFF 会话/CSRF、验证码 worker、当前企业资料变更及完成消息 |
| POST /auth/personal/tenant-deletion/request /complete | 当前租户 OTP 与不可逆确认，成员资料擦除和会话撤销；完成通知仍可使用事先保护的目标地址，投递后销毁密文 |
| POST /auth/member-appeals | 拒绝客户端 user_id；仅目标企业负责人获得通知；重复申诉限流；投递不恢复申请人权限 |

所有新增测试追加到 `scripts/ci_access_tests.json` 的 notification-delivery 套件，原有16个顶层测试保持不变。测试不输出完整 OTP、激活链接、密码或联系方式；只比对值并输出失配分类。每个场景检验当前 outbox 和当前测试 sender，而不是引用旧绿色回执。

## 仍需单独完成

- `ResetPasswordWithAuthorization` 是已有持久化方法，当前没有据此找到可供“管理员重置”验收的独立生成 HTTP/Action 入口；不能将用户自主密码恢复等同于管理员重置完成。
- 登录锁定通知当前仍在登录失败提交之后由 handler 尝试入队；本轮能证明正常 HTTP 到 outbox，不声称已消除登录锁定与入队之间的故障窗口。
- IdP 自动消费目前仍依赖 qualification sender；普通业务路由/external worker 的生产运行期启动、可配置渠道、provider callback/认证/终态尚需独立闭环。
- 站内60秒及时率≥99.9%的批量统计、最终 Full Gate 与 MAIN_VERIFIED 仍不属于本轮局部入口测试通过的结论。

实际候选、运行编号和原始日志摘要写入 PR / Issue 完成回执；本文定义可复核的覆盖范围，不自证未执行的测试成功。
