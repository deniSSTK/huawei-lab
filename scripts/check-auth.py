#!/usr/bin/env python3
"""Lab 17 retest + regression. Restarts/recreates the lab; keeps old evidence.

Reads ignored local seed credentials, never writes passwords/cookies to logs.
Uses the two real Guest devices for authentication and direct network probes.
"""
import datetime
import hashlib
import json
import os
from pathlib import Path
import shlex
import socket
import subprocess
import sys
import time
import urllib.parse
from zoneinfo import ZoneInfo

ROOT = Path(__file__).resolve().parents[1]
STAMP = datetime.datetime.now(ZoneInfo('Europe/Kiev')).strftime('%Y-%m-%d_%H-%M-%S')
OUT = ROOT / 'evidence/lab-17' / STAMP
OUT.mkdir(parents=True)
USERS = json.loads((ROOT / '.secrets/test-users.json').read_text())
SECRET = (ROOT / '.secrets/gateway-token').read_text().strip()
RESULTS = []
PORTAL = 'http://172.28.10.20:8080'
CURRENT = 'environment'
TEST_ENV = dict(os.environ, SESSION_TTL_SECONDS='12')


def sanitize(text):
    for value, replacement in ((str(ROOT), '/workspace/huawei-lab'),
                               (str(Path.home()), '/home/lab-user'),
                               (socket.gethostname(), 'lab-host'),
                               (SECRET, '[REDACTED MANAGEMENT SECRET]')):
        if value:
            text = text.replace(value, replacement)
    for value in USERS.values():
        text = text.replace(value, '[REDACTED PASSWORD]')
    return text


def record(text):
    text = sanitize(text)
    for name in (CURRENT + '.log', 'execution.log'):
        with (OUT / name).open('a') as stream:
            stream.write(text + '\n')
    print(text, flush=True)


def run(args, *, input=None, expected=(0,), private=False, env=None, timeout=180):
    if not private:
        record('$ ' + shlex.join(args))
    result = subprocess.run(args, input=input, cwd=ROOT, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                            env=env, timeout=timeout)
    if not private:
        record(result.stdout.rstrip() + f'\nexit_status={result.returncode}')
    if result.returncode not in expected:
        raise AssertionError(f'Unexpected exit {result.returncode}: {shlex.join(args)}')
    return result.stdout


def dc(*args, **kwargs):
    return run(['docker', 'compose', *args], **kwargs)


def http(path, *, role=None, password=None, device='guest-client', jar='session', form=None, cookie=True, headers=()):
    args = ['exec', '-T', device, 'curl', '-4', '--noproxy', '*', '-sS', '--max-time', '5',
            '-H', 'Accept: application/json', '-w', '\nHTTP:%{http_code}']
    if cookie:
        args += ['-b', '/tmp/lab17-' + jar, '-c', '/tmp/lab17-' + jar]
    for header in headers:
        args += ['-H', header]
    data = None
    if role is not None or form is not None:
        args += ['--data-binary', '@-', '-H', 'Content-Type: application/x-www-form-urlencoded']
        fields = form if form is not None else {'username': role + '.test', 'password': password or USERS[role + '.test']}
        data = urllib.parse.urlencode(fields)
    args += [PORTAL + path]
    output = dc(*args, input=data)
    body, _, code = output.rpartition('\nHTTP:')
    return int(code), body


def login(role, **kwargs):
    code, body = http('/api/login', role=role, **kwargs)
    assert code == 200, (code, body)
    assert json.loads(body)['role'] == role
    return json.loads(body)


def logout(**kwargs):
    code, body = http('/api/logout', form={}, **kwargs)
    assert code == 204, (code, body)


