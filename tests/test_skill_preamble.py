"""Native skill bootstrap and exit contracts, with isolated state and no network."""
import json
import os
from pathlib import Path
import subprocess

import pytest

ROOT = Path(__file__).resolve().parents[1]


@pytest.fixture(scope='module')
def binary(tmp_path_factory):
    binary = tmp_path_factory.mktemp('preamble-bin') / 'bbs'
    subprocess.run(['go', 'build', '-o', str(binary), './cmd/bbs'], cwd=ROOT, check=True)
    return binary


@pytest.fixture
def runtime(tmp_path, binary):
    state = tmp_path / 'state'
    state.mkdir()
    (state / 'config.yaml').write_text('update_check: false\nproactive: false\n')
    repo = tmp_path / 'quoted "repo'
    repo.mkdir()
    env = {'HOME': str(tmp_path / 'home'), 'PATH': os.environ['PATH'],
           'BABYSIT_HOME': str(state), 'BABYSIT_STATE_DIR': str(state),
           'BABYSIT_PROJECT_HOME': str(state / 'projects/test'),
           'BABYSIT_CURRENT_AGENT': 'codex', 'CODEX_THREAD_ID': 'test-session'}
    for args in (['init', '-qb', 'main'], ['config', 'user.email', 'test@example.invalid'],
                 ['config', 'user.name', 'Test'], ['commit', '--allow-empty', '-qm', 'init']):
        subprocess.run(['git', *args], cwd=repo, env=env, check=True, capture_output=True)

    def run(*args, check=True):
        return subprocess.run([str(binary), *args], cwd=repo, env=env, text=True,
                              capture_output=True, check=check, timeout=30)
    return state, env, run


def fields(result):
    return dict(line.split(': ', 1) for line in result.stdout.splitlines()
                if ': ' in line and not line.startswith('{'))


def test_standalone_bootstrap_and_correlated_exit(runtime):
    state, env, run = runtime
    start = fields(run('skill', 'enter', '--name', 'implement'))
    assert start['TICKET'] == '<none>'
    assert start['SKILL_REF'] == '$bbs:'
    assert start['PROACTIVE'] == 'false'
    assert start['AUTOPILOT_CONTRACT'] == 'v2'
    assert not (state / 'projects/test/tickets').exists()
    session = state / 'sessions/cx-test-session.yaml'
    assert session.exists()
    invocation = start['SESSION_ID']
    marker = state / 'sessions' / (invocation + '.active')
    assert marker.exists()
    other = fields(run('skill', 'enter', '--name', 'review-pr'))['SESSION_ID']
    result = json.loads(run('skill', 'exit', '--invocation', invocation,
                            '--outcome', 'success').stdout)['data']
    assert result['duration_s'] >= 0
    assert not marker.exists()
    assert (state / 'sessions' / (other + '.active')).exists()
    assert session.exists()
    rows = [json.loads(line) for line in (state / 'analytics/skill-usage.jsonl').read_text().splitlines()]
    assert [row['event'] for row in rows] == ['start', 'start', 'end']
    assert rows[0]['session'] == rows[-1]['session'] == invocation
    assert rows[-1]['skill'] == 'implement'
    assert rows[0]['repo'] == 'quoted "repo'


def test_ticket_init_refresh_sweep_and_recovery(runtime):
    state, env, run = runtime
    env.update(BABYSIT_TICKET='bs-test', BABYSIT_SESSION='explicit-session')
    sessions = state / 'sessions'
    sessions.mkdir()
    old = sessions / 'old.yaml'
    old.write_text('stale')
    os.utime(old, (1, 1))
    fresh = sessions / 'fresh.yaml'
    fresh.write_text('fresh')
    result = run('skill', 'enter', '--name', 'qa')
    assert fields(result)['TICKET'] == 'bs-test'
    assert (state / 'projects/test/tickets/bs-test/index.json').exists()
    assert not old.exists() and fresh.exists()
    session = sessions / 'explicit-session.yaml'
    before = session.read_text()
    assert 'ticket: bs-test\n' in before
    os.utime(session, (1, 1))
    run('skill', 'enter', '--name', 'qa')
    after = session.read_text()
    assert next(x for x in before.splitlines() if x.startswith('started_at:')) == next(
        x for x in after.splitlines() if x.startswith('started_at:'))
    assert session.stat().st_mtime > 1
    snapshot = json.loads(next(line for line in result.stdout.splitlines() if line.startswith('{')))
    assert snapshot['ticket']['id'] == 'bs-test'
    assert 'gates' in snapshot and 'obligations' in snapshot
    assert 'export BABYSIT_TICKET=bs-test' in run('ticket', 'session', 'attach', 'explicit-session').stdout


def test_json_enter_stays_telemetry_only_and_off_writes_no_events(runtime):
    state, env, run = runtime
    (state / 'config.yaml').write_text('update_check: false\ntelemetry: off\n')
    result = json.loads(run('skill', 'enter', '--name', 'qa', '--json').stdout)
    assert result['ok'] and result['data']['telemetry'] == 'off'
    assert not (state / 'sessions').exists()
    start = fields(run('skill', 'enter', '--name', 'qa'))
    run('skill', 'exit', '--invocation', start['SESSION_ID'], '--outcome', 'abort')
    assert not list((state / 'sessions').glob('*.active'))
    assert not (state / 'analytics').exists()


def test_identity_conflict_fails_before_writes(runtime):
    state, env, run = runtime
    env.update(BABYSIT_TICKET='one', BBS_TICKET='two')
    result = run('skill', 'enter', '--name', 'qa', check=False)
    assert result.returncode == 2
    assert not (state / 'sessions').exists()
    assert not (state / 'analytics').exists()
    assert not (state / 'projects').exists()
