#!/usr/bin/env python3
"""Restore product dumps into FRESH local PostgreSQL databases only.

Run after tayga restore-full, on the isolated destination host. Existing roles
or databases are never reused or overwritten. TMS must remain stopped.
"""
import argparse
import base64
import hashlib
import hmac
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import tempfile
from urllib.parse import urlsplit

from product_db import (local_dsn, provision_shared_database, restore_schema,
                        role_name, schema_name, shared_name)

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--root', type=Path, required=True)
p.add_argument('--index', required=True, help='relative .product-dumps-*/restore.json path from restored snapshot')
p.add_argument('--postgres-port', type=int, default=5432)
a = p.parse_args()
if not 1 <= a.postgres_port <= 65535:
    p.error('Invalid PostgreSQL port')
root = a.root.resolve(strict=True)
def inside(name):
    candidate = root / name
    if Path(name).is_absolute() or '..' in Path(name).parts or not candidate.resolve(strict=True).is_relative_to(root):
        raise SystemExit('Restore path escapes installation')
    if candidate.is_symlink():
        raise SystemExit('Restore paths must not be symlinks')
    return candidate
index_path = inside(a.index)
index = json.loads(index_path.read_text())
binding_path = root / index['bindings']
pending = inside(index['bindings'] + '.restore-pending')
if binding_path.exists() or binding_path.is_symlink():
    raise SystemExit('Live bindings already exist; refusing to replace them')
if pending.stat().st_mode & 0o077:
    raise SystemExit('Pending credentials must be private')
old = json.loads(pending.read_text())
keys = {(x['scope'], x['plugin_id']) for x in old}
rows = index['databases']
if len(keys) != len(old) or len(rows) != len(keys) or {(x['scope'], x['plugin_id']) for x in rows} != keys:
    raise SystemExit('Dump/binding set mismatch')
prepared = []
by_key = {(x['scope'], x['plugin_id']): x for x in old}
for row in rows:
    scope, plugin = row['scope'], row['plugin_id']
    if not re.fullmatch(r'domain:[A-Za-z0-9_-]{1,128}:[A-Za-z0-9_-]{1,128}', scope) or plugin not in ('playbooks', 'focalboard'):
        raise SystemExit('Invalid product scope')
    name = role_name(scope, plugin)
    database = urlsplit(by_key[(scope, plugin)]['dsn']).path.removeprefix('/')
    if database not in (name, shared_name(scope)):
        raise SystemExit('Invalid archived product database name')
    if row['file'] != index['directory'] + '/' + name + '.dump':
        raise SystemExit('Invalid dump name')
    dump = inside(row['file'])
    with dump.open('rb') as f:
        if f.read(5) != b'PGDMP':
            raise SystemExit('Invalid PostgreSQL custom dump')
    prepared.append((row, name, database, dump))
admin = ['sudo', '-u', 'postgres', 'psql', '-X', '-p', str(a.postgres_port), '-v', 'ON_ERROR_STOP=1']
# Preflight ALL names before creating any resource. CREATE still protects races.
names = ','.join("'" + n + "'" for n in sorted({name for _, name, _, _ in prepared} | {db for _, _, db, _ in prepared}))
check = subprocess.run(admin + ['-Atc', f'SELECT rolname FROM pg_roles WHERE rolname IN ({names}) UNION ALL SELECT datname FROM pg_database WHERE datname IN ({names})'], text=True, capture_output=True)
if check.returncode or check.stdout.strip():
    raise SystemExit('Destination must be fresh: existing roles/databases or failed PostgreSQL preflight')
def verifier(password):
    salt = secrets.token_bytes(16)
    salted = hashlib.pbkdf2_hmac('sha256', password.encode(), salt, 4096)
    client = hmac.new(salted, b'Client Key', hashlib.sha256).digest()
    server = hmac.new(salted, b'Server Key', hashlib.sha256).digest()
    b64 = lambda v: base64.b64encode(v).decode()
    return f'SCRAM-SHA-256$4096:{b64(salt)}${b64(hashlib.sha256(client).digest())}:{b64(server)}'
entries = []
shared_created = set()
for row, name, database, dump in prepared:
    password = secrets.token_urlsafe(40)
    sql = f'''CREATE ROLE "{name}" LOGIN PASSWORD '{verifier(password)}' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
'''
    if database == name:
        sql += f'''
CREATE DATABASE "{name}" OWNER "{name}";
REVOKE ALL ON DATABASE "{name}" FROM PUBLIC;
'''
    result = subprocess.run(admin, input=sql, text=True, capture_output=True)
    if result.returncode:
        raise SystemExit('Provisioning failed; bindings remain quarantined. Inspect partial destination resources before retrying.')
    if database == name:
        env = {k: v for k, v in os.environ.items() if not k.startswith('PG')}
        env.update(PGHOST='127.0.0.1', PGPORT=str(a.postgres_port), PGUSER=name, PGDATABASE=name, PGPASSWORD=password, PGSSLMODE='disable', PGCONNECT_TIMEOUT='10')
        result = subprocess.run(['pg_restore', '--no-owner', '--no-acl', '--exit-on-error', '--single-transaction', '--dbname', name, str(dump)], env=env, capture_output=True)
        if result.returncode:
            raise SystemExit('Product restore failed; bindings remain quarantined. No existing databases were overwritten.')
    else:
        try:
            if database not in shared_created:
                provision_shared_database(a.postgres_port, database)
                shared_created.add(database)
            restore_schema(a.postgres_port, database, schema_name(row['plugin_id']), dump, name)
        except RuntimeError as error:
            raise SystemExit(f'Product restore failed; bindings remain quarantined. {error}')
    entries.append({'scope': row['scope'], 'plugin_id': row['plugin_id'],
                    'dsn': local_dsn(a.postgres_port, database, name, password)})
fd, temporary = tempfile.mkstemp(prefix='.databases-restored-', dir=binding_path.parent)
try:
    with os.fdopen(fd, 'w') as f:
        json.dump(entries, f, indent=2)
        f.write('\n')
        f.flush()
        os.fsync(f.fileno())
    if os.geteuid() == 0:
        os.chown(temporary, pending.stat().st_uid, pending.stat().st_gid)
    # Hard-link publication refuses any concurrently created live binding.
    os.link(temporary, binding_path)
finally:
    Path(temporary).unlink(missing_ok=True)
directory_fd = os.open(binding_path.parent, os.O_RDONLY | os.O_DIRECTORY)
try:
    os.fsync(directory_fd)
finally:
    os.close(directory_fd)
pending.unlink()
for _, _, _, dump in prepared:
    dump.unlink()
index_path.unlink()
try:
    index_path.parent.rmdir()
except OSError:
    pass  # Never remove unrelated files from the restored installation.
print(f'Restored {len(entries)} product schemas with fresh isolated credentials. Service was not started.')
