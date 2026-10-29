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

需要受支持的 coding agent 及其现有 model 访问权限。Claude Code 和 Codex 还需要 CLI 在 PATH 中。2build 无需单独的 model 账户；agent 的正常使用费用仍适用。**只有 Team 需要 Orca。** 参见[安装与排错](docs/install.md)（英文）。

<details>
<summary>想手动运行命令？macOS 或 Linux 上的 Homebrew</summary>

```bash
brew tap 2found/2build https://github.com/2found/2build
brew install 2found/2build/bbs
bbs install
```

`bbs install` 自动检测 Claude Code、Codex、Antigravity 和 OMP。Grok 和其他兼容 agent 请参见 [skill 安装指南](docs/install.md#grok-and-other-compatible-agents)。也可用 `bbs install claude`、`bbs install codex`、`bbs install antigravity` 或 `bbs install omp` 选择一个。安装后重启 agent。没有 Homebrew 时使用 [release archive](docs/install.md#release-archives-macos-or-linux)；Windows 使用 WSL。

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

1. 重启 agent，打开 Git repo，选择一个有明确成功检查的小 bug 或功能。Autopilot 在当前 checkout 上工作，并让改动保持未 commit 状态；需要隔离时先创建自己的 branch。
2. 在 **agent chat 中调用 skill**，将示例换成你的任务：

   | Agent | 示例 |
   |-------|------|
   | Claude Code | `/bbs:autopilot "添加已保存搜索的页面。先展示原型，再实现、审查和 QA。"` |
   | Codex | `$bbs:autopilot "添加已保存搜索的页面。先展示原型，再实现、审查和 QA。"` |
   | OMP | `/autopilot "添加已保存搜索的页面。先展示原型，再实现、审查和 QA。"` |
   | Grok | `/bbs:autopilot "添加已保存搜索的页面。先展示原型，再实现、审查和 QA。"` |
   | Antigravity / 其他兼容 agent | 请它为任务使用已安装的 `autopilot` skill。 |

3. 审阅计划；对于 UI 任务，打开原型，在此调整布局、交互或范围。Autopilot 返回 `/goal` block 时，将其粘贴到同一 agent 以启动执行。对于没有 goal mode 的 agent，在首次请求中加入 `--stop-after=plan`，确保停在此审阅检查点，然后按 handoff 中的原生指令恢复。若要无人值守运行，加上 `--auto`（例如 `/bbs:autopilot --auto "…"`）：Autopilot 自行审阅计划，不输出 `/goal` block；agent 允许时自行设定 goal，否则在同一 session 中继续工作，直到 ticket 完成或被阻塞。
4. 让 Autopilot 实现、审查、修复、测试并运行 QA。通过完成 gate 后，它把已验证的改动以未 commit 状态留在你的 checkout 中并附上证据（由你 commit 或交给 `create-pr`）；被阻塞时会指出缺少的输入。重启后，用 handoff 中的 ticket ID 恢复。

第一个 ticket 无需 Orca、新 worker session 或项目配置。开始前选好 session 的 model。[查看进度和恢复 session](docs/companion-cli.md)。

## 与 2found 继续构建

产品需要更多能力时，按下一步选择工具：

| 下一步 | 探索 |
|--------|------|
| 把重复性工作交给一支小型 agent 团队 | [**Soot**](https://trysoot.com)，由 **2agent** 提供支持 — 按计划回复访客、关注信息源、撰写内容、跟进联系，只在需要人来处理时才询问你。 |
| 部署已构建的产品 | [**2server**](https://github.com/2found/2server) — 在你拥有的基础设施上部署和运营应用。 |
| 让客户发现产品 | [**2market**](https://2found.dev/#2market) — 正在开发的营销工作空间，将产品背景、内容和渠道整合到一个流程中。 |

从 2build 开始产品工程工作，在有需要时探索这些产品。

## 按工作内容选择

`/bbs:autopilot` 在一个 session 中把一项工作(bug 修复、功能,或需要一整天的长变更)推进到 plan、review 和 QA。从这里开始:一个长 session 让 context 保持温热,token 消耗最少。只有需要单个 session 给不了的东西时,才把整个项目交给 `/bbs:team`([见下文](#team))。专项工作可直接调用 skill。

在 [skill 索引](docs/skills.md)中查阅独立 skill 和 workflow 示例。

<a id="配置-foreman"></a>
<a id="foreman多-ticket-项目"></a>

<a id="team"></a>

## Team:承接整个项目的产品团队

[Team](docs/team.md) 像你雇来的一支产品团队。你只与它碰面两次:

1. **委托** — 你给出需求,确认团队写回的 spec(问题一次问完,之后不再提问),并审批 basic design:架构、整个产品的 prototype 和 release phase。
2. **交付** — 你连同证据一起审阅完成的产品,然后批准 release 或给出反馈。spec 范围内的反馈由团队自行修复后重新交付,无需再开会;超出 spec 的需求会作为 design 变更回到你这里。

在这两次之间,团队自行完成 design、build、review、QA 和 integrate(每个 phase 是本地 base 上的一个提交),不会问你任何问题。worker 从不提交,Team 需要 [Orca](https://www.onorca.dev)。`/bbs:foreman` 仍可作为 alias 使用。

它比 Autopilot 更贵(一个 lead session、一个 design session,以及每个 phase 一个 integration session),所以要有理由才用:

- **你想放手** — 只参与两次而不是驾驶一个 session,最后拿到已 review、可 release 的产品。
- **按 phase 发布** — 在审批 design 时允许的分阶段生产发布,而不是最后一次性上线大改动。
- **独立验证** — 由一个新 session review 和 QA 每个 phase;Autopilot 在写代码的同一个 session 里 review。
- **并行处理互不相交的部分** — 通过冻结的 contract 连接的独立 service 或 repo,在等待时间重要时并行推进。

如果都不符合,单次 Autopilot 运行(哪怕要一整天)更快也更省。

<details>
<summary>CLI 与配置详情</summary>

## Skill 与 CLI

Skill 是面向 agent 的工作流；`bbs` 是它们使用的配套 CLI。`/bbs:team` 运行产品团队；`bbs team`(alias `bbs foreman`)管理 model 策略查询、持久化的 lead 记录、phase（`seal`、`integrate`、`land`、`release`）、下一步建议和报告；`bbs contract` 管理并行 ticket 所依赖的接口契约。`/bbs:autopilot` 运行单 ticket 工作流；`bbs autopilot` 提供 checkpoint 和状态辅助命令。`bbs ticket` 管理 ticket 身份、DAG 关系、证据、测试环境租约和接收已 seal 的工作。

常用 CLI 入口：

```bash
bbs dashboard
bbs team report <parent-ticket>
bbs team next <parent-ticket>
bbs ticket dag <parent-ticket>
bbs autopilot snapshot --json
bbs autopilot recover --json
```

`snapshot` 读取 ticket 的权威状态和 gate 证据；`recover` 额外提供长度受限的产物摘录，以便恢复工作。

运行 `bbs <subcommand> --help` 查看用法。更多详情：[配套 CLI](docs/companion-cli.md)、[git flow](.claude/skills/references/git-flow.md) 和[运维](docs/operations.md)。

## 语义决策

`semantic-decision` 将任务规模、复杂度、路由、测试影响和审查判断变成显式、可复用的 skill 步骤。默认 provider 是当前 LLM，无需配置，也无需额外的 model 进程。要使用 Cloudflare：

```bash
bbs config set semantic_decision_provider cloudflare
bbs config set semantic_decision_model clef-flash  # or clef
```

在环境变量或 `~/.babysit/.env` 中设置 `CLOUDFLARE_ACCOUNT_ID` 和 `CLOUDFLARE_API_TOKEN`。只有用户配置能选择 provider；项目策略可以限制外部推理。Cloudflare 出错或回答不确定时，回退到当前 LLM。参见 [skill](.claude/skills/semantic-decision/SKILL.md) 和[契约](.claude/skills/references/semantic-decision.md)。配置路径、默认值、项目限制和人工审查行为见[没有 dashboard 控件的设置](docs/operations.md#settings-without-a-dashboard-control)。

</details>

## 文档（英文）

[安装与排错](docs/install.md) · [选择 skill](docs/skills.md) · [查看进度](docs/companion-cli.md) · [协调项目](docs/team.md)

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
