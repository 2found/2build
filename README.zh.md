# babysit

[English](README.md) | [Tiếng Việt](README.vi.md) | 中文 | [日本語](README.ja.md) | [한국어](README.ko.md)

**给它一个目标。你离开时，它会规划、实现、审查并验证。**

Babysit 是一套 agent skill，加上配套的 Go CLI。单个串行 ticket 用 Autopilot；项目包含相互依赖的 ticket、需要监督 worker 时用 Foreman。Babysit 的 skill 按产品团队的五种工作角色组织：Prototyper、Builder、Sweeper、Grower、Maintainer，根据工作内容选择，而非文件类型。参见[角色说明](.claude/skills/references/archetypes.md)。

## 安装

安装 CLI，然后让 Babysit 自动检测并配置已安装的 harness：

```bash
brew install lohi-ai/babysit/bbs
bbs install
```

安装后重启 agent。`bbs install` 支持 Claude Code、Codex 和 Antigravity；Claude Code 和 Codex 的 CLI 必须在 PATH 中。也可单独运行 `bbs install claude`、`bbs install codex` 或 `bbs install antigravity`。参见[安装详情](docs/install.md)，其中也介绍了 Linux 软件包。

要开发 Babysit 本身，请克隆仓库并运行 `go run ./cmd/bbs setup --full`；它会构建 `bbs` 并输出将当前 checkout 注册为本地插件的命令。`bbs update` 更新 CLI 和已安装的插件。

## 配置 Foreman

Foreman 需要启用了 orchestration 的 [Orca](https://www.onorca.dev)。它依次根据显式选择、兼容的阶段固定配置或目标主机上的 Orca 默认配置选择 worker 的 agent。Babysit 旧 YAML 中的 agent/provider/model/effort 偏好已弃用，不再生效。

Babysit 根据任务复杂度（`simple`、`normal` 或 `hard`）和阶段类别选择 worker 的 model 与 effort。规划、设计和审查属于 `critical` 阶段；实现、QA 和交付属于 `normal` 阶段。策略将这些组合映射到 `flash`、`pro` 或 `max` 层级，每个层级为所选 agent 配有对应的 model。显式的阶段覆盖配置和有效的已保存恢复路由优先。

在项目目录中查看生效的策略，或查询一次具体选择：

```bash
bbs foreman model --json
bbs foreman model --agent codex --complexity normal --phase-class critical --json
```

这些查询不需要 ticket 或 Orca 连接。添加 `--dir <repo-or-worktree>` 可查看其他项目的策略。在 `~/.babysit/settings.json` 或 `<repo>/.babysit/settings.json` 的 `foreman.models` 下覆盖单个字段；仓库配置优先于全局配置，全局配置优先于内置默认值。例如，将 `hard` 任务的 `normal` 阶段路由到 `max` 层级：

```json
{
  "foreman": {
    "models": {
      "routing": {
        "hard": { "normal": "max" }
      }
    }
  }
}
```

各 agent 的 model/effort 绑定和恢复行为见 [model routing](.claude/skills/foreman/references/model-routing.md#model-tiers)。

## Foreman：多 ticket 项目

在 agent 中调用 **Foreman skill**（以下是 skill 调用，不是 `bbs` CLI 命令）：

```text
# Claude Code
/bbs:foreman "重建横跨 web 和 API 的请求流程"
/bbs:foreman --auto "重建横跨 web 和 API 的请求流程"

# Codex
$bbs:foreman "重建横跨 web 和 API 的请求流程"
```

Foreman 初始化或恢复父项目，并绑定对应的 Orca Run。规划/设计 worker 首先创建总体计划、原型（或非 UI 的流程/接口设计），以及明确范围和依赖关系的稳定 ticket manifest。默认情况下，你先审阅这些产物和拟定的 ticket，之后才会创建子 ticket、worktree 或派发正式实现工作。显式指定 `--auto` 会将这次审查交给独立的证据检查 worker，并记录批准结果；它不会绕过安全暂停或后续 QA。

批准后，已接受的方案转为具有明确依赖边的 ticket DAG。Foreman 只派发已就绪的 ticket，遵守 worker/资源限制，并确保每个 ticket worktree 只有一个写入 worker。每个 ticket 依次经历独立且范围明确的 Plan、Implement、Review 和 QA worker 阶段。路由依据任务复杂度和阶段，或单次派发的显式选择。

Foreman 通过 Orca 等待 worker 的报告、问题或升级请求；它不会轮询终端，也不会启动重试计时器。进入下一阶段前，它会检查当前阶段的产物、修订版本和 verdict。ticket 通过各项 gate 后，由配置的收尾策略（`review`、`land` 或 `pr`）决定交付方式。Foreman 验证回执，释放已结束的 worker 和租约，关闭自己拥有的 Orca 界面，并仅删除符合条件的干净 worktree，同时保留 branch。

项目收尾同样由 worker 执行：审计和获授权的交付/清理 worker 先完成工作，再由只读的最终 QA worker 检查准确的已交付 base 或保留的 QA 组合。只有最终证据、清理和就绪检查全部通过，Foreman 才会完成父项目。相互影响的 ticket 还会在 land 前接受集成 QA；它不能替代项目最终 QA。

## Autopilot：单个 ticket

为单个 ticket 调用 **Autopilot skill**：

```text
# Claude Code skill，在你已打开的 session 中调用
/bbs:autopilot "添加深色模式开关"
```

独立运行的 Autopilot 在你启动的 session 中规划、实现、审查和 QA。它不会另选 model，也不会中途切换 model；请在开始前选好。在 Codex 中使用 `$bbs:autopilot` 调用。它无需 Orca，也不会打开后台 worker session。

## Skill 与 CLI

Skill 是面向 agent 的工作流；`bbs` 是它们使用的配套 CLI。`/bbs:foreman` 运行项目协调器；`bbs foreman` 管理 model 策略查询、持久化的 Foreman 记录、契约和报告。`/bbs:autopilot` 运行单 ticket 工作流；`bbs autopilot` 提供 checkpoint 和状态辅助命令。`bbs ticket` 管理 ticket 身份、DAG 关系、证据、测试环境、交付和清理。

常用 CLI 入口：

```bash
bbs dashboard
bbs foreman report <parent-ticket>
bbs ticket dag <parent-ticket>
bbs autopilot snapshot --json
bbs autopilot recover --json
```

`snapshot` 读取 ticket 的权威状态和 gate 证据；`recover` 额外提供长度受限的产物摘录，以便恢复工作。

运行 `bbs <subcommand> --help` 查看用法。更多详情：[配套 CLI](docs/companion-cli.md)、[profile](docs/profiles.md) 和[运维](docs/operations.md)。

## 仓库结构

- `bbs` — 多命令二进制文件（构建产物由 gitignore 忽略；`go build -o bbs ./cmd/bbs`）。Hook 是编译后的子命令：`bbs hooks <name>`。
- `.claude/skills/` — agent skill 和工作流参考资料。
- `internal/` — Go CLI 和服务。
- `web/` — dashboard SPA，嵌入发布构建。
- `tests/`、`docs/` — 验证套件和用户文档。

## 遥测

Skill 使用情况以 JSONL 格式记录在本地 `~/.babysit/analytics/` 下；遥测是无人值守运行的主要反馈渠道。

## 许可证

MIT。
