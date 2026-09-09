# CE-08 框架阻塞最小复现

对应 [执行回执](../evidence/CE-08.md)、[机器记录](../evidence/CE-08-blocker-reproduction.json) 与 [yunka.io#181](https://github.com/hvritual/yunka.io/issues/181)。

## 用途与限制

[fixture](ce08_cross_domain_cycle_test.go.txt) 在真实框架生成器中构造三个 Application：Commercial entitlement -> Access tenant -> Commercial subscription。应用图无环，生成 domain/application 包形成循环。第二个测试移除 entitlement 反向域边作为单向对照。两种输入均实际编译 Operation plans、两次确定性生成并检查 Go loader。

**诊断测试通过 = 已复现缺陷，不是业务验收通过。** 隔离 loader 模块仅提供 DTO 声明，不提供外部 runtime 依赖；因此不是完整生成代码编译测试。另一次未经修改 biz candidate 的 `go test ./...` 退出 1，是实际消费者失败证据。复现不使用 fake business service、数据库 mock 或授权 fallback。

文件采用 `.go.txt` 避免参与 biz 构建；不要复制到正式框架分支或手改已生成的 zz_yunka 文件。

## 可重复命令

使用一次性 checkout，按框架 `tools/toolchain.env` 准备工具链；`BIZ_SOURCE` 为含本 fixture 的 biz checkout，`FRAMEWORK_SOURCE` 为一次性框架 checkout 的绝对路径：

```sh
set -euo pipefail
: "${BIZ_SOURCE:?set absolute biz checkout path}"
: "${FRAMEWORK_SOURCE:?set disposable framework checkout path}"
cd "$FRAMEWORK_SOURCE"
# 固定以下任一个 SHA；两个 SHA 均已由记录中的 workflow 运行。
git checkout --detach 6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb
# 另一对照快照：d6840be86c72ea91b51e7f32f8ed61067bcab643
source tools/toolchain.env
test "$(go env GOVERSION)" = "go${GO_VERSION}"
test ! -e pkg/contract/ce08_blocker_probe_test.go
cp "$BIZ_SOURCE/docs/commercial-entitlements/repro/ce08_cross_domain_cycle_test.go.txt" pkg/contract/ce08_blocker_probe_test.go
trap 'rm -f "$FRAMEWORK_SOURCE/pkg/contract/ce08_blocker_probe_test.go"' EXIT
export CE08_REPRO_OUT="$(mktemp -d)"
cd pkg
go test ./contract -run '^TestCE08Repro' -count=1 -v
```

CI 等价入口为仓库 `.github/workflows/ce08-blocker-repro.yml`，仅只读权限，固定框架/候选 SHA，制品含日志、生成内容和源码归档。业务候选单独 checkout 为框架的兄弟目录，保证原 `../yunka.io` 工作区拓扑，不改 go.mod/go.work 绕过失败。

## 修复后的测试策略

保留这些固定旧 SHA 的历史诊断。框架修复 PR 应新增面向“修复后生成代码实际编译成功”的回归，而不是把历史预期改成通配成功。未声明 child、权限、根事务、内部 operation 的传输隔离以及真正应用/操作环拒绝都须保持。

不得将本目录诊断结果写入 CE-08 的 DONE verification。CE-08 的规则、并发、幂等、所有 child 故障原子性与 MySQL/B12 业务回归仍是独立验收要求。