def probe(port, allowed=False, device='guest-client'):
    counter = 'iot_to_corporate' if device == 'iot-client' else ('role_allowed' if allowed else 'guest_to_corporate')
    before = count(counter)
    if device == 'iot-client':
        args = ['exec', '-T', device, 'wget', '-T', '1', '-t', '1', '-qO-', f'http://172.28.20.10:{port}/']
        output = dc(*args, expected=(0,) if allowed else (1,))
    else:
        args = ['exec', '-T', device, 'curl', '-4', '--noproxy', '*', '-fsS', '--connect-timeout', '1', '--max-time', '2', f'http://172.28.20.10:{port}/']
        output = dc(*args, expected=(0,) if allowed else (28,))
    after = count(counter)
    assert after > before, (counter, before, after)
    if allowed:
        assert {80: 'STUDENT', 8081: 'TEACHER', 8082: 'ADMIN'}[port] in output
    record(f'PASS: {device} TCP/{port} {"ALLOWED" if allowed else "BLOCKED"}; {counter} {before} -> {after}')


def count(name):
    output = dc('exec', '-T', 'gateway', 'nft', '-j', 'list', 'counter', 'inet', 'lab_filter', name)
    return next(item['counter']['packets'] for item in json.loads(output)['nftables'] if 'counter' in item)


def scenario(name, description, action):
    global CURRENT
    CURRENT = name
    record(f'[{datetime.datetime.now(ZoneInfo("Europe/Kiev")).isoformat(timespec="seconds")}] {description}')
    try:
        action()
        RESULTS.append({'id': name, 'description': description, 'status': 'PASS', 'evidence': name + '.log'})
        record('RESULT: PASS ' + name)
    except Exception as error:
        RESULTS.append({'id': name, 'description': description, 'status': 'FAIL', 'error': sanitize(str(error)), 'evidence': name + '.log'})
        record('RESULT: FAIL ' + name + ': ' + str(error))
        raise


def unauthenticated():
    code, body = http('/login', cookie=False)
    assert code == 200 and 'University Access' in body
    for port in (80, 8081, 8082):
        dc('exec', '-T', 'gateway', 'wget', '-T', '2', '-qO-', f'http://172.28.20.10:{port}/')
        probe(port)
    code, _ = http('/api/login', role='student', password='intentionally-wrong')
    assert code == 401
    code, _ = http('/api/me')
    assert code == 401
    probe(80)


def roles():
    for role, allowed_ports in [('student', (80,)), ('teacher', (80, 8081)), ('admin', (80, 8081, 8082))]:
        login(role)
        for port in (80, 8081, 8082):
            probe(port, port in allowed_ports)
        logout()


def peer():
    login('admin')
    probe(8082, True)
    probe(80, device='guest-peer')
    data = dc('exec', '-T', 'guest-client', 'cat', '/tmp/lab17-session', private=True)
    dc('exec', '-T', 'guest-peer', 'sh', '-c', 'cat > /tmp/lab17-copied', input=data, private=True)
    code, _ = http('/api/me', device='guest-peer', jar='copied')
    assert code == 401
    record('PASS: copied session cookie rejected on a second Guest IP; token omitted from evidence')
    login('student', device='guest-peer', jar='peer')
    probe(80, True, device='guest-peer')
    probe(8082, True)
    logout(device='guest-peer', jar='peer')
    probe(80, device='guest-peer')
    probe(8082, True)
    logout()


def downgrade():
    login('admin', jar='old-admin')
    login('student')
    code, _ = http('/api/me', jar='old-admin')
    assert code == 401
    probe(80, True)
    probe(8081)
    probe(8082)
    logout()
    probe(80)


def persistent_flow():
    # A single socket is kept across logout: the second request must time out
    # even though conntrack still recognises the TCP connection as established.
    program = r'''
import http.client, json, socket, sys, urllib.parse, urllib.request
password=json.load(sys.stdin)['password']
def post(path,data,cookie=''):
 req=urllib.request.Request('http://172.28.10.20:8080'+path,data=urllib.parse.urlencode(data).encode(),headers={'Accept':'application/json','Cookie':cookie})
 return urllib.request.urlopen(req,timeout=5)
r=post('/api/login',{'username':'student.test','password':password})
cookie=r.headers['Set-Cookie'].split(';')[0]
conn=http.client.HTTPConnection('172.28.20.10',80,timeout=2)
conn.request('GET','/')
res=conn.getresponse();assert res.status==200;assert b'STUDENT' in res.read()
assert conn.sock is not None
sock=conn.sock
assert post('/api/logout',{},cookie).status==204
conn.request('GET','/')
assert conn.sock is sock
try:
 conn.getresponse()
 raise AssertionError('established connection survived logout')
except (TimeoutError,socket.timeout):
 print('PASS: same established TCP connection blocked after Logout')
finally:conn.close()
'''
    # Secret input is read from a separate inherited stream inside the client.
    payload = 'import io,sys\nsys.stdin=io.StringIO(' + repr(json.dumps({'password': USERS['student.test']})) + ')\n' + program
    before = count('guest_to_corporate')
    dc('exec', '-T', 'guest-client', 'python3', '-', input=payload)
    assert count('guest_to_corporate') > before
    probe(80)


