# babysit

[English](README.md) | [Tiếng Việt](README.vi.md) | [中文](README.zh.md) | [日本語](README.ja.md) | 한국어

**목표를 맡기세요. 자리를 비운 동안 계획하고, 구현하고, 리뷰하고, 검증합니다.**

Babysit은 agent skill 팩과 이를 지원하는 Go CLI입니다. 순차적으로 처리할 ticket 하나에는 Autopilot을, 서로 의존하는 ticket과 worker 감독이 필요한 프로젝트에는 Foreman을 사용하세요. Skill은 제품 개발 팀의 다섯 역할인 Prototyper, Builder, Sweeper, Grower, Maintainer에 따라 구성되며, 파일 종류가 아닌 작업 내용으로 선택합니다. [역할 설명](.claude/skills/references/archetypes.md)을 참고하세요.

## 설치

CLI를 설치한 뒤 Babysit이 설치된 harness를 자동으로 감지하고 설정하게 하세요.

```bash
brew install lohi-ai/babysit/bbs
bbs install
```

설치 후 agent를 다시 시작하세요. `bbs install`은 Claude Code, Codex, Antigravity를 지원합니다. Claude Code와 Codex CLI는 PATH에 있어야 합니다. 개별 설치는 `bbs install claude`, `bbs install codex`, `bbs install antigravity`를 사용하세요.

세 가지 OS를 모두 지원합니다: macOS와 Linux는 Homebrew, Linux는 아키텍처별 tarball로도 설치할 수 있습니다. Windows용 바이너리는 배포하지 않습니다 — WSL 또는 Git-Bash에서 실행하거나, checkout에서 `go run ./cmd/bbs setup`으로 빌드하세요. 전체 플랫폼 매트릭스는 [설치 안내](docs/install.md)를 참고하세요.

Babysit 자체를 개발하려면 저장소를 clone하고 `go run ./cmd/bbs setup --full`을 실행하세요. `bbs`를 build하고 checkout을 로컬 플러그인으로 등록하는 명령을 출력합니다. `bbs update`는 CLI와 설치된 플러그인을 갱신합니다.

## Foreman 설정

Foreman에는 orchestration이 활성화된 [Orca](https://www.onorca.dev)가 필요합니다. Worker의 agent는 명시적 선택, 호환되는 단계별 고정 설정, 실행 대상 호스트의 Orca 기본 설정 순으로 선택합니다. 기존 Babysit YAML의 agent/provider/model/effort 설정은 폐기되어 무시됩니다.

Babysit은 작업 복잡도(`simple`, `normal`, `hard`)와 단계 분류에 따라 worker의 model과 effort를 선택합니다. 계획, 설계, 리뷰는 `critical` 단계이고, 구현, QA, 인도는 `normal` 단계입니다. 정책은 이 조합을 `flash`, `pro`, `max` tier에 매핑하며, 각 tier는 선택한 agent에 맞는 model 설정을 갖습니다. 명시적인 단계별 재정의와 재개를 위해 저장된 유효한 route가 우선합니다.

프로젝트 디렉터리에서 적용 중인 정책 전체 또는 특정 선택 결과를 확인하세요.

```bash
bbs foreman model --json
bbs foreman model --agent codex --complexity normal --phase-class critical --json
```

이 조회에는 ticket이나 Orca 연결이 필요하지 않습니다. 다른 프로젝트의 정책은 `--dir <repo-or-worktree>`를 추가해 확인하세요. `~/.babysit/settings.json` 또는 `<repo>/.babysit/settings.json`의 `foreman.models` 아래에서 필드별로 재정의할 수 있습니다. 저장소 설정, 전역 설정, 내장 기본값 순으로 우선합니다. 예를 들어 `hard` 작업의 `normal` 단계를 `max` tier로 보내려면 다음과 같이 설정합니다.

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

Agent별 model/effort 설정과 재개 동작은 [model routing](.claude/skills/foreman/references/model-routing.md#model-tiers)을 참고하세요.

## Foreman: 여러 ticket으로 구성된 프로젝트

Agent에서 **Foreman skill**을 호출하세요. 아래 예시는 `bbs` CLI 명령이 아닌 skill 호출입니다.

```text
# Claude Code
/bbs:foreman "web과 API에 걸친 요청 처리 흐름을 다시 구축해 줘"
/bbs:foreman --auto "web과 API에 걸친 요청 처리 흐름을 다시 구축해 줘"

# Codex
$bbs:foreman "web과 API에 걸친 요청 처리 흐름을 다시 구축해 줘"
```

Foreman은 상위 프로젝트를 초기화하거나 재개하고 해당 Orca Run을 연결합니다. 계획/설계 worker가 먼저 전체 계획, prototype(UI가 없는 작업이면 흐름/인터페이스 설계), 범위와 의존성을 담은 안정적인 ticket manifest를 만듭니다. 기본적으로 하위 ticket이나 worktree를 만들거나 실제 구현 작업을 배정하기 전에 사용자가 이 산출물과 제안된 ticket을 검토합니다. `--auto`를 명시하면 별도의 증거 검토 worker에게 이 검토를 맡기고 승인을 기록합니다. 안전상 보류나 이후 QA를 건너뛰지는 않습니다.

승인된 초안은 의존 관계가 명시된 ticket DAG가 됩니다. Foreman은 worker/리소스 한도 내에서 준비된 ticket만 배정하며, ticket worktree마다 쓰기 담당 worker를 하나로 유지합니다. 각 ticket은 범위가 정해진 별도의 Plan, Implement, Review, QA worker 단계를 거칩니다. Routing은 작업 복잡도와 단계 또는 배정별 명시적 선택을 따릅니다.

Foreman은 Orca를 통해 worker의 보고, 질문, 상위 판단 요청을 기다립니다. Terminal을 폴링하거나 재시도 타이머를 시작하지 않습니다. 다음 단계로 넘어가기 전에 각 단계의 산출물, 리비전, verdict를 확인합니다. Ticket이 gate를 통과하면 설정된 종료 정책(`review`, `land`, `pr`)에 따라 인도합니다. Foreman은 실행 결과 확인 기록을 검증하고, 종료된 worker와 리소스 임대를 해제하고, 자신이 소유한 Orca 화면을 닫습니다. 조건을 충족하는 깨끗한 worktree만 삭제하며 branch는 유지합니다.

프로젝트 마무리도 worker가 수행합니다. 감사와 승인된 인도/정리 worker가 먼저 종료한 뒤, 읽기 전용 최종 QA worker가 실제로 인도된 base 또는 유지된 QA 통합 상태를 확인합니다. Foreman은 최종 증거, 정리, 준비 상태 확인이 모두 통과해야 상위 프로젝트를 완료합니다. 서로 영향을 주는 ticket에는 land 전 통합 QA도 수행하지만, 이것이 프로젝트 최종 QA를 대신하지는 않습니다.

## Autopilot: 하나의 ticket

단일 ticket에는 **Autopilot skill**을 호출하세요.

```text
# 이미 연 session에서 Claude Code skill 호출
/bbs:autopilot "다크 모드 전환 버튼을 추가해 줘"
```

독립 실행하는 Autopilot은 사용자가 시작한 session에서 계획, 구현, 리뷰, QA를 수행합니다. 새 model을 선택하거나 실행 중에 model을 바꾸지 않으므로 시작 전에 선택하세요. Codex에서는 `$bbs:autopilot`으로 호출합니다. Orca 없이 작동하며 백그라운드 worker session을 열지 않습니다.

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
