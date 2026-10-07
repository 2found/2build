# 2build

[English](README.md) | [Tiếng Việt](README.vi.md) | [中文](README.zh.md) | 日本語 | [한국어](README.ko.md)

**Coding agent に目標を渡す。レビューとテストを経た変更を受け取る。**

2build は Claude Code、Codex、Antigravity 向けのオープンソース skill pack です。付属 CLI が進捗と検証の証拠を保存します。まずは **Autopilot**：いつもの agent session で、1 つの ticket を要件からローカル commit まで進めます。

<a id="インストール"></a>

## 1 つの prompt でインストール

ターミナルを実行できる coding agent に、次を貼り付けてください：

```text
今使っている coding agent に 2build をインストールしてください。
https://raw.githubusercontent.com/2found/2build/main/docs/install.md に従い、OS と現在の agent を確認し、動作する bbs があれば再利用、なければ CLI をインストールして、この agent だけに skill pack を導入してください。
bbs --version とインストール済み plugin を検証し、前提条件が不足していれば成功とせず報告してください。
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

`bbs install` は検出した対応 agent すべてに導入します。1 つだけなら `bbs install claude`、`bbs install codex`、`bbs install antigravity` を使います。導入後は agent を再起動してください。Homebrew がない場合は [release archive](docs/install.md#release-archives-macos-or-linux)、Windows では WSL を使います。

</details>

## 2build の違い

Prompt は作り方を説明できます。2build は workflow とディスク上の状態を加え、目標からレビュー・検証まで進めます。Session を再起動しても再開できます。

| 必要なこと | 2build が提供すること |
|------------|-----------------------|
| 毎ステップ指示せずにタスクを終える | Autopilot が 1 ticket の計画、実装、レビュー修正、QA を進めます。 |
| Crash や context reset から再開する | 要件、計画、checkpoint、handoff をディスクに保存し、そこから復元します。 |
| 結果が検証されたか確認する | Review / QA verdict を保存し、完了には最新チェックの成功と重大な未解決指摘がないことを要求します。 |
| 納品を自分で管理する | 単独 Autopilot はローカル commit まで。証拠を確認してから push や PR 作成を行います。 |

検証を伴う引き継ぎが必要なタスクや、席を離れている間の作業に向きます。小さな修正なら coding agent だけで足りる場合もあります。プロジェクトのテスト環境は必要です。アクセスや必須チェックが不足すると `NEEDS_CONTEXT` / `BLOCKED` で報告します。

<a id="autopilot1-つの-ticket"></a>

## 最初の ticket を実行

1. Agent を再起動し、Git repo を開き、成功条件が明確な小さな bug や機能を選びます。Autopilot は現在の checkout で作業・commit します。分離したい場合は先に branch を作成してください。
2. **Agent chat で skill を呼び出し**、例を自分のタスクに置き換えます：

   | Agent | 例 |
   |-------|----|
   | Claude Code | `/bbs:autopilot "検索結果が空の状態を修正し、回帰テストを追加する"` |
   | Codex | `$bbs:autopilot "検索結果が空の状態を修正し、回帰テストを追加する"` |
   | Antigravity | 導入済みの `autopilot` skill を使ってタスクを進めるよう依頼します。 |

3. 計画と `/goal` block が返されたら計画を確認し、同じ agent に block を貼り付けて build を開始します。Goal mode のない agent では、その session で続行します。
4. 結果はローカル commit、review / QA の証拠、変更とチェックを記した handoff です。停止した場合は不足が報告されます。再起動後は handoff の ticket ID を Autopilot に渡して再開します。

最初の ticket に Orca、新しい worker session、プロジェクト設定は不要です。開始前に session の model を選んでください。[進捗確認と session 復旧](docs/companion-cli.md)。

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
