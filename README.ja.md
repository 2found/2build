# 2build

[English](README.md) | [Tiếng Việt](README.vi.md) | [中文](README.zh.md) | 日本語 | [한국어](README.ko.md)

![2build — 計画から検証済みの引き継ぎまで。コンセプトイラスト。](docs/assets/2build-banner.jpg)

<img src="assets/2build-mascot-transparent.png" alt="2build マスコット：ヘルメットをかぶり、チェック付きのブロックを持つオレンジ色のビルダー。" width="128" height="128" align="right">

**プロトタイプを確認。あとは Autopilot に任せる。**

**[Autopilot で始める](#インストール) · [ドキュメントを読む（英語）](docs/install.md)**

2build は coding agent が機能を完成させるためのツールです。要件を伝え、プロトタイプを確認したら、**Autopilot** にコードの作成、検証、修正を任せられます。

小さな画面の開発では **約 5 分** でプロトタイプを確認することを目指します。所要時間はタスク、モデル、プロジェクトによって変わります。

- **品質：** 引き渡し前にコードをレビューし、テストして問題を修正します。
- **生産性：** 各工程は agent が進めるので、逐一指示する必要はありません。
- **効率：** プロトタイプの段階で調整し、作り直しを減らします。

**Claude Code、Codex、Antigravity、OMP、Grok** と、Claude Code skills に対応するすべての coding agent で使えます。

2found の製品です。旧名は babysit。`bbs` CLI、`bbs:` skill、`.babysit` の状態は互換性を維持します。[ブランドと命名](BRANDING.md)（英語）を参照してください。

<a id="インストール"></a>

## 1 つの prompt でインストール

ターミナルを実行できる coding agent に、次を貼り付けてください：

```text
今使っている coding agent に 2build をインストールしてください。
https://raw.githubusercontent.com/2found/2build/main/docs/install.md に従い、OS と現在の agent を確認し、動作する bbs があれば再利用、なければ CLI をインストールして、この agent だけに skill pack を導入してください。
bbs --version とインストール済み skills を検証し、前提条件が不足していれば成功とせず報告してください。
再起動が必要かを説明し、最初の小さなタスクを実行するための、この agent 用の正確な Autopilot 呼び出しを教えてください。
```

対応 coding agent と、その既存の model 利用環境が必要です。Claude Code と Codex は CLI が PATH 上に必要です。2build 用の model アカウントは不要ですが、agent の通常の利用料金は発生します。**Orca が必要なのは Foreman だけです。** [インストールとトラブル対処](docs/install.md)（英語）を参照してください。

<details>
<summary>コマンドで導入する場合：macOS / Linux の Homebrew</summary>

```bash
brew tap 2found/2build https://github.com/2found/2build
brew install 2found/2build/bbs
bbs install
```

`bbs install` は Claude Code、Codex、Antigravity を自動検出します。OMP、Grok、その他の対応 agent は [skill 導入ガイド](docs/install.md#omp-grok-and-other-compatible-agents)を参照してください。1 つだけなら `bbs install claude`、`bbs install codex`、`bbs install antigravity` を使います。導入後は agent を再起動してください。Homebrew がない場合は [release archive](docs/install.md#release-archives-macos-or-linux)、Windows では WSL を使います。

</details>

## 新しいプロジェクトを始める

[**2build starters**](https://github.com/2found/2build-starters) は、agent 向けの作業環境、
アーキテクチャガイド、テスト、ローカル QA を備えたバージョン付きプロジェクトテンプレートです。
最初のテンプレートは `hono-bun`：TypeScript と Zod を使う Bun/Hono API です。

`bbs bootstrap` と `bbs starter check` は CLI 1.95.0 以降で利用できます。Starter の安定版を
GitHub から直接ダウンロードできます。コマンドと前提条件は
[クイックスタート](docs/starters.md)を参照してください。

<a id="autopilot1-つの-ticket"></a>

## 最初の ticket を実行

1. Agent を再起動し、Git repo を開き、成功条件が明確な小さな bug や機能を選びます。Autopilot は現在の checkout で作業・commit します。分離したい場合は先に branch を作成してください。
2. **Agent chat で skill を呼び出し**、例を自分のタスクに置き換えます：

   | Agent | 例 |
   |-------|----|
   | Claude Code | `/bbs:autopilot "保存した検索の画面を追加する。先にプロトタイプを見せ、その後に実装、レビュー、QAを行う。"` |
   | Codex | `$bbs:autopilot "保存した検索の画面を追加する。先にプロトタイプを見せ、その後に実装、レビュー、QAを行う。"` |
   | OMP | `/autopilot "保存した検索の画面を追加する。先にプロトタイプを見せ、その後に実装、レビュー、QAを行う。"` |
   | Grok | `/bbs:autopilot "保存した検索の画面を追加する。先にプロトタイプを見せ、その後に実装、レビュー、QAを行う。"` |
   | Antigravity / その他の対応 agent | 導入済みの `autopilot` skill を使ってタスクを進めるよう依頼します。 |

3. 計画を読み、UI のタスクではプロトタイプを開きます。ここでレイアウト、操作の流れ、範囲を調整してください。Autopilot が `/goal` block を返したら、同じ agent に貼り付けて実行を始めます。Goal mode がない agent では、最初の依頼に `--stop-after=plan` を追加してこの確認段階で停止させ、handoff の再開指示に従ってください。
4. 実装、レビュー、修正、テスト、QA を Autopilot に任せます。完了 gate を通過するとローカル commit と証拠を返します。阻害要因があれば不足を明示します。再起動後は handoff の ticket ID で再開できます。

最初の ticket に Orca、新しい worker session、プロジェクト設定は不要です。開始前に session の model を選んでください。[進捗確認と session 復旧](docs/companion-cli.md)。

## 2found と次のステップへ

製品に必要なものが増えたら、次の仕事に合うツールを選べます：

| 次にしたいこと | 探す |
|----------------|------|
| 製品に AI agent を追加する | [**Soot**](https://trysoot.com)、powered by **2agent** — **agent as config** — ソースコードに設定を追加。AI の仲間が増えます。 |
| 作った製品をデプロイする | [**2server**](https://github.com/2found/2server) — 自分のインフラ上でアプリをデプロイ・運用。 |
| 顧客に製品を見つけてもらう | [**2market**](https://2found.dev/#2market) — 開発中のマーケティング用ワークスペース。製品の背景、コンテンツ、チャネルを 1 つの流れにつなぎます。 |

製品開発は 2build から始め、必要になったときにこれらの製品を検討してください。

## 仕事の内容で選ぶ

Autopilot はタスクを製品チームの 5 つの役割に振り分けます。Workflow の指定や skill の直接呼び出しも可能です。

| 役割 | 仕事 |
|------|------|
| Prototyper | 本番コードに投資する前にリスクのある仮説を検証。 |
| Builder | 機能や bug 修正を review と QA まで完了。 |
| Sweeper | 動作を保ちながら冗長な部分を削除、測定済みのホットパスを改善。 |
| Grower | 文案、コンバージョン、測定可能な成長実験を改善。 |
| Maintainer | 障害の原因を調べ、信頼性・セキュリティ・依存関係を強化。 |

個別 skill と workflow の例は [skill 一覧](docs/skills.md)を参照してください。

<a id="foreman-の設定"></a>
<a id="foreman複数-ticket-のプロジェクト"></a>

## 大きなプロジェクト：Foreman

依存する複数 ticket には [Foreman](docs/foreman.md) を使います。[Orca](https://www.onorca.dev) 経由で全体計画、子 ticket の worktree 分離、実行可能な作業の dispatch、プロジェクト全体の QA を行います。既定では本番作業の dispatch 前に親計画・設計を確認します。明示的な `--auto` はその確認を worker に委任します。設定済み finish policy が納品を決めます。

<details>
<summary>CLI と設定の詳細</summary>

## Skill と CLI

Skill は agent 向けのワークフローで、`bbs` はそれを支える CLI。`/bbs:foreman` はプロジェクトの調整役を実行し、`bbs foreman` は model ポリシーの照会、永続的な Foreman 記録、契約、報告を扱う。`/bbs:autopilot` は単一 ticket のワークフローを実行し、`bbs autopilot` は checkpoint と状態の補助コマンドを提供する。`bbs ticket` は ticket の識別、DAG 関係、証拠、テスト環境、引き渡し、クリーンアップを管理する。

よく使う CLI コマンド：

```bash
bbs dashboard
bbs foreman report <parent-ticket>
bbs ticket dag <parent-ticket>
bbs autopilot snapshot --json
bbs autopilot recover --json
```

`snapshot` は ticket の正規状態と gate の証拠を読み取る。`recover` は再開に使う、分量を制限した成果物の抜粋を追加する。

使い方は `bbs <subcommand> --help` で確認できる。詳細は[付属 CLI](docs/companion-cli.md)、[profile](docs/profiles.md)、[運用](docs/operations.md)を参照。

</details>

## ドキュメント（英語）

[導入とトラブル対処](docs/install.md) · [Skill を選ぶ](docs/skills.md) · [進捗を確認する](docs/companion-cli.md) · [プロジェクトを調整する](docs/foreman.md)

Coding agent 向けには、[llms.txt](llms.txt) から Markdown ガイドと runtime の規約を直接参照できます。

## リポジトリ構成

- `bbs` — 複数のコマンドを扱うバイナリ（build 出力は gitignore 対象。`go build -o bbs ./cmd/bbs`）。hook はコンパイル済みのサブコマンド `bbs hooks <name>`。
- `.claude/skills/` — agent skill とワークフローの参考資料。
- `internal/` — Go CLI とサービス。
- `web/` — dashboard SPA。リリース build に埋め込まれる。
- `tests/`、`docs/` — 検証スイートとユーザー向け文書。

## テレメトリ

Skill の利用状況は `~/.babysit/analytics/` に JSONL 形式でローカル記録される。テレメトリは無人実行の主なフィードバック経路となる。

## ライセンス

MIT。
