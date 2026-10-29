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

対応 coding agent と、その既存の model 利用環境が必要です。Claude Code と Codex は CLI が PATH 上に必要です。2build 用の model アカウントは不要ですが、agent の通常の利用料金は発生します。**Orca が必要なのは Team だけです。** [インストールとトラブル対処](docs/install.md)（英語）を参照してください。

<details>
<summary>コマンドで導入する場合：macOS / Linux の Homebrew</summary>

```bash
brew tap 2found/2build https://github.com/2found/2build
brew install 2found/2build/bbs
bbs install
```

`bbs install` は Claude Code、Codex、Antigravity、OMP を自動検出します。Grok とその他の対応 agent は [skill 導入ガイド](docs/install.md#grok-and-other-compatible-agents)を参照してください。1 つだけなら `bbs install claude`、`bbs install codex`、`bbs install antigravity`、`bbs install omp` を使います。導入後は agent を再起動してください。Homebrew がない場合は [release archive](docs/install.md#release-archives-macos-or-linux)、Windows では WSL を使います。

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

1. Agent を再起動し、Git repo を開き、成功条件が明確な小さな bug や機能を選びます。Autopilot は現在の checkout で作業し、変更は未 commit のまま残します。分離したい場合は先に branch を作成してください。
2. **Agent chat で skill を呼び出し**、例を自分のタスクに置き換えます：

   | Agent | 例 |
   |-------|----|
   | Claude Code | `/bbs:autopilot "保存した検索の画面を追加する。先にプロトタイプを見せ、その後に実装、レビュー、QAを行う。"` |
   | Codex | `$bbs:autopilot "保存した検索の画面を追加する。先にプロトタイプを見せ、その後に実装、レビュー、QAを行う。"` |
   | OMP | `/autopilot "保存した検索の画面を追加する。先にプロトタイプを見せ、その後に実装、レビュー、QAを行う。"` |
   | Grok | `/bbs:autopilot "保存した検索の画面を追加する。先にプロトタイプを見せ、その後に実装、レビュー、QAを行う。"` |
   | Antigravity / その他の対応 agent | 導入済みの `autopilot` skill を使ってタスクを進めるよう依頼します。 |

3. 計画を読み、UI のタスクではプロトタイプを開きます。ここでレイアウト、操作の流れ、範囲を調整してください。Autopilot が `/goal` block を返したら、同じ agent に貼り付けて実行を始めます。Goal mode がない agent では、最初の依頼に `--stop-after=plan` を追加してこの確認段階で停止させ、handoff の再開指示に従ってください。無人で実行するには `--auto` を追加します（例：`/bbs:autopilot --auto "…"`）。Autopilot は自分で計画を確認し、`/goal` block を出力せず、agent が許す場合は自分で goal を設定し、そうでなければ同じ session で ticket が完了するか阻害されるまで作業を続けます。
4. 実装、レビュー、修正、テスト、QA を Autopilot に任せます。完了 gate を通過すると、検証済みの変更を未 commit のまま checkout に残し、証拠を添えます（commit するか `create-pr` に渡します）。阻害要因があれば不足を明示します。再起動後は handoff の ticket ID で再開できます。

最初の ticket に Orca、新しい worker session、プロジェクト設定は不要です。開始前に session の model を選んでください。[進捗確認と session 復旧](docs/companion-cli.md)。

## 2found と次のステップへ

製品に必要なものが増えたら、次の仕事に合うツールを選べます：

| 次にしたいこと | 探す |
|----------------|------|
| 繰り返しの仕事を小さなエージェントのチームに任せる | [**Soot**](https://trysoot.com)、powered by **2agent** — 来訪者への返答、情報源の監視、執筆、フォローアップをスケジュール通りにこなし、人が必要なときだけあなたに尋ねます。 |
| 作った製品をデプロイする | [**2server**](https://github.com/2found/2server) — 自分のインフラ上でアプリをデプロイ・運用。 |
| 顧客に製品を見つけてもらう | [**2market**](https://2found.dev/#2market) — 開発中のマーケティング用ワークスペース。製品の背景、コンテンツ、チャネルを 1 つの流れにつなぎます。 |

製品開発は 2build から始め、必要になったときにこれらの製品を検討してください。

## 仕事の内容で選ぶ

`/bbs:autopilot` は 1 つの作業(bug 修正、機能、丸一日かかる長い変更)を 1 つの session で plan、review、QA まで進めます。まずはこちらを使ってください。長い 1 session は context を温かく保ち、token 消費も最小です。プロジェクト全体を `/bbs:team` に任せるのは、1 session では得られないものが必要なときだけです([後述](#team))。特定の作業には skill を直接呼び出してください。

個別 skill と workflow の例は [skill 一覧](docs/skills.md)を参照してください。

<a id="foreman-の設定"></a>
<a id="foreman複数-ticket-のプロジェクト"></a>

<a id="team"></a>

## Team: プロジェクト全体を任せるプロダクトチーム

[Team](docs/team.md) は、雇ったプロダクトチームのように働きます。あなたが関わるのは 2 回だけです:

1. **依頼** — 要件を伝え、チームが書き戻した spec を確認し(質問は 1 回にまとめて、その後はなし)、basic design を承認します:アーキテクチャ、プロダクト全体の prototype、release phase。
2. **納品** — 完成したプロダクトを証拠とともにレビューし、release の go を出すか、フィードバックを返します。spec の範囲内のフィードバックはチームが自分で直して再納品し、追加の打ち合わせは不要です。spec を超える要望は design の変更として戻ってきます。

その間、チームは自分で design、build、review、QA、integrate を行い(phase ごとにローカル base 上の 1 コミット)、あなたには何も聞きません。worker はコミットせず、Team には [Orca](https://www.onorca.dev) が必要です。`/bbs:foreman` も alias として動作します。

Autopilot より高くつきます(lead session、design session、phase ごとの integration session)。理由があるときに使ってください:

- **手を離したい** — session を操縦する代わりに 2 回関わるだけで、最後に review 済みで release 可能なプロダクトを受け取る。
- **phase ごとの release** — design 承認時に許可した本番への段階的 release。最後に大きな変更を 1 回出す代わりに。
- **独立した検証** — 新しい session が各 phase を review・QA します。Autopilot はコードを書いた session 自身が review します。
- **分離した部分の並列作業** — 固定した contract でつながる別々の service や repo を、待ち時間が重要なときに並列で進める。

どれにも当てはまらなければ、丸一日かかる作業でも Autopilot 1 回のほうが速く安く済みます。

<details>
<summary>CLI と設定の詳細</summary>

## Skill と CLI

Skill は agent 向けのワークフローで、`bbs` はそれを支える CLI。`/bbs:team` はプロダクトチームを実行し、`bbs team`(alias `bbs foreman`)は model ポリシーの照会、永続的な lead 記録、phase（`seal`、`integrate`、`land`、`release`）、次の手順の提案、報告を扱い、`bbs contract` は並行 ticket が前提とするインターフェース契約を扱う。`/bbs:autopilot` は単一 ticket のワークフローを実行し、`bbs autopilot` は checkpoint と状態の補助コマンドを提供する。`bbs ticket` は ticket の識別、DAG 関係、証拠、テスト環境のリース、seal 済み作業の取り込みを管理する。

よく使う CLI コマンド：

```bash
bbs dashboard
bbs team report <parent-ticket>
bbs team next <parent-ticket>
bbs ticket dag <parent-ticket>
bbs autopilot snapshot --json
bbs autopilot recover --json
```

`snapshot` は ticket の正規状態と gate の証拠を読み取る。`recover` は再開に使う、分量を制限した成果物の抜粋を追加する。

使い方は `bbs <subcommand> --help` で確認できる。詳細は[付属 CLI](docs/companion-cli.md)、[git flow](.claude/skills/references/git-flow.md)、[運用](docs/operations.md)を参照。

## セマンティック判断

`semantic-decision` は、タスクの規模、複雑さ、ルーティング、テストへの影響、レビューの判断を、明示的で再利用できる skill のステップにします。既定の provider は現在の LLM で、設定も追加の model プロセスも不要です。Cloudflare を使うには：

```bash
bbs config set semantic_decision_provider cloudflare
bbs config set semantic_decision_model clef-flash  # or clef
```

`CLOUDFLARE_ACCOUNT_ID` と `CLOUDFLARE_API_TOKEN` を環境変数または `~/.babysit/.env` に設定します。provider を選べるのはユーザー設定だけで、プロジェクトのポリシーは外部推論を制限できます。Cloudflare がエラーを返した場合や回答が不確かな場合は、現在の LLM にフォールバックします。[skill](.claude/skills/semantic-decision/SKILL.md) と[契約](.claude/skills/references/semantic-decision.md)を参照してください。設定パス、既定値、プロジェクトの制限、人によるレビューの扱いは[dashboard にコントロールがない設定](docs/operations.md#settings-without-a-dashboard-control)を参照してください。

</details>

## ドキュメント（英語）

[導入とトラブル対処](docs/install.md) · [Skill を選ぶ](docs/skills.md) · [進捗を確認する](docs/companion-cli.md) · [プロジェクトを調整する](docs/team.md)

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
