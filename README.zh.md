# babysit

[English](README.md) | [Tiếng Việt](README.vi.md) | 中文 | [日本語](README.ja.md) | [한국어](README.ko.md)

**给 coding agent 一个目标，拿回经过审查和测试的变更。**

Babysit 是面向 Claude Code、Codex 和 Antigravity 的开源 skill pack，配套 CLI 保存进度和验证证据。从 **Autopilot** 开始：在你现有的 agent session 中，将一个 ticket 从需求推进到本地 commit。

<a id="安装"></a>

## 用一个 prompt 安装

把下面的 prompt 粘贴到能运行终端命令的 coding agent：

```text
为我当前使用的 coding agent 安装 Babysit。
按照 https://raw.githubusercontent.com/lohi-ai/babysit/main/docs/install.md 操作。
识别操作系统和当前 agent，复用可用的 bbs 或安装 CLI，然后只为此 agent 安装 skill pack。
验证 bbs --version 和已安装的 plugin；缺少前提条件时明确报告，不要宣称成功。
说明是否需要重启，并给出适合当前 agent 的准确 Autopilot 调用方式，让我运行第一个小任务。
```

需要受支持的 coding agent 及其现有 model 访问权限。Claude Code 和 Codex 还需要 CLI 在 PATH 中。Babysit 无需单独的 model 账户；agent 的正常使用费用仍适用。**只有 Foreman 需要 Orca。** 参见[安装与排错](docs/install.md)（英文）。

<details>
<summary>想手动运行命令？macOS 或 Linux 上的 Homebrew</summary>

```bash
brew tap lohi-ai/babysit https://github.com/lohi-ai/babysit
brew install lohi-ai/babysit/bbs
bbs install
```

`bbs install` 为所有检测到的受支持 agent 安装。也可用 `bbs install claude`、`bbs install codex` 或 `bbs install antigravity` 选择一个。安装后重启 agent。没有 Homebrew 时使用 [release archive](docs/install.md#release-archives-macos-or-linux)；Windows 使用 WSL。

</details>

## Babysit 有何不同？

Prompt 能描述如何构建。Babysit 加上 workflow 和磁盘状态，将目标推进到审查和验证，即使 session 重启也能继续。

| 你需要 | Babysit 提供 |
|--------|-------------|
| 无需逐步指挥也能完成任务 | Autopilot 将一个 ticket 推进过规划、实现、审查修复和 QA。 |
| crash 或 context reset 后继续 | 需求、计划、checkpoint 和 handoff 保存在磁盘，可据此恢复。 |
| 知道结果是否经过检查 | 持久保存 review/QA verdict；完成要求当前检查通过且无未解决的重大问题。 |
| 控制交付 | 独立 Autopilot 只做本地 commit；你查看证据后再 push 或创建 PR。 |

适用于需要经过验证的交接，或希望离开时 agent 继续工作的任务。简单修改可能只需 coding agent。Babysit 仍需要可用的项目测试环境；缺少访问权限或必需检查时会报告 `NEEDS_CONTEXT` 或 `BLOCKED`。

<a id="autopilot单个-ticket"></a>

## 运行第一个 ticket

1. 重启 agent，打开 Git repo，选择一个有明确成功检查的小 bug 或功能。Autopilot 在当前 checkout 上工作并 commit；需要隔离时先创建自己的 branch。
2. 在 **agent chat 中调用 skill**，将示例换成你的任务：

   | Agent | 示例 |
   |-------|------|
   | Claude Code | `/bbs:autopilot "修复搜索结果为空时的状态，并添加回归测试"` |
   | Codex | `$bbs:autopilot "修复搜索结果为空时的状态，并添加回归测试"` |
   | Antigravity | 请它为任务使用已安装的 `autopilot` skill。 |

3. Autopilot 返回计划和 `/goal` block 时，审阅计划，再将 block 粘贴到同一 agent 以开始构建。没有 goal mode 的 agent 会在当前 session 中继续。
4. 预期结果：本地 commit、review/QA 证据，以及说明改动和检查的 handoff。被阻塞时会指出缺口。重启后把 handoff 中的 ticket ID 交给 Autopilot 即可恢复。

第一个 ticket 无需 Orca、新 worker session 或项目配置。开始前选好 session 的 model。[查看进度和恢复 session](docs/companion-cli.md)。

## 按工作内容选择

Autopilot 将任务路由到产品团队的五种角色之一。也可指定 workflow 或直接调用 skill。

| 角色 | 工作 |
|------|------|
| Prototyper | 在投入生产代码前验证风险假设。 |
| Builder | 将功能或 bug 修复推进到 review 和 QA。 |
| Sweeper | 移除冗余或优化已测量的热点，保持行为。 |
| Grower | 改善文案、转化或可衡量的增长实验。 |
| Maintainer | 诊断故障，强化可靠性、安全性和依赖。 |

在 [skill 索引](docs/skills.md)中查阅独立 skill 和 workflow 示例。

<a id="配置-foreman"></a>
<a id="foreman多-ticket-项目"></a>

## 更大的项目：Foreman

对于存在依赖的多个 ticket，[Foreman](docs/foreman.md) 通过 [Orca](https://www.onorca.dev) 规划项目、用 worktree 隔离子 ticket、调度就绪任务并执行项目级 QA。默认先由你审阅父项目计划/设计，再派发生产工作；显式 `--auto` 将该审阅委托给 worker。配置的 finish policy 控制交付。

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
