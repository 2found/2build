# babysit

[English](README.md) | [Tiếng Việt](README.vi.md) | [中文](README.zh.md) | 日本語 | [한국어](README.ko.md)

**目標を渡す。席を離れている間に、計画、実装、レビュー、検証まで進める。**

Babysit は agent 向けの skill パックと、それを支える Go CLI。直列で進める 1 つの ticket には Autopilot、依存関係のある複数の ticket と worker の監督が必要なプロジェクトには Foreman を使う。skill は製品開発チームの 5 つの役割 — Prototyper、Builder、Sweeper、Grower、Maintainer — に沿って構成され、ファイルの種類ではなく作業内容で選ぶ。[役割の説明](.claude/skills/references/archetypes.md)を参照。

## インストール

CLI と agent プラグインをそれぞれインストールする：

```bash
brew install lohi-ai/babysit/bbs

# Claude Code
claude plugin marketplace add lohi-ai/babysit
claude plugin install bbs@babysit

# Codex CLI
codex plugin marketplace add lohi-ai/babysit
codex plugin add bbs@babysit
```

インストール後は agent を再起動する。プラグインは skill を提供する。`bbs` は skill とローカル hook が使う必須の CLI で、プラグインには同梱されていない。Linux パッケージを含む[インストールの詳細](docs/install.md)を参照。

Babysit 自体を開発する場合は、リポジトリを clone して `go run ./cmd/bbs setup --full` を実行する。`bbs` を build し、checkout 内の skill をリンクする。`bbs update` は CLI とインストール済みプラグインを更新する。

## Foreman の設定

Foreman には orchestration を有効にした [Orca](https://www.onorca.dev) が必要。worker の agent は、明示的な指定、互換性のあるフェーズ固定設定、実行先ホストの Orca デフォルト設定の順で選ぶ。Babysit の旧 YAML にある agent/provider/model/effort 設定は廃止され、無視される。

Babysit はタスクの複雑度（`simple`、`normal`、`hard`）とフェーズ区分から worker の model と effort を選ぶ。計画、設計、レビューは `critical`、実装、QA、引き渡しは `normal` フェーズに分類される。ポリシーはこの組み合わせを `flash`、`pro`、`max` の tier に対応づけ、各 tier が選択された agent 用の model 設定を持つ。明示的なフェーズ別の上書き設定と、再開用に保存された有効なルートが優先される。

プロジェクトのディレクトリで、有効なポリシー全体または個別の選択結果を確認する：

```bash
bbs foreman model --json
bbs foreman model --agent codex --complexity normal --phase-class critical --json
```

この照会には ticket も Orca 接続も不要。別のプロジェクトを調べるには `--dir <repo-or-worktree>` を追加する。`~/.babysit/settings.json` または `<repo>/.babysit/settings.json` の `foreman.models` でフィールド単位の上書きができる。リポジトリ設定、グローバル設定、組み込みデフォルトの順に優先される。例えば、`hard` タスクの `normal` フェーズを `max` tier に割り当てるには：

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

agent ごとの model/effort 設定と再開時の動作は [model routing](.claude/skills/foreman/references/model-routing.md#model-tiers) を参照。

## Foreman：複数 ticket のプロジェクト

agent 内で **Foreman skill** を呼び出す（以下は skill の呼び出し例であり、`bbs` CLI コマンドではない）：

```text
# Claude Code
/bbs:foreman "web と API にまたがるリクエスト処理フローを作り直す"
/bbs:foreman --auto "web と API にまたがるリクエスト処理フローを作り直す"

# Codex
$bbs:foreman "web と API にまたがるリクエスト処理フローを作り直す"
```

Foreman は親プロジェクトを初期化または再開し、対応する Orca Run を関連づける。計画・設計 worker が最初に、全体計画、prototype（UI のない作業ではフローやインターフェースの設計）、スコープと依存関係を定めた安定した ticket manifest を作る。デフォルトでは、子 ticket や worktree の作成、本実装の割り当てに進む前に、これらの成果物と ticket 案を人がレビューする。明示的に `--auto` を指定すると、そのレビューを証拠を確認する別の worker に委ね、承認を記録する。安全上の保留や後続の QA は省略しない。

承認後、受け入れた案を明示的な依存関係を持つ ticket DAG に変換する。Foreman は worker・リソース上限の範囲で準備のできた ticket だけを割り当て、各 ticket worktree の書き込み担当を 1 worker に保つ。各 ticket は、範囲を限定した個別の Plan、Implement、Review、QA の worker フェーズを進む。ルーティングにはタスクの複雑度とフェーズ、または割り当てごとの明示的な指定を使う。

Foreman は Orca を通じて worker の報告、質問、エスカレーションを待つ。terminal のポーリングや再試行タイマーの起動はしない。先に進む前に、各フェーズの成果物、リビジョン、verdict を確認する。ticket が gate を通過すると、設定された終了ポリシー（`review`、`land`、`pr`）に従って引き渡す。Foreman は実行結果の受領記録を検証し、終了した worker とリースを解放し、自身が所有する Orca の画面を閉じる。削除するのは条件を満たす clean な worktree だけで、branch は保持する。

プロジェクトの完了処理も worker が担う。監査と、許可された引き渡し・クリーンアップの worker が先に終了し、その後、読み取り専用の最終 QA worker が、実際に引き渡された base または保持された QA 用の統合状態を確認する。最終証拠、クリーンアップ、準備状態の確認がすべて通過して初めて、Foreman は親プロジェクトを完了する。相互に影響する ticket には land 前の統合 QA も行うが、これはプロジェクト最終 QA の代わりにはならない。

## Autopilot：1 つの ticket

単一 ticket には **Autopilot skill** を呼び出す：

```text
# 開いている session 内で Claude Code skill を呼び出す
/bbs:autopilot "ダークモードの切り替えを追加する"
```

単独の Autopilot は、起動した session 内で計画、実装、レビュー、QA を行う。新たな model を選んだり、途中で model を切り替えたりはしない。開始前に model を選ぶ。Codex では `$bbs:autopilot` として呼び出す。Orca なしで動作し、バックグラウンドの worker session は開かない。

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
