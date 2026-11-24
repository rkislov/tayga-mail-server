#!/usr/bin/env python3
"""Provision a dedicated plugin/domain PostgreSQL role and private TMS binding.

Run on the PostgreSQL/TMS host as an operator who can sudo -u postgres psql.
Never pass passwords on the command line. This script generates its own secret.
"""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import tempfile
from urllib.parse import quote

parser = argparse.ArgumentParser()
parser.add_argument('--plugin-directory', type=Path, required=True)
parser.add_argument('--tenant-id', required=True)
parser.add_argument('--domain-id', required=True)
parser.add_argument('--plugin', choices=['focalboard', 'playbooks'], required=True)
args = parser.parse_args()
for identifier in (args.tenant_id, args.domain_id):
    if not re.fullmatch(r'[A-Za-z0-9_-]{1,128}', identifier):
        parser.error('Use internal tenant/domain IDs, not display names')
folder = args.plugin_directory.resolve(strict=True)
owner = folder.stat()
if owner.st_mode & 0o022:
    raise SystemExit('Plugin directory must not be writable by other accounts')
scope = f'domain:{args.tenant_id}:{args.domain_id}'
name = 'tmsplug_' + hashlib.sha256((scope + '\x00' + args.plugin).encode()).hexdigest()[:24]
binding = folder / '.databases.json'
lock_fd = os.open(folder / '.databases.lock', os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
with os.fdopen(lock_fd, 'w') as lock:
    fcntl.flock(lock, fcntl.LOCK_EX)
    if binding.exists():
        info = binding.lstat()
        if binding.is_symlink() or info.st_mode & 0o077:
            raise SystemExit('Existing bindings must be a private regular file')
        entries = json.loads(binding.read_text())
    else:
        entries = []
    if any(row['scope'] == scope and row['plugin_id'] == args.plugin for row in entries):
        print(f'Already bound: {args.plugin} / {scope}')
        raise SystemExit(0)
    password = secrets.token_urlsafe(40)
    sql = f'''CREATE ROLE "{name}" LOGIN PASSWORD '{password}' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
CREATE DATABASE "{name}" OWNER "{name}";
REVOKE ALL ON DATABASE "{name}" FROM PUBLIC;
'''
    result = subprocess.run(['sudo', '-u', 'postgres', 'psql', '-X', '-v', 'ON_ERROR_STOP=1'], input=sql, text=True, capture_output=True)
    if result.returncode:
        # psql errors can echo the SQL statement including the generated secret.
        raise SystemExit('PostgreSQL provisioning failed; inspect server logs without exposing credentials. Existing roles/databases were not overwritten.')
    entries.append({'scope': scope, 'plugin_id': args.plugin, 'dsn': f'postgres://{name}:{quote(password)}@127.0.0.1:5432/{name}?sslmode=disable'})
    fd, temporary = tempfile.mkstemp(prefix='.databases-', dir=folder)
    try:
        with os.fdopen(fd, 'w') as out:
            json.dump(entries, out, indent=2)
            out.write('\n')
            out.flush()
            os.fsync(out.fileno())
        if os.geteuid() == 0:
            os.chown(temporary, owner.st_uid, owner.st_gid)
            os.fchown(lock.fileno(), owner.st_uid, owner.st_gid)
        os.replace(temporary, binding)
    finally:
        Path(temporary).unlink(missing_ok=True)
print(f'Provisioned {args.plugin} / {scope}: {name}; private binding saved. Restart TMS to load it.')
