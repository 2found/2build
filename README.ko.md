# 2build

[English](README.md) | [Tiếng Việt](README.vi.md) | [中文](README.zh.md) | [日本語](README.ja.md) | 한국어

**Coding agent에게 목표를 맡기고, 리뷰와 테스트를 거친 변경을 받으세요.**

2build은 Claude Code, Codex, Antigravity를 위한 오픈소스 skill pack입니다. 함께 제공되는 CLI가 진행 상황과 검증 근거를 저장합니다. **Autopilot**부터 시작하세요. 기존 agent session에서 하나의 ticket을 요구사항부터 로컬 commit까지 진행합니다.

<a id="설치"></a>

## 하나의 prompt로 설치

터미널을 실행할 수 있는 coding agent에 다음을 붙여 넣으세요:

```text
지금 사용 중인 coding agent에 2build을 설치해 주세요.
https://raw.githubusercontent.com/2found/2build/main/docs/install.md 를 따르세요.
OS와 현재 agent를 확인하고, 작동하는 bbs가 있으면 재사용하거나 CLI를 설치한 뒤 이 agent에만 skill pack을 설치하세요.
bbs --version과 설치된 plugin을 검증하고, 필요한 조건이 빠졌다면 성공이라고 하지 말고 알려 주세요.
재시작이 필요한지 설명하고, 첫 작은 작업을 실행할 수 있도록 이 agent에 맞는 정확한 Autopilot 호출을 알려 주세요.
```

지원되는 coding agent와 기존 model 이용 권한이 필요합니다. Claude Code와 Codex는 CLI가 PATH에 있어야 합니다. 2build용 별도 model 계정은 필요 없으며 agent의 일반 이용 요금은 적용됩니다. **Orca는 Foreman에만 필요합니다.** [설치 및 문제 해결](docs/install.md)(영어)을 참고하세요.

<details>
<summary>직접 명령을 실행하려면: macOS / Linux의 Homebrew</summary>

```bash
brew tap 2found/2build https://github.com/2found/2build
brew install 2found/2build/bbs
bbs install
```

