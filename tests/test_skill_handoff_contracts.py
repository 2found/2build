"""No-model dry run: instruction boundaries plus real CLI artifact round trips.

These fixtures do not execute a model, product QA or release actions. They prove
that the documented producer schema/commands are consumable by the next skill.
"""
import json
import os
from pathlib import Path
import re
import shlex
import subprocess

import pytest

ROOT = Path(__file__).resolve().parents[1]
SKILLS = ROOT / '.claude/skills'


def skill(name):
    return (SKILLS / name / 'SKILL.md').read_text()


def test_orca_contract_is_owned_and_injected_only_by_foreman():
    for path in SKILLS.rglob('*.md'):
        if path.relative_to(SKILLS).parts[0] != 'foreman':
            assert not re.search(r'orca|worker_done|skills/orchestration',
                                 path.read_text(), re.IGNORECASE), path
    assert 'references/worker-lifecycle.md' in skill('foreman')
    adapter = (SKILLS / 'foreman/references/worker-lifecycle.md').read_text()
    for contract in ('every worker Task', 'SPAWNED=true', 'AGENT_ROLE=orca',
                     'orca orchestration ask', 'orca orchestration check',
                     'orca orchestration send', 'exactly one `worker_done`',
                     'whole Dispatch, not each nested skill'):
        assert contract in adapter


def test_qa_surface_ownership_and_blocked_coverage():
    qa = skill('qa')
    assert 'Foreman' not in qa
    assert 'Select the surface mode before preparation' in qa
    readonly = qa.split('**Caller-prepared read-only surface:**')[1].split('**Ticket worktree mode:**')[0]
    for constraint in ('preparation/migrations', 'code edits', "caller's lease",
                       'This explicit mode takes precedence'):
        assert constraint in readonly
    assert 'missing required coverage or failed\nacceptance forces FAIL → BLOCKED' in qa
    assert 'DONE_WITH_CONCERNS is only for nonblocking residuals' in qa
    assert 'After QA fixes, return for\ncaller commit/retest' in qa
    worktrees = (SKILLS / 'references/worktrees.md').read_text()
    assert 'QA does not commit or push' in worktrees
    assert 'pending retest is BLOCKED, never PASS' in worktrees
    assert 'set-verdict --skill qa`' not in worktrees


@pytest.fixture(scope='session')
def binary(tmp_path_factory):
    path = tmp_path_factory.mktemp('handoff-bin') / 'bbs'
    subprocess.run(['go', 'build', '-o', str(path), './cmd/bbs'], cwd=ROOT, check=True)
    return path


@pytest.fixture
def repo(tmp_path, binary):
    path = tmp_path / 'repo'
    path.mkdir()
    env = {k: v for k, v in os.environ.items()
           if not k.startswith(('BABYSIT_', 'BBS_'))}
    env.update(HOME=str(tmp_path / 'home'), BABYSIT_HOME=str(tmp_path / 'state'),
               BABYSIT_STATE_DIR=str(tmp_path / 'state'),
               BABYSIT_PROJECT_HOME=str(tmp_path / 'state/projects/dry-run'),
               BABYSIT_ANALYTICS_DIR=str(tmp_path / 'analytics'),
               GIT_CONFIG_NOSYSTEM='1')
    Path(env['HOME']).mkdir()

    def run(*args, check=True):
        return subprocess.run([str(binary) if args[0] == 'bbs' else args[0], *args[1:]],
                              cwd=path, env=env, capture_output=True, text=True, check=check)
    run('git', 'init', '-qb', 'main')
    run('git', 'config', 'user.email', 'fixture@example.invalid')
    run('git', 'config', 'user.name', 'Contract fixture')
    (path / 'app.txt').write_text('baseline\n')
    run('git', 'add', 'app.txt')
    run('git', 'commit', '-qm', 'baseline')
    return path, env, run


def attach(repo):
    _, env, run = repo
    env['BABYSIT_TICKET'] = 'dry-run'
    run('bbs', 'ticket', 'init')
    return run


def test_documented_handoff_command_round_trip(repo, tmp_path):
    run = attach(repo)
    doc = (SKILLS / 'references/handoff-contracts.md').read_text()
    command = next(line for line in doc.splitlines() if line.startswith('bbs ticket add-handoff '))
    brief = tmp_path / 'implement-brief.md'
    brief.write_text('SUMMARY: fixture\nFILES: app.txt\nAPPROACH: bounded edit\n'
                     'BLAST_RADIUS: file reader\n\n## Deviations\nnone\n')
    command = command.replace('<skill>', 'implement').replace('<status>', 'DONE')
    command = command.replace('<brief-path>', str(brief))
    result = run(*shlex.split(command))
    consumed = Path(run('bbs', 'ticket', 'path', 'handoff', '--skill', 'implement',
                        '--latest', '--read').stdout.strip())
    assert consumed == Path(result.stdout.strip())
    assert consumed.read_text() == brief.read_text()


