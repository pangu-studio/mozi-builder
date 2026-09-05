#!/usr/bin/env python3
"""Destructive only to the named mozi-v2-poc fixture: scales/restarts its services.
Requires the stack to be up. Restores stopped services and API replica count on failure.
"""
import json
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.request
import uuid

COMPOSE = Path(__file__).with_name('compose.yaml')
PREFIX = ['docker', 'compose'] if subprocess.run(['docker', 'compose', 'version'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0 else ['docker-compose']
BASE = 'http://127.0.0.1:19080'
DKRON = 'http://127.0.0.1:18081/v1'

def compose(*args):
    return subprocess.check_output(PREFIX + ['-f', str(COMPOSE), *args], text=True)

def request(url, method='GET', body=None, headers=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method, headers={'Content-Type': 'application/json', **(headers or {})})
    with urllib.request.urlopen(req, timeout=6) as response:
        raw = response.read()
        return json.loads(raw) if raw else None

def wait(label, predicate, seconds=45):
    deadline = time.monotonic() + seconds
    last = None
    while time.monotonic() < deadline:
        try:
            value = predicate()
            if value:
                print('PASS', label, flush=True)
                return value
        except (OSError, ValueError, urllib.error.HTTPError) as error:
            last = error
        time.sleep(1)
    raise AssertionError(f'{label} timed out: {last}')

def health():
    result = request(BASE + '/poc/health')
    return result if result['rpc_status'] == 'SERVING' else None

def run_job(key, fail_first=False, schedule='@every 1h', manual=True):
    job = {'name': key, 'schedule': schedule, 'timezone': 'Asia/Shanghai', 'disabled': False,
           'concurrency': 'forbid', 'retries': 1, 'tags': {'server': 'true:1'}, 'executor': 'http',
           'executor_config': {'method': 'POST', 'url': 'http://api:8080/poc/jobs/run' + ('?fail_first=1' if fail_first else ''),
               'headers': json.dumps(['Idempotency-Key: ' + key]), 'timeout': '3', 'expectCode': '200'}}
    request(DKRON + '/jobs', 'POST', job)
    if manual:
        request(DKRON + '/jobs/' + key + '/run', 'POST')
    return wait('Dkron business result ' + key, lambda: status(key))

def status(key):
    result = request(BASE + '/poc/jobs/status?id=' + key)
    return result if result['completed'] else None

def main():
    jobs = []
    try:
        wait('APISIX -> go-zero HTTP -> discovered RPC', health, 90)
        compose('up', '-d', '--no-build', '--scale', 'api=2', 'api')
        instances = set()
        def both():
            result = health()
            if result:
                instances.add(result['instance'])
            return len(instances) >= 2
        wait('new HTTP instance reaches APISIX', both)
        compose('up', '-d', '--no-build', '--scale', 'api=1', 'api')
        time.sleep(12)  # Bound > the fixture's 10-second registration lease.
        for _ in range(5):
            assert health()
        print('PASS scale-down removes expired instance', flush=True)
        compose('stop', 'registry')
        for _ in range(3):
            assert health()
            time.sleep(1)
        print('PASS registry outage preserves gateway and RPC cached endpoints', flush=True)
        compose('start', 'registry')
        wait('registry recovery', health)
        compose('restart', 'controller')
        wait('controller restart recovery', health)
        compose('up', '-d', '--no-build', '--scale', 'api=0', 'api')
        def disabled_route():
            try:
                request(BASE + '/poc/health')
            except urllib.error.HTTPError as error:
                return error.code == 404
            return False
        wait('authoritative empty registry disables route', disabled_route)
        compose('up', '-d', '--no-build', '--scale', 'api=1', 'api')
        wait('new registration re-enables route after controller restart', health)
        wait('Dkron API available', lambda: request(DKRON + '/members'), 60)
        key = 'mozi-poc-retry-' + uuid.uuid4().hex[:10]; jobs.append(key)
        result = run_job(key, fail_first=True)
        assert result['attempts'] >= 2, result
        duplicate = request(BASE + '/poc/jobs/run', 'POST', {}, {'Idempotency-Key': key})
        assert duplicate['duplicate'] is True, duplicate
        print('PASS HTTP executor retry retains configured logical execution ID', flush=True)
        timeout_key = 'mozi-poc-timeout-' + uuid.uuid4().hex[:10]; jobs.append(timeout_key)
        request(DKRON + '/jobs', 'POST', {
            'name': timeout_key, 'schedule': '@every 1h', 'timezone': 'UTC',
            'disabled': False, 'concurrency': 'forbid', 'retries': 0,
            'tags': {'server': 'true:1'}, 'executor': 'http',
            'executor_config': {'method': 'POST', 'url': 'http://api:8080/poc/jobs/run?slow=1',
                'headers': json.dumps(['Idempotency-Key: ' + timeout_key]), 'timeout': '1', 'expectCode': '200'}})
        request(DKRON + '/jobs/' + timeout_key + '/run', 'POST')
        def timed_out():
            executions = request(DKRON + '/jobs/' + timeout_key + '/executions')
            return any(not row.get('success') and ('timeout' in row.get('output', '').lower() or 'deadline' in row.get('output', '').lower()) for row in executions)
        wait('Dkron timeout recorded as failure', timed_out)
        assert status(timeout_key) is None
        scheduled = 'mozi-poc-scheduled-' + uuid.uuid4().hex[:10]; jobs.append(scheduled)
        run_job(scheduled, schedule='@every 3s', manual=False)
        request(DKRON + '/jobs/' + scheduled, 'DELETE')
        jobs.remove(scheduled)
        print('PASS recurring timer invokes HTTP executor (single observed firing)', flush=True)
    finally:
        for job in jobs:
            try:
                request(DKRON + '/jobs/' + job, 'DELETE')
            except OSError:
                pass
        compose('start', 'registry', 'controller')
        compose('up', '-d', '--no-build', '--scale', 'api=1', 'api')

if __name__ == '__main__':
    main()