def expiry(stop_portal=False):
    info = login('teacher')
    probe(8081, True)
    if stop_portal:
        dc('stop', 'portal')
    delay = max(0, info['expires'] - time.time()) + 1
    record(f'Wait {delay:.1f}s for configured 12s TTL (no permission refresh)')
    time.sleep(delay)
    probe(80)
    probe(8081)
    if stop_portal:
        dc('up', '-d', '--wait', 'portal', env=TEST_ENV)
    code, _ = http('/api/me')
    assert code == 401


def control_boundary():
    code, _ = http('/api/login', role='student', headers=('Origin: https://untrusted.example',))
    assert code == 403
    for device in ('guest-client', 'iot-client'):
        if device == 'iot-client':
            dc('exec', '-T', device, 'wget', '-T', '1', '-t', '1', '-qO-', 'http://172.28.50.254:9090/grant', expected=(1,))
        else:
            dc('exec', '-T', device, 'curl', '--noproxy', '*', '--max-time', '2', 'http://172.28.50.254:9090/grant', expected=(28,))
    dc('exec', '-T', 'portal', 'wget', '-T', '2', '-qO-', 'http://172.28.50.254:9090/grant', expected=(1,))
    record('PASS: management API unreachable from Guest/IoT; unauthenticated Portal request rejected')


def gateway_failure():
    # Remove the grant set while the daemon is alive, making nft transactions
    # fail. The portal must not report successful authentication.
    dc('exec', '-T', 'gateway', 'nft', 'flush', 'chain', 'inet', 'lab_filter', 'forward')
    # Restore fail-closed drop policy remains in place while removing set.
    dc('exec', '-T', 'gateway', 'nft', 'delete', 'set', 'inet', 'lab_filter', 'role_access')
    code, _ = http('/api/login', role='admin')
    assert code == 503
    code, _ = http('/api/me')
    assert code == 401
    dc('restart', 'gateway')
    dc('up', '-d', '--wait', env=TEST_ENV)
    probe(80)
    login('student')
    probe(80, True)
    logout()


def alternative():
    # Temporary listener, identical purpose to Lab 16 NT-01/02. Not a role port.
    config = "pid /tmp/lab17-nginx.pid; error_log /tmp/lab17-error.log; events {} http { access_log off; server { listen 8080; location / { return 200 'ALTERNATE probe'; } } }"
    dc('exec', '-T', 'internal-site', 'sh', '-c', "cat > /tmp/lab17-nginx.conf; nginx -c /tmp/lab17-nginx.conf", input=config)
    try:
        dc('exec', '-T', 'gateway', 'wget', '-T', '2', '-qO-', 'http://172.28.20.10:8080/')
        probe(8080)
        probe(8080, device='iot-client')
        login('admin')
        probe(8080)
        probe(80, device='iot-client')
        probe(8081, device='iot-client')
        probe(8082, device='iot-client')
        logout()
    finally:
        dc('exec', '-T', 'internal-site', 'nginx', '-c', '/tmp/lab17-nginx.conf', '-s', 'quit')


def regression():
    run(['sh', 'scripts/check-isolation.sh'])
    run(['sh', 'scripts/check-internet.sh'])


def recovery():
    login('student')
    probe(80, True)
    dc('restart', 'gateway')
    dc('up', '-d', '--wait', env=TEST_ENV)
    probe(80)
    # Relogin reconciles the persisted session with the cleared gateway set.
    login('student')
    probe(80, True)
    logout()
    regression()
    dc('up', '-d', '--force-recreate', '--wait', env=TEST_ENV)
    probe(80)
    login('teacher')
    probe(8081, True)
    logout()
    regression()


