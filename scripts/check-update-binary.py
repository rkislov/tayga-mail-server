#!/usr/bin/env python3
"""Exercise the updater lifecycle without systemd, root or a real installation."""
import hashlib
import os
from pathlib import Path
import subprocess
import tempfile

SCRIPT = Path(__file__).resolve().parents[1] / 'deploy/systemd/update-binary.sh'

def write(path, body):
    path.write_text(body)
    path.chmod(0o755)

with tempfile.TemporaryDirectory(prefix='tayga-updater-test-') as name:
    root = Path(name)
    fake = root / 'tools'
    fake.mkdir()
    install = root / 'install'
    (install / 'bin').mkdir(parents=True)
    (install / 'config').mkdir()
    (install / 'config/tayga.yaml').write_text('storage: {}\n')
    state = root / 'active'
    log = root / 'systemctl.log'
    target = install / 'bin/tayga-mail'
    new = root / 'tayga-mail'
    old_script = '''#!/bin/sh
if [ "$1" = version ]; then echo 'tayga-mail 0.10.12 (old)'; exit 0; fi
if [ "$1" = backup-full ]; then
 [ "${TEST_BACKUP_FAIL:-0}" = 0 ] || exit 17
 while [ "$#" -gt 0 ]; do if [ "$1" = -out ]; then shift; printf 'encrypted-fixture' > "$1"; exit 0; fi; shift; done
fi
exit 1
'''
    write(target, old_script)
    write(new, "#!/bin/sh\necho 'tayga-mail 1.0.1 (new)'\n")
    write(fake / 'uname', '#!/bin/sh\necho Linux\n')
    write(fake / 'id', '#!/bin/sh\necho 0\n')
    write(fake / 'sleep', '#!/bin/sh\nexit 0\n')
    write(fake / 'systemctl', '''#!/usr/bin/env python3
import os,sys
from pathlib import Path
p=Path(os.environ['TEST_STATE']);args=sys.argv[1:]
if args[0]=='show':print('{ path='+os.environ['TAYGA_BINARY']+' ; argv[]=tayga-mail ; }')
elif args[0]=='is-active':sys.exit(0 if p.exists() else 3)
else:
 with open(os.environ['TEST_LOG'],'a') as f:f.write(args[0]+'\\n')
 if args[0]=='stop':p.unlink(missing_ok=True)
 if args[0] in ['start','restart']:p.touch()
''')
    write(fake / 'curl', '''#!/bin/sh
if [ "${TEST_READY_FAIL:-0}" = 1 ] && grep -q '1.0.1' "$TAYGA_BINARY"; then exit 22; fi
exit 0
''')
    write(fake / 'readlink', '#!/usr/bin/env python3\nimport os,sys\nprint(os.path.realpath(sys.argv[-1]))\n')
    write(fake / 'stat', '''#!/usr/bin/env python3
import os,sys
s=os.stat(sys.argv[-1]);print({'%u':s.st_uid,'%g':s.st_gid,'%a':oct(s.st_mode&0o777)[2:]}[sys.argv[2]])
''')
    write(fake / 'chmod', '#!/usr/bin/env python3\nimport os,sys\nargs=[x for x in sys.argv[1:] if x!="--"];os.chmod(args[1],int(args[0],8))\n')
    write(fake / 'du', '#!/bin/sh\nprintf "1024\\tfixture\\n"\n')
    write(fake / 'df', '#!/bin/sh\nprintf "Filesystem 1-blocks Used Available Capacity Mounted\\nfixture 9999999999 0 9999999999 0%% /\\n"\n')
    write(fake / 'sha256sum', '''#!/usr/bin/env python3
import hashlib,sys
for path in sys.argv[1:]:print(hashlib.sha256(open(path,'rb').read()).hexdigest()+'  '+path)
''')
    backup = root / 'backup/full.age'
    env = dict(os.environ, PATH=str(fake)+os.pathsep+os.environ['PATH'], TAYGA_NEW_BINARY=str(new), TAYGA_ROOT=str(install), TAYGA_CONFIG=str(install/'config/tayga.yaml'), TAYGA_BINARY=str(target), TEST_STATE=str(state), TEST_LOG=str(log), TAYGA_BACKUP_FILE=str(backup), TAYGA_RECIPIENT='age1-test-public-recipient', TAYGA_READY_URL='http://fixture/readyz', TAYGA_EXPECTED_SHA256=hashlib.sha256(new.read_bytes()).hexdigest())
    def run(mode, **overrides):
        return subprocess.run(['bash', str(SCRIPT), mode], env=dict(env, **overrides), capture_output=True, text=True)
    state.touch()
    result = run('--backup', TEST_BACKUP_FAIL='1')
    assert result.returncode == 17, result.stderr
    assert state.exists() and log.read_text().splitlines() == ['stop', 'start']
    assert target.read_text() == old_script
    result = run('--backup')
    assert result.returncode == 0, result.stderr
    assert backup.read_bytes() == b'encrypted-fixture' and state.exists()
    env['TAYGA_BACKUP_SHA256'] = hashlib.sha256(backup.read_bytes()).hexdigest()
    result = run('--apply', TAYGA_EXPECTED_SHA256='0'*64)
    assert result.returncode != 0 and target.read_text() == old_script
    result = run('--apply')
    assert result.returncode == 0, result.stderr
    assert target.read_bytes() == new.read_bytes() and state.exists()
    assert list((install/'bin').glob('tayga-mail.before-1.0.1.*'))
    # Readiness failure retains the full snapshot and never silently restores a
    # previous binary over a potentially migrated database.
    write(target, old_script)
    result = run('--apply', TEST_READY_FAIL='1')
    assert result.returncode != 0 and target.read_bytes() == new.read_bytes()
    assert backup.exists() and 'database may have migrated' in result.stderr
print('Updater: failed backup restarts old service, hashes reject changes, atomic install preserves prior binary, readiness failure retains recovery evidence — OK')
