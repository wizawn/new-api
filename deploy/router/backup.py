#!/usr/bin/env python3
"""复用已有加密异地仓库；暂存仅在 /run，结束后由 systemd 删除。"""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import sqlite3
import subprocess
from datetime import datetime, timezone

root = Path('/opt/new-api-router')
stage = Path('/run/new-api-router-backup/snapshot')
stage.mkdir(mode=0o700)
docker = ['docker', '--host', 'unix:///run/redeemai-rehearsal-docker/docker.sock']
for name, command in [
    ('database.dump', ['pg_dump', '-U', 'newapi', '-d', 'newapi', '-Fc']),
    ('roles.sql', ['pg_dumpall', '-U', 'newapi', '--globals-only']),
]:
    with (stage / name).open('wb') as output:
        subprocess.run(docker + ['exec', 'new-api-router-postgres-1'] + command,
                       stdout=output, check=True, timeout=120)

# 管理员初始密码仅由操作者本机保管，不列入服务端备份。
sources = [root / name for name in ('.env', 'compose.yaml', 'image-lock.json',
    'network.py', 'new-api-router.service', 'tools-server', 'backup.py', 'waf-policy.json')]
sources.extend(root / name for name in ('new-api-router-backup.service',
    'new-api-router-backup.timer', 'new-api-router-tools.service', 'new-api-router-tools.socket'))
for folder in ('tls', 'data', 'tools-data'):
    sources.extend(p for p in (root / folder).rglob('*') if p.is_file() and 'geoip' not in p.parts)
for source in sources:
    if source.is_symlink():
        raise RuntimeError('备份源不允许符号链接: ' + str(source))
    if source.name.endswith(('-wal', '-shm')):
        continue
    target = stage / 'runtime' / source.relative_to(root)
    target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    with source.open('rb') as handle:
        is_sqlite = handle.read(16) == b'SQLite format 3\x00'
    if is_sqlite:
        with sqlite3.connect(source.as_uri() + '?mode=ro', uri=True) as src, sqlite3.connect(target) as dst:
            src.backup(dst)
    else:
        before = source.stat()
        shutil.copy2(source, target)
        after = source.stat()
        if (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns):
            raise RuntimeError('备份期间文件发生变化: ' + str(source))

manifest = {'utc': datetime.now(timezone.utc).isoformat(), 'files': {
    str(p.relative_to(stage)): hashlib.sha256(p.read_bytes()).hexdigest()
    for p in stage.rglob('*') if p.is_file()}}
payload = (json.dumps(manifest, sort_keys=True) + '\n').encode()
(stage / 'manifest.json').write_bytes(payload)
spec = importlib.util.spec_from_file_location('existing_dr', '/var/lib/redeemai-gdrive-direct/gdrive-direct.py')
dr = importlib.util.module_from_spec(spec)
spec.loader.exec_module(dr)
command = dr.restic_args()
environment = dr.restic_env(dr.ROOT)
result = subprocess.check_output(command + ['backup', '--tag', 'new-api-router', '--host', 'cdkey',
    '--json', str(stage)], env=environment, timeout=600)
summary = next(json.loads(line) for line in result.splitlines()
               if json.loads(line).get('message_type') == 'summary')
snapshot = summary['snapshot_id']
readback = subprocess.check_output(command + ['dump', snapshot, str(stage / 'manifest.json')],
                                  env=environment, timeout=120)
if readback != payload:
    raise RuntimeError('异地快照清单回读不一致')
receipt = {'utc': manifest['utc'], 'snapshot_id': snapshot, 'manifest_verified': True,
           'file_count': len(manifest['files']), 'staging': 'tmpfs /run; removed on service exit'}
(root / 'backup-last-success.json').write_text(json.dumps(receipt, indent=2) + '\n')
print(json.dumps(receipt))