def test_plan_is_persisted_for_autopilot_and_implement(repo, tmp_path):
    producer = skill('plan-draft')
    for contract in ('bbs ticket path plan --write', 'set-pointer plan',
                     'set-verdict --skill plan-draft'):
        assert contract in producer
    run = attach(repo)
    assert 'bbs ticket path design --write' in skill('design-ui')
    design = Path(run('bbs', 'ticket', 'path', 'design', '--write').stdout.strip())
    design.write_text('Design: fixture\nPrototype: supplied fixture\n')
    run('bbs', 'ticket', 'set-pointer', 'design', str(design))
    assert Path(run('bbs', 'ticket', 'path', 'design', '--read').stdout.strip()) == design
    plan = Path(run('bbs', 'ticket', 'path', 'plan', '--write').stdout.strip())
    plan.write_text('# Plan\n**Goal:** fixture\n**Verify:** inspect app.txt\n')
    run('bbs', 'ticket', 'set-pointer', 'plan', str(plan))
    verdict = tmp_path / 'plan-verdict.md'
    verdict.write_text('STATUS: DONE\nVERDICT: PLANNED(S)\nPLAN: ' + str(plan) + '\n')
    run('bbs', 'ticket', 'set-verdict', '--skill', 'plan-draft', '--body-file', str(verdict))
    assert run('bbs', 'ticket', 'verdict-status', '--skill', 'plan-draft').stdout.strip() == 'DONE'
    assert Path(run('bbs', 'ticket', 'path', 'plan', '--read').stdout.strip()).read_text() == plan.read_text()


def test_implement_publishes_qa_inputs():
    producer, consumer = skill('implement'), skill('qa')
    assert 'handoff-contracts.md' in producer
    for field in ('SUMMARY', 'FILES', 'APPROACH', 'BLAST_RADIUS', '## Deviations'):
        assert field in producer
    assert 'add-handoff --skill implement --status' in producer
    assert 'path handoff --skill implement --latest --read' in consumer


def test_review_emits_gate_report_not_just_findings(repo, tmp_path):
    producer = skill('review-pr')
    for field in ('STATUS:', 'VERDICT:', 'FINDINGS:', 'FIXED:', 'RISK_AREAS:'):
        assert field in producer
    run = attach(repo)
    # The previous findings-only output must not masquerade as a gate verdict.
    raw = tmp_path / 'raw.json'
    raw.write_text('[]\n')
    assert run('bbs', 'ticket', 'set-verdict', '--skill', 'review-pr',
               '--body-file', str(raw), check=False).returncode == 2
    for status, verdict, finding in [('BLOCKED', 'FINDINGS(1)', 'material fixture'),
                                     ('DONE_WITH_CONCERNS', 'FINDINGS(1)', 'minor fixture'),
                                     ('DONE', 'PASS', 'none')]:
        raw.write_text(f'STATUS: {status}\nVERDICT: {verdict}\nSUMMARY: fixture\n'
                       f'FINDINGS: {finding}\nFIXED: none\nRISK_AREAS: file reader\n')
        run('bbs', 'ticket', 'set-verdict', '--skill', 'review-pr', '--body-file', str(raw))
        assert run('bbs', 'ticket', 'verdict-status', '--skill', 'review-pr').stdout.strip() == status
        path = Path(run('bbs', 'ticket', 'path', 'verdict', '--skill', 'review-pr', '--read').stdout.strip())
        assert path.read_text() == raw.read_text()


@pytest.mark.parametrize('name', ['implement', 'qa', 'review-pr'])
def test_standalone_without_ticket_plan_origin_or_clean_tree(name, repo):
    text = skill(name)
    assert '## Standalone' in text
    assert 'no ticket' in text.lower()
    path, env, run = repo
    (path / 'app.txt').write_text('local change\n')
    (path / 'new.txt').write_text('untracked\n')
    assert run('git', 'remote').stdout == ''
    assert run('bbs', 'ticket', 'resolve', check=False).returncode != 0
    # The documented no-origin fallback reads the dirty checkout without mutations.
    assert 'local change' in run('git', 'diff', 'HEAD').stdout
    assert 'new.txt' in run('git', 'ls-files', '--others', '--exclude-standard').stdout
    assert run('git', 'branch', '--show-current').stdout.strip() == 'main'
    assert not list(Path(env['BABYSIT_PROJECT_HOME']).glob('tickets/*'))
    if name in ('qa', 'review-pr'):
        assert 'handoff-contracts.md' in text
    if name == 'qa':
        assert 'scratch' in text.lower()
        assert 'security-review' not in text  # no such bundled skill


