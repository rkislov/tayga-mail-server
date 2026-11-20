// Run the actual extension in an isolated Thunderbird profile against a local fixture API.
import { mkdtemp, mkdir, readFile, writeFile, cp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "node:http";
import { spawn } from "node:child_process";
const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const temp = await mkdtemp(join(tmpdir(), "tayga-thunderbird-smoke-"));
let processHandle;
let resolveResult;
let result = new Promise((resolve) => {
  resolveResult = resolve;
});
let vacation = { enabled: false, subject: "", body: "" };
let scripts = [];
let logins = 0, refreshes = 0;
let accessToken = "fixture-token";
const staged = new Map();
let lostChunkResponse = false, completedBytes = 0, uploadSessions = 0, pauseUpload = false;
const server = createServer(async (req, res) => {
  const chunks = [];
  for await (const chunk of req) chunks.push(chunk);
  const text = Buffer.concat(chunks).toString();
  let body = {};
  try {
    body = JSON.parse(text || "{}");
  } catch {}
  res.setHeader("Content-Type", "application/json");
  const reply = (value) => res.end(JSON.stringify(value));
  if (req.url === "/result") {
    resolveResult(body);
    return reply({ ok: true });
  }
  if (req.url === "/test/auth-counts") return reply({ logins, refreshes, completedBytes, staged: staged.size, uploadSessions });
  if (req.url === "/test/pause-upload") { pauseUpload = true; return reply({ ok: true }); }
  if (["/api/v1/auth/login", "/api/v1/auth/token"].includes(req.url)) {
    if (req.url.endsWith("/login")) logins++; else {
      refreshes++;
      accessToken = "fixture-token-" + refreshes;
    }
    return reply({
      tokens: {
        access_token: accessToken,
        refresh_token: "fixture-refresh-" + refreshes,
      },
    });
  }
  if (req.headers.authorization !== "Bearer " + accessToken) {
    res.statusCode = 401;
    return reply({ error: "unauthorized" });
  }
  if (req.url === "/api/v1/files/uploads" && req.method === "POST") {
    const record = { id: crypto.randomUUID(), path: body.path, size: body.size, offset: 0, complete: false, parts: [] };
    staged.set(record.id, record);
    uploadSessions++;
    return reply({ ...record, parts: undefined });
  }
  if (req.url.startsWith("/api/v1/files/uploads/")) {
    const [id, command] = req.url.slice("/api/v1/files/uploads/".length).split('/');
    const record = staged.get(id);
    if (!record) { res.statusCode = 404; return reply({ error: 'not found' }); }
    const wire = () => ({ ...record, parts: undefined });
    if (req.method === 'DELETE') { staged.delete(id); return reply({ ok: true }); }
    if (req.method === 'PATCH') {
      if (Number(req.headers['upload-offset']) !== record.offset) { res.statusCode = 409; return reply({ error: 'offset mismatch' }); }
      if (pauseUpload && record.offset > 0) { pauseUpload = false; return; }
      const part = Buffer.concat(chunks);
      record.parts.push(part); record.offset += part.length;
      if (!lostChunkResponse) { lostChunkResponse = true; req.socket.destroy(); return; }
      return reply(wire());
    }
    if (command === 'complete') {
      if (record.offset !== record.size || !record.parts.every(part => part.every(byte => byte === 84))) {
        res.statusCode = 400; return reply({ error: 'invalid file bytes' });
      }
      completedBytes = record.size;
      record.complete = true; return reply(wire());
    }
    return reply({ upload: wire() });
  }
  if (req.url === "/api/v1/mail/vacation") {
    if (req.method === "PUT") vacation = body;
    return reply(vacation);
  }
  if (req.url === "/api/v1/sieve/scripts") return reply({ scripts });
  if (req.url.startsWith("/api/v1/sieve/scripts/")) {
    scripts = [{ name: "smoke", script: body.script, active: body.active }];
    return reply(scripts[0]);
  }
  if (req.url.startsWith("/api/v1/files/content?"))
    return reply({ size: text.length });
  if (req.url === "/api/v1/files/shares")
    return reply({
      id: "smoke-share",
      url: `http://127.0.0.1:${server.address().port}/s/smoke`,
    });
  reply({});
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const base = `http://127.0.0.1:${server.address().port}`;
try {
  const profile = join(temp, "profile");
  const extension = join(profile, "extensions", "tayga@mail.local");
  await mkdir(extension, { recursive: true });
  for (const name of [
    "manifest.json",
    "diagnostics.js",
    "diagnostics-ui.js",
    "diagnostics.html",
    "client.js",
    "background.js",
    "options.html",
    "options.js",
    "popup.html",
    "popup.js",
    "styles.css",
    "icon.svg",
    "ews.js",
    "rules.js",
    "invites.js",
    "invites-ui.js",
    "invites.html",
    "rules.html",
    "rules-ui.js",
    "rules.css",
    "experiments/ews/api.js",
    "experiments/ews/schema.json",
  ]) {
    await mkdir(resolve(join(extension, name), ".."), { recursive: true });
    await cp(join(root, name), join(extension, name));
  }
  const manifest = JSON.parse(
    await readFile(join(extension, "manifest.json"), "utf8"),
  );
  manifest.background.scripts.push("smoke-background.js");
  if (process.env.TAYGA_EWS_FIXTURE)
    manifest.permissions.push("compose", "compose.send", "accountsRead");
  await writeFile(join(extension, "manifest.json"), JSON.stringify(manifest));
  await writeFile(
    join(extension, "smoke-background.js"),
    `
    (async () => {
      try {
        await browser.storage.local.set({ewsEnabled:${!!process.env.TAYGA_EWS_FIXTURE}}); if (!browser.cloudFile?.onFileUpload) throw new Error('cloudFile API unavailable');
        await browser.cloudFile.getAllAccounts();
        await browser.tabs.create({url: browser.runtime.getURL('smoke.html')});
      } catch (error) { await fetch(${JSON.stringify(base + "/result")}, {method:'POST', body:JSON.stringify({ok:false,error:error.message})}); }
    })();
  `,
  );
  await writeFile(
    join(extension, "smoke.html"),
    '<!DOCTYPE html><html><head><meta charset="utf-8"></head><body><iframe src="options.html" style="width:100%;height:1200px"></iframe><script src="smoke-ui.js"></script></body></html>',
  );
  const defaultUI = `
    const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
    async function until(test) { for (let i=0;i<200;i++) { if(test()) return; await delay(50); } throw new Error('UI timeout: '+document.querySelector('iframe').contentDocument.getElementById('msg')?.textContent); }
    (async () => { try {
      const frame = document.querySelector('iframe');
      await until(() => frame.contentDocument?.getElementById('login-form'));
      const doc = frame.contentDocument, el = id => doc.getElementById(id);
      const background = await browser.runtime.getBackgroundPage();
      const restart = (await browser.storage.local.get('fileSmokeRestart')).fileSmokeRestart;
      if (restart) {
        await until(() => el('session').textContent.startsWith('Подключено:'));
        await until(() => el('uploads-list').textContent.includes('Загрузка сохранена'));
        const saved = await background.client.pendingUploads();
        if (saved.length !== 1 || saved[0].offset !== 8 * 1024 * 1024) throw new Error('Upload checkpoint did not survive restart: '+JSON.stringify(saved));
        const resumed = new File([new Uint8Array(restart.size).fill(84)], 'large.bin');
        const result = await background.uploadFile({id:'smoke-account'}, {id:78,name:resumed.name,data:resumed});
        if (result.error || !result.url) throw new Error('Resumed upload failed: '+JSON.stringify(result));
        const state = await fetch(${JSON.stringify(base + "/test/auth-counts")}).then(res => res.json());
        if (state.completedBytes !== resumed.size || state.staged !== 0 || state.logins !== 1 || state.uploadSessions !== 1) throw new Error('Resume duplicated data, session or login: '+JSON.stringify(state));
        if ((await background.client.pendingUploads()).length) throw new Error('Completed upload journal not removed');
        const diagnostic = await browser.runtime.sendMessage({type:'tayga.diagnostics.export'});
        const logJSON = JSON.stringify(diagnostic);
        for (const value of ['fixture-token','fixture-refresh','fixture-password','smoke@example.test','large.bin','тест.txt','tb-out/','/s/smoke','/api/v1/files']) {
          if (logJSON.includes(value)) throw new Error('Upload diagnostics leaked private data');
        }
        if (!diagnostic.entries.some(entry=>entry.event==='upload') || !diagnostic.entries.some(entry=>entry.event==='upload.retry')) throw new Error('Upload diagnostic events missing');

        el('logout').click();
        await until(() => el('session').textContent==='Вход не выполнен' && !doc.querySelector('button').disabled);
        await fetch(${JSON.stringify(base + "/result")}, {method:'POST', body:JSON.stringify({ok:true,checks:[...restart.checks,'128 MiB Filelink upload survives full Thunderbird restart and resumes exact bytes from saved block','upload diagnostics record retry without filenames, account addresses, tokens or links','logout']})});
        return;
      }
      await until(() => el('session').textContent === 'Вход не выполнен');
      el('baseUrl').value = ${JSON.stringify(base)}; el('email').value = 'smoke@example.test'; el('password').value = 'fixture-password';
      if(!el('login-form').checkValidity()) throw new Error('Invalid login form: '+Array.from(el('login-form').querySelectorAll('input')).map(node=>node.id+':'+node.validationMessage+':'+node.value).join(',')); el('login-form').requestSubmit();
      await until(() => el('session').textContent.startsWith('Подключено:') && !doc.querySelector('button').disabled);
      if (el('password').value) throw new Error('Password retained in UI');
      if (!el('login-form').hidden || el('shared-session').hidden || el('logout').hidden) throw new Error('Signed-in user is asked to log in again');
      const management = document.createElement('iframe');
      management.src = browser.runtime.getURL('options.html'); document.body.append(management);
      await until(() => management.contentDocument?.getElementById('session')?.textContent.startsWith('Подключено:'));
      if (!management.contentDocument.getElementById('login-form').hidden) throw new Error('Filelink management asks for another login');
      el('theme').value='tayga-light'; el('preferences-form').requestSubmit();
      await until(() => el('msg').textContent==='Настройки сохранены.' && !doc.querySelector('button').disabled);
      const lightTheme = await browser.theme.getCurrent();
      if (lightTheme.properties.color_scheme !== 'light' || lightTheme.properties.content_color_scheme !== 'light') throw new Error('Light theme did not select light chrome and content');
      if (lightTheme.colors.sidebar !== '#f4f6f2' || lightTheme.colors.toolbar_text !== '#203b29') throw new Error('Tayga light palette not applied');
      const savedTheme = await browser.storage.local.get('theme');
      if (savedTheme.theme !== 'tayga-light') throw new Error('Light theme selection not saved');
      el('theme').value='tayga'; el('preferences-form').requestSubmit();
      await until(() => el('msg').textContent==='Настройки сохранены.' && !doc.querySelector('button').disabled);
      const darkTheme = await browser.theme.getCurrent();
      if (darkTheme.colors.toolbar !== '#243022' || darkTheme.properties?.color_scheme === 'light') throw new Error('Switching back to dark Tayga failed');
      el('theme').value='default'; el('preferences-form').requestSubmit();
      await until(() => el('msg').textContent==='Настройки сохранены.' && !doc.querySelector('button').disabled);
      el('vac-on').checked=true; el('vac-body').value='Native smoke test'; el('vacation-form').requestSubmit();
      await until(() => el('msg').textContent==='Автоответчик сохранён.');
      el('sieve-name').value='smoke'; el('sieve-body').value='keep;'; el('sieve-form').requestSubmit();
      await until(() => el('msg').textContent==='Скрипт сохранён.');
      if (!el('sieve-list').textContent.includes('smoke')) throw new Error('Script not rendered');
      // Exercise the REST upload path with a native Gecko File object.
      await browser.storage.local.set({expiresAt: 1});
      const file = new File(['native-file'], 'тест.txt');
      const upload = await background.client.upload(file);
      if (!upload.url.endsWith('/s/smoke')) throw new Error('Upload failed');
      const counts = await fetch(${JSON.stringify(base + "/test/auth-counts")}).then(res => res.json());
      if (counts.logins !== 1 || counts.refreshes !== 1) throw new Error('Filelink did not reuse the mail session: '+JSON.stringify(counts));
      await fetch(${JSON.stringify(base + "/test/pause-upload")}, {method:'POST'});
      const largeFile = new File([new Uint8Array(128 * 1024 * 1024).fill(84)], 'large.bin');
      background.uploadFile({id:'smoke-account'}, {id:77,name:largeFile.name,data:largeFile}).catch(() => {});
      await until(() => Array.from(doc.querySelectorAll('#uploads-list li')).some(row => row.textContent.includes('Загружается') && row.querySelector('progress')?.value === 8 * 1024 * 1024));
      await browser.storage.local.set({fileSmokeRestart:{size:largeFile.size,checks:['native cloudFile API','settings DOM','Tayga light and dark themes','password login','vacation','Sieve','native File upload renews shared session','Filelink management reuses login','lost chunk response recovers durable offset before restart']}});
      await fetch(${JSON.stringify(base + "/result")}, {method:'POST',body:JSON.stringify({restart:true})});
    } catch(error) { await fetch(${JSON.stringify(base + "/result")}, {method:'POST', body:JSON.stringify({ok:false,error:error.message})}); } })();
  `;
  const ewsFixture = process.env.TAYGA_EWS_FIXTURE;
  const ui = ewsFixture
    ? (await readFile(join(root, "tests/ews-smoke-ui.js"), "utf8"))
        .replace('"__FIXTURE__"', JSON.stringify(ewsFixture))
        .replace('"__RESULT__"', JSON.stringify(base + "/result"))
    : defaultUI;
  await writeFile(join(extension, "smoke-ui.js"), ui);
  const experiment = join(extension, "experiments/ews/api.js");
  await writeFile(
    experiment,
    (await readFile(experiment, "utf8")).replace(
      "switch (command) {",
      'if (command === "test") return taygaNativeTest(this, args); switch (command) {',
    ) +
      "\n" +
      (await readFile(join(root, "tests/ews-native-helpers.js"), "utf8")),
  );
  // Expose only to this temporary test extension's same-origin smoke page.
  await writeFile(
    join(extension, "background.js"),
    (await readFile(join(extension, "background.js"), "utf8")) +
      "\nglobalThis.client = client; globalThis.ews = ews; globalThis.uploadFile = uploadFile;\n",
  );
  const prefs = {
    "devtools.console.stdout.chrome": true,
    "javascript.options.showInConsole": true,
    "extensions.logging.enabled": true,
    "extensions.autoDisableScopes": 0,
    "extensions.enabledScopes": 15,
    "extensions.startupScanScopes": 15,
    "xpinstall.signatures.required": false,
    "mail.shell.checkDefaultClient": false,
    "mail.provider.enabled": false,
    "datareporting.policy.dataSubmissionEnabled": false,
    "toolkit.telemetry.enabled": false,
    "mail.tabs.restoreSession": false,
    "app.update.auto": false,
    "browser.startup.homepage_override.mstone": "ignore",
  };
  await writeFile(
    join(profile, "user.js"),
    Object.entries(prefs)
      .map(
        ([key, value]) =>
          `user_pref(${JSON.stringify(key)}, ${JSON.stringify(value)});`,
      )
      .join("\n"),
  );
  const binary =
    process.env.THUNDERBIRD_BINARY ||
    "/Applications/Thunderbird.app/Contents/MacOS/thunderbird";
  let output = "";
  for (let attempt = 0; attempt < 2; attempt++) {
    if (attempt)
      result = new Promise((resolve) => {
        resolveResult = resolve;
      });
    processHandle = spawn(
      binary,
      ["--headless", "--no-remote", "--profile", profile],
      { stdio: ["ignore", "pipe", "pipe"] },
    );
    processHandle.stdout.on("data", (chunk) => {
      output += chunk;
    });
    processHandle.stderr.on("data", (chunk) => {
      output += chunk;
    });
    processHandle.on("error", (error) =>
      resolveResult({ ok: false, error: error.message }),
    );
    const timer = setTimeout(
      () =>
        resolveResult({
          ok: false,
          error: "Thunderbird smoke timeout",
          output: output.slice(-6000),
        }),
      90000,
    );
    const report = await result;
    clearTimeout(timer);
    if (report.restart && attempt === 0) {
      const exited = new Promise((resolve) =>
        processHandle.once("exit", resolve),
      );
      processHandle.kill("SIGTERM");
      await exited;
      processHandle = null;
      continue;
    }
    console.log(JSON.stringify(report, null, 2));
    if (!report.ok) {
      console.error(output.slice(-6000));
      process.exitCode = 1;
    }
    break;
  }
} finally {
  if (processHandle?.pid && processHandle.exitCode === null) {
    const exited = new Promise((resolve) =>
      processHandle.once("exit", resolve),
    );
    processHandle.kill("SIGTERM");
    const killTimer = setTimeout(() => processHandle.kill("SIGKILL"), 4000);
    await exited;
    clearTimeout(killTimer);
  }
  server.closeAllConnections();
  await new Promise((resolve) => server.close(resolve));
  await rm(temp, { recursive: true, force: true });
}
