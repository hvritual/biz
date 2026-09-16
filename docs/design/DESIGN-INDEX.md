# 源码派生的组件与模式检索

关联 #142；维护边界见 [Design System Runtime](DESIGN-SYSTEM-RUNTIME.md)。索引仅帮助找到源码和使用场景，不执行页面、不授予权限、不推断后端上线状态。

## 使用

```sh
cd web
npm run design:index
npm run design:find -- --query '成员详情' --kind component --limit 5
npm run design:find -- --query 'tenant status' --domain platform --json
npm run check:design-index
npm run test:design
```

生成位置为被 Git 忽略的 `.cache/design-index.json`，不进入前端 bundle。`design:find` 只读，不会在查询时偷偷生成或修改源码；没有索引或来源摘要已变化时明确失败。先修改原始来源，再重新生成。`check:design-index` 在干净环境可生成；已有过期缓存必须先报告过期，不能边重建边掩盖漂移。

## 输入与事实边界

真实 Vue/TypeScript 提供组件名、导入路径、Props/Emits/Slots 声明和实际组件组合关系。`ui-contracts.json` 提供页面义务和模式实例，`component-scopes.json` 只补充使用场景、范围、别名、useWhen/avoidWhen、状态及示例路径。禁止在元数据中手工定义另一个 Props/API。同名组件通过完整源码路径 ID 区分。

解析器支持现有项目采用的编译宏、内联/本地类型、运行时声明、`withDefaults`、`defineModel` 和模板插槽；外部类型的内部形状、无法解析的宏参数和动态插槽保留 unknown 及源码位置，不让模型猜测。类型文本可以包含导入的领域类型名，这不是已递归展开的 JSON schema。

组件依赖和消费者由可见模板组合图生成，包含间接组合；不能将其解释为所有潜在运行分支或服务端依赖。静态分析范围继承 #141，动态条件是否可见仍需浏览器测试。场景元数据没有批准状态时默认 `needs-review`，源文件存在不自动升级为生产可用；Pattern 的 implemented 只表示有当前页面组合实例。

## 检索行为

先用实际组件名、中文业务词或英文别名检索，再根据 kind/layer/domain/status 收窄。结果返回匹配原因、实际导入路径、使用限制、来源摘要和源码片段。精确别名优先于弱词片段，结果排序稳定；无匹配返回 gap，不新建组件。查询最长256字符、结果上限50，优先小范围检索而非把全站索引放进模型上下文。

当前使用确定性词和标签检索，没有向量库、网络服务或外部上传。语义相似的建议不是 API 兼容证明；生成前必须读取候选源码、真实消费者及其测试。

## 验证

`npm run check` 已接入索引验证和设计测试。源文件、合同、元数据、解析器和依赖锁进入来源指纹；不包含生成时间等随机值。同样输入两次生成必须相同。缺失组件/示例、过期元数据、手工 API 覆盖、陈旧或篡改索引都需非零失败。缓存无效不影响 Vue 页面运行。

本任务首次实测索引扫描到73个组件、40个页面、4个模式。这是固定候选的观察，不是以后必须维持的数量。无真实消费者的 MetricsPage 保留 reserved。组件增加/删减应由源码和测试解释，不能为了符合数字创建占位组件。

新增业务组件为0；新增服务为0；未改应用权限、业务行为、主题 authority 或组件公共 API。后续 #146 才验证 AI 页面使用的实际收益，不能用“索引条数”冒充开发效率提升。