EXPECTED = [
    ('RT-01', 'NT-04: login page, wrong password and unauthenticated restrictions', unauthenticated),
    ('RT-02', 'NT-04: full Student/Teacher/Admin network port matrix', roles),
    ('RT-03', 'NT-04: independent device isolation and copied-cookie rejection', peer),
    ('RT-04', 'NT-04: role downgrade invalidates old session and old grants', downgrade),
    ('RT-05', 'NT-04: Logout closes an already established TCP connection', persistent_flow),
    ('RT-06', 'NT-04: session TTL revokes network access', lambda: expiry(False)),
    ('RT-07', 'NT-04: gateway TTL revokes access while Portal is stopped', lambda: expiry(True)),
    ('RT-08', 'NT-04: CSRF and closed/authenticated management boundary', control_boundary),
    ('RT-09', 'NT-04: nft failure rejects login; recovery remains fail closed', gateway_failure),
    ('RG-01', 'NT-01/02: alternate port denied, IoT stays isolated even during Admin session', alternative),
    ('RG-02', 'TC-01/02/07: Guest web and Guest/IoT isolation regression', regression),
    ('RG-03', 'NT-03: gateway restart and full recreation; login and regression after readiness', recovery),
]
(OUT / 'expected.json').write_text(json.dumps([{'id': n, 'expected': d, 'status': 'PASS'} for n, d, _ in EXPECTED], indent=2) + '\n')
exit_code = 0
try:
    record('Lab 17 started ' + STAMP + ' Europe/Kiev')
    run(['git', 'rev-parse', 'HEAD'])
    run(['git', 'status', '--short'])
    run(['docker', 'version'])
    dc('version')
    dc('config')
    record('Test override: SESSION_TTL_SECONDS=12; default TTL=300')
    dc('up', '-d', '--force-recreate', '--wait', 'portal', env=TEST_ENV)
    dc('ps')
    for name, description, action in EXPECTED:
        scenario(name, description, action)
except Exception as error:
    record('Execution stopped: ' + str(error))
    exit_code = 1
finally:
    CURRENT = 'restore'
    # Remove any test login and restore the normal deployment TTL. Full
    # gateway recreation removes permissions even after an interrupted test.
    restore_env = dict(os.environ, SESSION_TTL_SECONDS='300')
    try:
        dc('up', '-d', '--force-recreate', '--wait', 'gateway', 'portal', env=restore_env)
        dc('ps')
    except Exception as error:
        record('Restore failed: ' + str(error))
        exit_code = 1
    (OUT / 'results.json').write_text(json.dumps(RESULTS, indent=2) + '\n')
    files = [p for folder in ('auth', 'gateway', 'client', 'internal-site', 'scripts')
             for p in (ROOT / folder).rglob('*') if p.is_file() and '__pycache__' not in p.parts]
    files += [ROOT / 'docker-compose.yml', ROOT / '.dockerignore', ROOT / '.gitignore']
    manifest = {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(files)}
    (OUT / 'source-sha256.json').write_text(json.dumps(manifest, indent=2) + '\n')
    summary = ['# Lab 17 retest and regression', '', f'Run: {STAMP} Europe/Kiev.',
               'Test TTL: 12 seconds. Restored deployment TTL: 300 seconds.', '',
               '| ID | Result | Scenario | Evidence |', '| --- | --- | --- | --- |']
    summary += [f'| {r["id"]} | {r["status"]} | {r["description"]} | {r["evidence"]} |' for r in RESULTS]
    summary += ['', 'Source snapshot: source-sha256.json. Expectations saved before execution.',
                'Recovery checks cover state after readiness, not uninterrupted isolation during container recreation.',
                'Session grants are IP-bound in a trusted-client lab. Host browser is not a simulated Guest device.',
                f'Exit status: {exit_code}.']
    (OUT / 'results.md').write_text('\n'.join(summary) + '\n')
    print('Evidence: ' + str(OUT.relative_to(ROOT)), flush=True)
sys.exit(exit_code)
