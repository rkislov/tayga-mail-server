#!/usr/bin/env python3
"""Run isolated ItemAttachment server checks; never claim Outlook acceptance."""
import argparse
import datetime
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
CHECKS = {
    "github.com/tayga/tms/internal/flowsync": [
        "TestEWSCalendarItemAttachmentLifecycle",
        "TestEWSStructuredMessageAttachment",
        "TestEWSInvalidItemAttachmentsAreAtomic",
        "TestEWSItemAttachmentStorageFailureRollsBack",
        "TestEWSStructuredItemAttachmentsLifecycle",
        "TestEWSStructuredItemRejectionIsAtomic",
        "TestEWSAttachedExtendedPropertiesRoundTrip",
        "TestEWSAttachedExtendedValueTypes",
        "TestEWSAttachedBadExtendedPropertiesRejectBatch",
        "TestEWSAttachedTaskRecurrenceRoundTrip",
        "TestEWSAttachedTaskBadRecurrenceIsAtomic",
        "TestEWSDeleteCalendarAttachmentBatchAndAuthorization",
    ],
    "github.com/tayga/tms/internal/storage": [
        "TestCalendarEmbeddedMigrationKeepsAttachedMail",
    ],
}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True, help="JSON report path")
    args = parser.parse_args()
    names = [name for tests in CHECKS.values() for name in tests]
    command = ["go", "test", "-json", "./internal/flowsync", "./internal/storage",
               "-run", "^(" + "|".join(names) + ")$", "-count=1", "-timeout=5m"]
    env = dict(os.environ, CGO_ENABLED="0")
    completed = subprocess.run(command, cwd=ROOT, env=env, text=True,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    outcomes = {}
    failed_packages = []
    for line in completed.stdout.splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        package, name, action = event.get("Package"), event.get("Test"), event.get("Action")
        if name and action in ("pass", "fail", "skip"):
            outcomes[(package, name)] = action
        elif not name and action == "fail":
            failed_packages.append(package)
    checks = [{"package": package, "test": name,
               "status": outcomes.get((package, name), "not_run")}
              for package, tests in CHECKS.items() for name in tests]
    passed = completed.returncode == 0 and not failed_packages and all(
        check["status"] == "pass" for check in checks)
    revision = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True,
                              stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    report = {
        "schema_version": 1,
        "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "revision": revision.stdout.strip() if revision.returncode == 0 else "unknown",
        "scope": "isolated_server_tests",
        "server_status": "pass" if passed else "fail",
        "client_acceptance": "not_run",
        "client_acceptance_note": "A real Outlook session is required; SOAP tests do not certify client compatibility.",
        "production_access": False,
        "checks": checks,
        "failed_packages": failed_packages,
        "go_exit_code": completed.returncode,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(f"ItemAttachment server checks: {report['server_status']} ({sum(c['status'] == 'pass' for c in checks)}/{len(checks)})")
    print(f"Report: {args.output.resolve()}")
    print("Outlook client acceptance: not run")
    if not passed:
        for check in checks:
            if check["status"] != "pass":
                print(f"{check['status']}: {check['test']}", file=sys.stderr)
        # Keep the JSON report free of SOAP bodies and diagnostic contents.
        print(completed.stderr[-4000:], file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