`bbs install`은 감지한 모든 지원 agent에 설치합니다. 하나만 선택하려면 `bbs install claude`, `bbs install codex`, `bbs install antigravity`를 사용하세요. 설치 후 agent를 재시작하세요. Homebrew가 없으면 [release archive](docs/install.md#release-archives-macos-or-linux)를, Windows에서는 WSL을 사용하세요.

</details>

## 2build의 차이점

Prompt는 만드는 방법을 설명할 수 있습니다. 2build은 workflow와 디스크 상태를 더해 목표를 리뷰와 검증까지 진행합니다. Session이 재시작되어도 이어서 작업할 수 있습니다.

| 필요한 것 | 2build이 제공하는 것 |
|-----------|----------------------|
| 단계마다 지시하지 않고 작업 완료 | Autopilot이 하나의 ticket을 계획, 구현, 리뷰 수정, QA까지 진행합니다. |
| Crash나 context reset 후 재개 | 요구사항, 계획, checkpoint, handoff를 디스크에 저장하고 근거를 읽어 복구합니다. |
| 결과가 검증되었는지 확인 | Review / QA verdict를 저장하며, 완료에는 최신 검사 통과와 중대한 미해결 문제의 해소가 필요합니다. |
| 전달 시점 직접 관리 | 단독 Autopilot은 로컬 commit까지 합니다. 근거를 검토한 후 push하거나 PR을 만드세요. |

검증된 인계가 필요하거나 자리를 비운 동안 agent가 일하게 하려는 작업에 적합합니다. 간단한 수정은 coding agent만으로도 충분할 수 있습니다. 프로젝트 테스트 환경은 필요하며, 접근 권한이나 필수 검사가 부족하면 `NEEDS_CONTEXT` / `BLOCKED`로 보고합니다.

<a id="autopilot-하나의-ticket"></a>

## 첫 ticket 실행

1. Agent를 재시작하고 Git repo를 열어 성공 조건이 명확한 작은 bug나 기능을 고르세요. Autopilot은 현재 checkout에서 작업하고 commit합니다. 작업을 분리하려면 먼저 원하는 branch를 만드세요.
2. **Agent chat에서 skill을 호출**하고 예제를 자신의 작업으로 바꾸세요:

   | Agent | 예제 |
   |-------|------|
   | Claude Code | `/bbs:autopilot "검색 결과가 비었을 때의 상태를 수정하고 회귀 테스트 추가"` |
   | Codex | `$bbs:autopilot "검색 결과가 비었을 때의 상태를 수정하고 회귀 테스트 추가"` |
   | Antigravity | 설치된 `autopilot` skill로 작업하도록 요청하세요. |

3. Autopilot이 계획과 `/goal` block을 반환하면 계획을 검토하고 같은 agent에 block을 붙여 넣어 build를 시작하세요. Goal mode가 없는 agent는 현재 session에서 계속합니다.
4. 로컬 commit, review / QA 근거, 변경과 검사를 설명한 handoff가 결과입니다. 막힌 실행은 부족한 조건을 알려 줍니다. 재시작 후 handoff의 ticket ID를 Autopilot에 전달해 재개하세요.

첫 ticket에는 Orca, 새 worker session, 프로젝트 설정이 필요 없습니다. 시작 전에 session model을 선택하세요. [진행 확인과 session 복구](docs/companion-cli.md).

## 작업 내용으로 선택

Autopilot은 작업을 제품 팀의 다섯 역할 중 하나로 배정합니다. Workflow를 지정하거나 skill을 직접 호출할 수도 있습니다.

| 역할 | 작업 |
|------|------|
| Prototyper | 프로덕션 코드에 투자하기 전에 위험한 가설 검증. |
| Builder | 기능이나 bug 수정을 review와 QA까지 완료. |
| Sweeper | 동작을 유지하면서 불필요한 부분 제거 또는 측정된 병목 개선. |
| Grower | 카피, 전환, 측정 가능한 성장 실험 개선. |
| Maintainer | 장애 원인 파악 및 안정성, 보안, 의존성 강화. |

개별 skill과 workflow 예제는 [skill 목록](docs/skills.md)을 참고하세요.

<a id="foreman-설정"></a>
<a id="foreman-여러-ticket으로-구성된-프로젝트"></a>

## 더 큰 프로젝트: Foreman

여러 ticket에 의존성이 있다면 [Foreman](docs/foreman.md)을 사용하세요. [Orca](https://www.onorca.dev)로 전체 계획을 세우고, 자식 ticket을 worktree로 분리하고, 준비된 작업을 dispatch하며 프로젝트 전체 QA를 수행합니다. 기본적으로 프로덕션 작업을 dispatch하기 전에 부모 계획과 설계를 검토합니다. 명시적인 `--auto`는 해당 검토를 worker에 위임합니다. 설정된 finish policy가 전달 방식을 결정합니다.

## Skill과 CLI

Skill은 agent용 workflow이고 `bbs`는 이를 지원하는 CLI입니다. `/bbs:foreman`은 프로젝트 조정자를 실행하며, `bbs foreman`은 model 정책 조회, 영구 Foreman 기록, 계약, 보고서를 관리합니다. `/bbs:autopilot`은 단일 ticket workflow를 실행하며, `bbs autopilot`은 checkpoint와 상태 관리 도구를 제공합니다. `bbs ticket`은 ticket 식별, DAG 관계, 증거, 테스트 환경, 인도, 정리를 관리합니다.

자주 쓰는 CLI 명령:

```bash
bbs dashboard
bbs foreman report <parent-ticket>
bbs ticket dag <parent-ticket>
bbs autopilot snapshot --json
bbs autopilot recover --json
```

`snapshot`은 ticket의 기준 상태와 gate 증거를 읽습니다. `recover`는 작업을 재개할 수 있도록 분량이 제한된 산출물 발췌를 추가합니다.

사용법은 `bbs <subcommand> --help`로 확인하세요. 자세한 내용은 [지원 CLI](docs/companion-cli.md), [profile](docs/profiles.md), [운영](docs/operations.md)을 참고하세요.

## 저장소 구조

- `bbs` — 여러 명령을 처리하는 바이너리입니다. Build 결과는 gitignore 대상이며, `go build -o bbs ./cmd/bbs`로 만듭니다. Hook은 컴파일된 하위 명령인 `bbs hooks <name>`입니다.
- `.claude/skills/` — agent skill과 workflow 참고 자료.
- `internal/` — Go CLI와 서비스.
- `web/` — dashboard SPA. 릴리스 build에 포함됩니다.
- `tests/`, `docs/` — 검증 모음과 사용자 문서.

## 텔레메트리

Skill 사용 기록은 로컬 `~/.babysit/analytics/` 아래에 JSONL 형식으로 저장됩니다. 텔레메트리는 무인 실행의 주요 피드백 채널입니다.

## 라이선스

MIT.
