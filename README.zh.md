# 2build

[English](README.md) | [Tiếng Việt](README.vi.md) | 中文 | [日本語](README.ja.md) | [한국어](README.ko.md)

![2build — 从计划到经过验证的交接。概念插图。](docs/assets/2build-banner.jpg)

<img src="assets/2build-mascot-transparent.png" alt="2build 吉祥物：戴安全帽的橙色小工程师，手持带勾选标记的积木。" width="128" height="128" align="right">

**先看原型，剩下的交给 Autopilot。**

**[从 Autopilot 开始](#安装) · [阅读文档（英文）](docs/install.md)**

2build 帮助 coding agent 完成一个功能。你提出需求、预览界面，再让 **Autopilot** 编写代码、检查并修复问题。

小型界面任务的目标是在 **约 5 分钟** 内看到原型；实际时间取决于任务、模型和项目。

- **质量：** 交付前审查代码、运行测试并修复问题。
- **生产力：** Agent 自己完成各个步骤，无需你不断提醒。
- **效率：** 提前调整原型，减少返工。

适用于 **Claude Code、Codex、Antigravity、OMP 和 Grok**，以及任何支持 Claude Code skills 的 coding agent。

2found 出品。前身为 babysit；`bbs` CLI、`bbs:` skill 和 `.babysit` 状态保持兼容。参见[品牌与命名](BRANDING.md)（英文）。

<a id="安装"></a>

## 用一个 prompt 安装

把下面的 prompt 粘贴到能运行终端命令的 coding agent：

```text
为我当前使用的 coding agent 安装 2build。
按照 https://raw.githubusercontent.com/2found/2build/main/docs/install.md 操作。
识别操作系统和当前 agent，复用可用的 bbs 或安装 CLI，然后只为此 agent 安装 skill pack。
验证 bbs --version 和已安装的 skills；缺少前提条件时明确报告，不要宣称成功。
说明是否需要重启，并给出适合当前 agent 的准确 Autopilot 调用方式，让我运行第一个小任务。
```

需要受支持的 coding agent 及其现有 model 访问权限。Claude Code 和 Codex 还需要 CLI 在 PATH 中。2build 无需单独的 model 账户；agent 的正常使用费用仍适用。**只有 Foreman 需要 Orca。** 参见[安装与排错](docs/install.md)（英文）。

<details>
<summary>想手动运行命令？macOS 或 Linux 上的 Homebrew</summary>

```bash
brew tap 2found/2build https://github.com/2found/2build
brew install 2found/2build/bbs
bbs install
```

`bbs install` 自动检测 Claude Code、Codex 和 Antigravity。OMP、Grok 和其他兼容 agent 请参见 [skill 安装指南](docs/install.md#omp-grok-and-other-compatible-agents)。也可用 `bbs install claude`、`bbs install codex` 或 `bbs install antigravity` 选择一个。安装后重启 agent。没有 Homebrew 时使用 [release archive](docs/install.md#release-archives-macos-or-linux)；Windows 使用 WSL。

</details>

## 开始新项目

[**2build starters**](https://github.com/2found/2build-starters) 提供版本化的项目模板，
包含 agent 工作环境、架构指南、测试和本地 QA。首个模板是 `hono-bun`：
使用 TypeScript 和 Zod 的 Bun/Hono API。

`bbs bootstrap` 和 `bbs starter check` 已在 CLI 1.95.0 及以上版本提供。Starter
稳定版可直接从 GitHub 下载。命令和前提条件请参阅
[快速开始指南](docs/starters.md)。

<a id="autopilot单个-ticket"></a>

## 运行第一个 ticket

1. 重启 agent，打开 Git repo，选择一个有明确成功检查的小 bug 或功能。Autopilot 在当前 checkout 上工作并 commit；需要隔离时先创建自己的 branch。
2. 在 **agent chat 中调用 skill**，将示例换成你的任务：

   | Agent | 示例 |
   |-------|------|
   | Claude Code | `/bbs:autopilot "添加已保存搜索的页面。先展示原型，再实现、审查和 QA。"` |
   | Codex | `$bbs:autopilot "添加已保存搜索的页面。先展示原型，再实现、审查和 QA。"` |
   | OMP | `/autopilot "添加已保存搜索的页面。先展示原型，再实现、审查和 QA。"` |
   | Grok | `/bbs:autopilot "添加已保存搜索的页面。先展示原型，再实现、审查和 QA。"` |
   | Antigravity / 其他兼容 agent | 请它为任务使用已安装的 `autopilot` skill。 |

3. 审阅计划；对于 UI 任务，打开原型，在此调整布局、交互或范围。Autopilot 返回 `/goal` block 时，将其粘贴到同一 agent 以启动执行。对于没有 goal mode 的 agent，在首次请求中加入 `--stop-after=plan`，确保停在此审阅检查点，然后按 handoff 中的原生指令恢复。
4. 让 Autopilot 实现、审查、修复、测试并运行 QA。通过完成 gate 后，它返回本地 commit 和证据；被阻塞时会指出缺少的输入。重启后，用 handoff 中的 ticket ID 恢复。

第一个 ticket 无需 Orca、新 worker session 或项目配置。开始前选好 session 的 model。[查看进度和恢复 session](docs/companion-cli.md)。

## 与 2found 继续构建

产品需要更多能力时，按下一步选择工具：

| 下一步 | 探索 |
|--------|------|
| 为产品添加 AI agent | [**Soot**](https://trysoot.com)，由 **2agent** 提供支持 — **agent as config** — 在源代码中加一份配置，多一位 AI 伙伴。 |
| 部署已构建的产品 | [**2server**](https://github.com/2found/2server) — 在你拥有的基础设施上部署和运营应用。 |
| 让客户发现产品 | [**2market**](https://2found.dev/#2market) — 正在开发的营销工作空间，将产品背景、内容和渠道整合到一个流程中。 |

从 2build 开始产品工程工作，在有需要时探索这些产品。

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

<details>
<summary>CLI 与配置详情</summary>

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

</details>

## 文档（英文）

[安装与排错](docs/install.md) · [选择 skill](docs/skills.md) · [查看进度](docs/companion-cli.md) · [协调项目](docs/foreman.md)

对于 coding agent，[llms.txt](llms.txt) 直接链接到 Markdown 指南和 runtime 约定。

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
