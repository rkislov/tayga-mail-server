# Original Boards and Tayga Playbooks

Available in TMS 1.2.0 as **experimental, separately installed modules**. Isolated
functional acceptance has passed. Advanced host integrations and native mobile
UX still need acceptance before this can be called complete Mattermost product
compatibility. An ordinary TMS upgrade does not install PostgreSQL, install plugin
bundles or enable products automatically.

## Distribution and licensing

- Boards: untouched official `focalboard` 10.1.0 compiled distribution. Its binary
  distribution is MIT-licensed. Source modifications have separate AGPL/commercial
  terms; TMS does not relicense the vendor source.
- Playbooks: [Tayga fork and binaries](https://github.com/rkislov/tms-playbooks/releases/tag/v2.9.7-tms.1), pinned upstream
  v2.9.7 / `47e06734cd71cfe0b01f57749aa116ac04006022`. The Apache-2.0 community
  distribution excludes `server/enterprise` and supplies independently written
  feature-availability policy. Original UI and core application remain upstream.
- Production and community acceptance use no Mattermost license and no development
  mode. `GetLicense()` remains nil. Preserve upstream license/notice files.
- Product IDs remain `focalboard` and `playbooks`; original and forked Playbooks
  cannot coexist under the same ID in one scope.

References: [Boards license](https://github.com/mattermost/mattermost-plugin-boards/blob/v10.1.0/LICENSE.txt),
[Playbooks license](https://github.com/mattermost/mattermost-plugin-playbooks/blob/v2.9.7/README.md#license).

## Tenant/domain isolation

Each `domain:<internal tenant ID>:<internal domain ID>` receives its own plugin
runtime, KV namespace, local file directory, PostgreSQL database and nonprivileged
login. Plugin SQL never runs against the TMS mail/account database. The host mirrors
only users, teams, channels, membership history and posts belonging to that domain.
Disabled users and foreign-domain requests are rejected before entering a plugin.
Database roles must have no memberships, superuser, create-role/database, replication
or RLS-bypass privileges. No role/database fallback to the mail store is allowed.

The TMS menu discovers enabled products for the current authenticated domain.
Original React interfaces run under `/mm/<scope hash>/boards` and `/playbooks`.
Scoped bootstrap/API calls authenticate with the current TMS bearer token. A short
HttpOnly, SameSite=Strict cookie permits original image/download GET requests only;
it never authorizes writes. JWT expiry/revocation is checked on every request.
Account changes remove old frames and cookies.

Native plugins remain trusted server-side code running as the TMS service user;
these scope checks are not an OS sandbox for hostile plugin executables. Install
only operator-reviewed binaries. A tenant must not receive permission to upload
arbitrary native code on the assumption that SQL scoping alone isolates it.

## Provisioning

The release archives include helpers under `scripts/mattermost/`. DEB/RPM installs
them under `/usr/share/tayga-mail/mattermost/`, with this guide under
`/usr/share/doc/tayga-mail/`. Provisioning and restore helpers require Python 3.

1. Install PostgreSQL and matching `pg_dump`/`pg_restore` tools. Keep PostgreSQL
   private, preferably on the same host. Remote bindings require verified TLS.
2. Stop TMS and back up the installation. Create/protect the plugin directory.
3. For **each** domain and each product, run the provisioning script as an operator
   with local `sudo -u postgres psql` access. Example (IDs, not domain names):

   ```sh
   python3 scripts/mattermost/provision-product-db.py \
     --plugin-directory /var/lib/tayga/mattermost-plugins \
     --tenant-id INTERNAL_TENANT_ID --domain-id INTERNAL_DOMAIN_ID \
     --plugin playbooks
   ```

   Repeat with `--plugin focalboard`. The script creates a dedicated role/database
   and writes private mode-0600 `.databases.json`; passwords never enter Git or
   command arguments. TMS must be able to read this file. Existing resources are
   never overwritten.
4. Install the matching native plugin bundles with TMS plugin management, enable
   them for the intended domain, then restart TMS to load database bindings.
5. The fork's bundle includes the shared `webapp/dist/tms-host.js` runtime needed
   to host both original interfaces. Build it with `scripts/build-tms.sh`; do not
   substitute a vendor Playbooks bundle without that host asset.

## Backup and restore

`backup-full` now dumps all bound product databases using `pg_dump --format=custom`
while the installation's exclusive offline maintenance lock fences plugin writers.
Logical dumps are staged in a private directory, included in the encrypted archive,
and removed on success/failure. Missing dump tools or inaccessible databases fail
backup rather than silently omit product data. Product snapshots use format v2,
which older TMS restore binaries reject; regular SQLite-only v1 snapshots remain
readable. Main PostgreSQL and S3 installations
still require their existing external backup procedure.

`restore-full` verifies file digests and custom-dump signatures and quarantines
restored connection credentials as `.databases.json.restore-pending`. It never
contacts the source PostgreSQL server or starts services. On the destination host,
with TMS stopped and a **fresh isolated PostgreSQL instance**, run:

```sh
python3 scripts/mattermost/restore-product-dbs.py \
  --root /path/to/restored-installation \
  --index .product-dumps-XXXXXX/restore.json
```

Use the actual dump directory printed/stored in the snapshot header. Restoration
creates fresh restricted roles and runs `pg_restore` as each product's own login,
then publishes new private bindings. Existing database/role names or live bindings
cause refusal. A partial failure leaves bindings quarantined; inspect those newly
created destination resources before retrying. Do not point restore at production.
`verify-full` checks archive contents, not a complete PostgreSQL restore rehearsal.

## Validation performed

- Original Linux ARM64 Boards bundle: activation/migrations/template import,
  board creation/read, card creation/read/update, anonymous denial.
- Community Playbooks without license/development mode: playbook and checklist,
  run creation, task completion and persisted run state.
- Browser, synthetic local installation: original Playbooks private-playbook
  creation, run launch, task completion and run finish; original Boards welcome,
  templates, board/card creation and persistence after page reload.
- Host tests: scoped SQL handles, transactions, plugin/domain property separation,
  stable property upserts, role evaluation, membership leave/rejoin history,
  cross-domain bulk profile/channel filtering, read-only cookies and revoked JWTs.
- Frontend tests: token renewal preserves the frame, identity change/logout removes
  it, cross-origin catalog URLs rejected, stale requests cannot revive logged-out UI.
- Backup fixture: real PostgreSQL logical dump in encrypted archive; restored
  credentials quarantined. Separate restore-script rehearsal preserves a card and
  rejects overwriting an existing destination.

Optional acceptance requires disposable localhost PostgreSQL and pinned bundle
hashes: see `TestOriginalProductBundles`, `TestProductBrowserStand` and
`TestFullProductDumpQuarantinesRestoredConnections`. Ordinary tests skip binary
acceptance unless configured; a normal green test run alone is insufficient.

## Remaining acceptance before production

- Advanced original-product actions: sharing changes, reminders/notifications,
  uploads/exports, reports, integrations and full UI permission matrix.
- The host sanitizes rendered Markdown at both formatting and component boundaries
  and supplies a basic textbox/timestamp adapter. Rich Mattermost editor
  behavior and chat-specific registered components are not fully hosted.
- Unsupported SDK methods return explicit errors. In particular, new linked
  property-field APIs and some channel/post mutation methods are not implemented.
- Boards vendor enterprise compliance endpoints still retain vendor license checks;
  the MIT binary is not a promise that every enterprise server integration exists.
- Metadata mirror refresh currently copies a domain's posts per plugin request;
  incremental synchronization/load validation is needed for large installations.
- Original mobile web UI has upstream beta limitations. Native iPhone plugin UX has
  not been accepted. This does not change the previously working Mattermost chat.

## Language and shortcuts in Tayga Chat

The product panes use the language chosen in the Tayga header. Russian
localization covers the default Boards 10.1 template catalog and Board
copies, and the presets bundled in the Tayga Playbooks web client.
Team templates and other user content remain as authored. The matching
Playbooks web bundle and its shared Tayga host are built from
[the localized fork](https://github.com/rkislov/tms-playbooks/tree/6e0cd6a20a98705d8a9f82a793e250a97e343733).

The mail and post action menus provide private board and playbook
creation for enabled products. The title is proposed from the subject
or text. Mail discussion opens a draft in a selected channel. The chat
action opens the reply composer for the selected thread.
