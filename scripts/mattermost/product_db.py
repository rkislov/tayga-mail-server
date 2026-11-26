"""Local PostgreSQL administration shared by the product setup and restore tools."""

import hashlib
import os
import subprocess
from urllib.parse import quote


def role_name(scope, plugin):
    return 'tmsplug_' + hashlib.sha256((scope + '\0' + plugin).encode()).hexdigest()[:24]


def shared_name(scope):
    return 'tmsprod_' + hashlib.sha256(scope.encode()).hexdigest()[:24]


def schema_name(plugin):
    # The native Playbooks migration code queries public explicitly.
    return 'boards' if plugin == 'focalboard' else 'public'


def pg(port, executable, *options, data=None, source=None, output=None):
    env = {key: value for key, value in os.environ.items() if not key.startswith('PG')}
    command = ['sudo', '-u', 'postgres', executable, '-p', str(port), *options]
    # Errors may contain SQL text, credentials, or data. Keep them off the console.
    result = subprocess.run(command, input=data, stdin=source,
                            stdout=output or subprocess.PIPE, stderr=subprocess.PIPE, env=env)
    if result.returncode:
        raise RuntimeError(f'{executable} failed on local PostgreSQL')
    return result.stdout


def sql(port, database, statement):
    return pg(port, 'psql', '-XqAt', '-v', 'ON_ERROR_STOP=1', '-d', database,
              data=statement.encode()).decode().strip()


def provision_shared_database(port, database):
    sql(port, 'postgres', f'CREATE DATABASE "{database}" TEMPLATE template0;')
    try:
        sql(port, 'postgres', f'REVOKE ALL ON DATABASE "{database}" FROM PUBLIC;')
        sql(port, database, 'REVOKE ALL ON SCHEMA public FROM PUBLIC;')
    except Exception:
        sql(port, 'postgres', f'DROP DATABASE "{database}";')
        raise


def provision_role(port, role, password):
    sql(port, 'postgres', f"""
        CREATE ROLE "{role}" LOGIN PASSWORD '{password}'
           NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
    """)


def bind_shared_role(port, database, role, plugin):
    schema = schema_name(plugin)
    if schema == 'public':
        schema_sql = f'ALTER SCHEMA public OWNER TO "{role}";'
    else:
        schema_sql = f'CREATE SCHEMA boards AUTHORIZATION "{role}";'
    sql(port, database, f"""
        BEGIN;
        {schema_sql}
        REVOKE ALL ON SCHEMA {schema} FROM PUBLIC;
        GRANT CONNECT, TEMPORARY ON DATABASE "{database}" TO "{role}";
        ALTER ROLE "{role}" IN DATABASE "{database}" SET search_path TO {schema};
        COMMIT;
    """)


def dump_schema(port, database, schema, archive):
    with archive.open('wb') as output:
        pg(port, 'pg_dump', '--format=custom', '--no-owner', '--no-acl',
           '--schema=' + schema, '--dbname=' + database, output=output)


def restore_schema(port, database, schema, archive, role=None):
    # pg_restore --schema restores objects inside the namespace. The schema
    # must already exist, and its login needs no database-wide CREATE privilege.
    if role:
        bind_shared_role(port, database, role, 'focalboard' if schema == 'boards' else 'playbooks')
    with archive.open('rb') as source:
        args = ['--no-owner', '--no-acl', '--exit-on-error', '--single-transaction',
                '--schema=' + schema, '--dbname=' + database]
        if role:
            args.append('--role=' + role)
        pg(port, 'pg_restore', *args, source=source)


def local_dsn(port, database, role, password):
    return f'postgres://{role}:{quote(password, safe="")}@127.0.0.1:{port}/{database}?sslmode=disable'
