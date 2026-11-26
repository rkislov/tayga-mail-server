#!/usr/bin/env python3
"""Provision a PostgreSQL schema for Boards or Playbooks. Stop TMS before merging old databases."""

import argparse
import fcntl
import json
import os
from pathlib import Path
import re
import secrets
import tempfile
from urllib.parse import urlsplit, urlunsplit

from product_db import (bind_shared_role, dump_schema, local_dsn, provision_role,
                        provision_shared_database, restore_schema, role_name, shared_name, sql)

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--plugin-directory', type=Path, required=True)
parser.add_argument('--tenant-id', required=True)
parser.add_argument('--domain-id', required=True)
mode = parser.add_mutually_exclusive_group(required=True)
mode.add_argument('--plugin', choices=['focalboard', 'playbooks'])
mode.add_argument('--merge-existing', action='store_true', help='copy legacy plugin databases to a new shared database')
parser.add_argument('--postgres-port', type=int, default=5432)
args = parser.parse_args()
port = args.postgres_port
if not 1 <= port <= 65535:
    parser.error('Invalid PostgreSQL port')
for identifier in (args.tenant_id, args.domain_id):
    if not re.fullmatch(r'[A-Za-z0-9_-]{1,128}', identifier):
        parser.error('Use internal tenant/domain IDs, not display names')
folder = args.plugin_directory.resolve(strict=True)
owner = folder.stat()
if owner.st_mode & 0o022:
    raise SystemExit('Plugin directory must not be writable by other accounts')
scope = f'domain:{args.tenant_id}:{args.domain_id}'
database = shared_name(scope)
binding = folder / '.databases.json'


def write_binding(destination, entries, *, replace):
    fd, temporary = tempfile.mkstemp(prefix='.databases-', dir=folder)
    try:
        with os.fdopen(fd, 'w') as out:
            json.dump(entries, out, indent=2)
            out.write('\n')
            out.flush()
            os.fsync(out.fileno())
        if os.geteuid() == 0:
            os.chown(temporary, owner.st_uid, owner.st_gid)
        if replace:
            os.replace(temporary, destination)
        else:
            os.link(temporary, destination)
    finally:
        Path(temporary).unlink(missing_ok=True)
    fd = os.open(folder, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def exists(db):
    return sql(port, 'postgres', f"SELECT 1 FROM pg_database WHERE datname='{db}';") == '1'


def merge(entries, current):
    if not current:
        raise RuntimeError('No existing product bindings in this domain')
    if all(urlsplit(row['dsn']).path == f'/{database}' for row in current):
        print(f'Already using shared product database for {scope}.')
        return
    if exists(database):
        raise RuntimeError('Shared destination already exists; refusing to overwrite it')
    sources = {}
    for row in current:
        plugin = row['plugin_id']
        if plugin in sources or plugin not in ('focalboard', 'playbooks'):
            raise RuntimeError('Invalid or duplicate legacy product binding')
        uri = urlsplit(row['dsn'])
        role = role_name(scope, plugin)
        if (uri.scheme != 'postgres' or uri.path != f'/{role}' or uri.username != role or
                uri.hostname not in ('127.0.0.1', 'localhost', '::1') or (uri.port or 5432) != port or not exists(role)):
            raise RuntimeError('Merge requires intact legacy databases on the selected local PostgreSQL instance')
        sources[plugin] = (role, uri)
    names = ','.join(f"'{role}'" for role, _ in sources.values())
    if sql(port, 'postgres', f'SELECT count(*) FROM pg_stat_activity WHERE datname IN ({names});') != '0':
        raise RuntimeError('Stop TMS and any other clients of the legacy databases before proceeding')

    # The deployment stays offline until both copies and private bindings are ready.
    rollback = folder / ('.databases.pre-merge-' + secrets.token_hex(6) + '.json')
    provision_shared_database(port, database)
    stage = None
    try:
        with tempfile.TemporaryDirectory(prefix='tayga-product-merge-') as temporary:
            scratch = Path(temporary)
            for plugin, (role, uri) in sources.items():
                archive = scratch / 'legacy.dump'
                dump_schema(port, role, 'public', archive)
                schema = 'public'
                if plugin == 'focalboard':
                    # PostgreSQL renames schema references without rewriting stored text.
                    stage_name = 'tmsstage_' + secrets.token_hex(10)
                    provision_shared_database(port, stage_name)
                    stage = stage_name
                    restore_schema(port, stage, 'public', archive)
                    sql(port, stage, 'ALTER SCHEMA public RENAME TO boards;')
                    schema = 'boards'
                    archive = scratch / 'boards.dump'
                    dump_schema(port, stage, schema, archive)
                restore_schema(port, database, schema, archive, role)
                if stage:
                    sql(port, 'postgres', f'DROP DATABASE "{stage}";')
                    stage = None
        write_binding(rollback, entries, replace=False)
        migrated = [dict(row, dsn=urlunsplit(sources[row['plugin_id']][1]._replace(path='/'+database)))
                    if row['scope'] == scope else row for row in entries]
        write_binding(binding, migrated, replace=True)
    except Exception:
        # Keep all existing databases and the candidate for inspection. The
        # original binding also remains available after a failed publication.
        if stage:
            print(f'Temporary PostgreSQL database retained: {stage}')
        print(f'Shared candidate database retained: {database}')
        if rollback.exists():
            print(f'Original private binding: {rollback.name}')
        raise
    print(f'Merged {len(sources)} product database(s) into {database}. Original databases and {rollback.name} retained.')


try:
    lock_fd = os.open(folder / '.databases.lock', os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    with os.fdopen(lock_fd, 'w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        if os.geteuid() == 0:
            os.fchown(lock.fileno(), owner.st_uid, owner.st_gid)
        if binding.exists() or binding.is_symlink():
            info = binding.lstat()
            if binding.is_symlink() or info.st_mode & 0o077 or not binding.is_file():
                raise RuntimeError('Existing bindings must be a private regular file')
            entries = json.loads(binding.read_text())
        else:
            entries = []
        current = [row for row in entries if row['scope'] == scope]
        if args.merge_existing:
            merge(entries, current)
        elif any(row['plugin_id'] == args.plugin for row in current):
            print(f'Already bound: {args.plugin} / {scope}')
        else:
            if any(urlsplit(row['dsn']).path != f'/{database}' for row in current):
                raise RuntimeError('Merge the existing databases of this domain before installing another product')
            if not current:
                provision_shared_database(port, database)
            role = role_name(scope, args.plugin)
            password = secrets.token_urlsafe(40)
            provision_role(port, role, password)
            bind_shared_role(port, database, role, args.plugin)
            entries.append({'scope': scope, 'plugin_id': args.plugin,
                            'dsn': local_dsn(port, database, role, password)})
            write_binding(binding, entries, replace=True)
            print(f'Provisioned {args.plugin} / {scope} in shared database {database}. Restart TMS to load it.')
except (RuntimeError, OSError, ValueError, KeyError) as error:
    # Paths and PostgreSQL errors can contain database credentials.
    raise SystemExit(str(error) if isinstance(error, RuntimeError) else 'Product database configuration failed')