def test_qa_report_is_consumable_by_evidence_audit(repo, tmp_path):
    run = attach(repo)
    report = tmp_path / 'qa-report.md'
    body = ('STATUS: DONE\nVERDICT: PASS\nSUMMARY: synthetic contract fixture\n'
            'SOURCES: fixture requirement\n'
            'RUBRIC: flow=B boundary=B regression=B data=N/A compat=N/A '
            'security=N/A a11y=N/A perf=N/A freshness=A\n'
            'EVIDENCE: CLI e2e contract fixture, not a product test\n'
            'CLEANUP: no owned processes\n')
    report.write_text(body)
    run('bbs', 'ticket', 'set-verdict', '--skill', 'qa', '--body-file', str(report))
    assert run('bbs', 'ticket', 'qa-evidence').stdout.strip() == 'ok'
    report.write_text(body.replace('freshness=A', 'freshness=B'))
    run('bbs', 'ticket', 'set-verdict', '--skill', 'qa', '--body-file', str(report))
    assert run('bbs', 'ticket', 'qa-evidence').stdout.strip() == 'contradiction:freshness=B'
    report.write_text('STATUS: BLOCKED\nVERDICT: FAIL\nSUMMARY: fixture failure\n')
    run('bbs', 'ticket', 'set-verdict', '--skill', 'qa', '--body-file', str(report))
    readiness = json.loads(run('bbs', 'ticket', 'readiness', '--action', 'review', '--json').stdout)
    assert not readiness['data']['ready']


def test_v2_projection_preserves_full_review_for_qa(repo, tmp_path):
    run = attach(repo)
    path, env, _ = repo
    env['BABYSIT_SKIP_CHECKPOINT_VALIDATION'] = '1'
    for kind in ('requirement', 'plan'):
        output = Path(run('bbs', 'ticket', 'path', kind, '--write').stdout.strip())
        output.write_text(f'{kind}: contract fixture\n')
    run('bbs', 'autopilot', 'checkpoint', '--ticket', 'dry-run', '--workflow', 'builder',
        '--step', 'run', '--status', 'in_progress', '--contract-version', '2')
    report = tmp_path / 'review.md'
    report.write_text('STATUS: DONE\nVERDICT: PASS\nFINDINGS: none\n'
                      'FIXED: none\nRISK_AREAS: fixture input parser\n')
    run('bbs', 'ticket', 'set-review', '--skill', 'review-pr', '--body-file', str(report))
    attempt = json.loads(run('bbs', 'autopilot', 'verification', 'begin', '--gate', 'review-pr',
                             '--owner', 'fixture', '--handle', 'fixture-session',
                             '--transport', 'fixture', '--harness', 'fixture').stdout)['data']['id']
    # Real read-only check on the fixture, with its real stdout retained as the log.
    check = run('git', 'show', 'HEAD:app.txt')
    log = tmp_path / 'check.log'
    log.write_text(check.stdout)
    result = tmp_path / 'results.json'
    result.write_text(json.dumps({'checks': [{'argv': ['git', 'show', 'HEAD:app.txt'],
                                              'cwd': str(path), 'exit_code': check.returncode,
                                              'log_path': str(log)}],
                                  'unresolved_findings': [], 'limitations': []}))
    run('bbs', 'autopilot', 'verification', 'record', '--attempt', attempt, '--file', str(result))
    projection = Path(run('bbs', 'ticket', 'path', 'verdict', '--skill', 'review-pr', '--read').stdout.strip())
    full = Path(run('bbs', 'ticket', 'path', 'review', '--skill', 'review-pr', '--read').stdout.strip())
    assert 'RISK_AREAS:' not in projection.read_text()
    assert full.read_text() == report.read_text()
    assert 'path review --skill review-pr --read' in skill('qa')
    assert 'set-review --skill review-pr' in skill('review-pr')


def test_workflows_never_replace_reports_with_empty_verdicts(repo, tmp_path):
    run = attach(repo)
    report = tmp_path / 'review.md'
    report.write_text('STATUS: DONE\nVERDICT: PASS\n')
    run('bbs', 'ticket', 'set-verdict', '--skill', 'review-pr', '--body-file', str(report))
    # A bare write really does erase a valid status, so examples must carry a body.
    run('bbs', 'ticket', 'set-verdict', '--skill', 'review-pr')
    assert run('bbs', 'ticket', 'verdict-status', '--skill', 'review-pr').stdout.strip() == 'none'
    for path in (SKILLS / 'autopilot/workflows').glob('*.md'):
        for command in re.findall(r'`([^`]*bbs ticket set-verdict[^`]*)`', path.read_text()):
            assert '--body-file ' in command, (path, command)
