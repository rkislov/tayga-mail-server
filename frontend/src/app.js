const state = {
  tokens: null,
  email: "",
  challenge: "",
  methods: [],
};

const $ = (id) => document.getElementById(id);


const THEMES = {
  taiga: {
    label: "Тайга",
    image: "/assets/taiga-forest.jpg",
    image2x: "/assets/taiga-forest@2x.jpg",
  },
  cosmos: {
    label: "Космос",
    image: "/assets/cosmic-abstract.jpg",
    image2x: "/assets/cosmic-abstract@2x.jpg",
  },
  city: {
    label: "Город",
    image: "/assets/city.jpg",
    image2x: "/assets/city@2x.jpg",
  },
  kalyazin: {
    label: "Колязин",
    image: "/assets/kalyazin.jpg",
    image2x: "/assets/kalyazin@2x.jpg",
  },
  temple: {
    label: "Храм на Нерли",
    image: "/assets/temple-nerl.jpg",
    image2x: "/assets/temple-nerl@2x.jpg",
  },
  moscow: {
    label: "Москва-Сити",
    image: "/assets/moscow-city.jpg",
    image2x: "/assets/moscow-city@2x.jpg",
  },
};

function applyTheme(name) {
  const theme = THEMES[name] || THEMES.taiga;
  const key = name in THEMES ? name : "taiga";
  document.documentElement.dataset.theme = key;
  document.documentElement.style.setProperty(
    "--tayga-bg-image",
    `image-set(url('${theme.image}') 1x, url('${theme.image2x}') 2x)`
  );
  const sel = document.getElementById("theme-select");
  if (sel && sel.value !== key) sel.value = key;
  const preview = document.getElementById("theme-preview");
  if (preview) preview.style.backgroundImage = `url('${theme.image}')`;
  localStorage.setItem("tayga.theme", key);
}

(function initTheme() {
  const saved = localStorage.getItem("tayga.theme") || "taiga";
  applyTheme(saved);
  document.getElementById("theme-select")?.addEventListener("change", (e) => {
    applyTheme(e.target.value);
  });
})();

const APPS = [
  "mail", "calendar", "contacts", "files",
  "profile", "security", "appearance", "filters", "language",
  "monitor", "tenants", "tls", "server",
];

const mailState = { mailboxID: "", messageID: "", mailboxes: [] };
const calState = { calendarID: "" };
const contactState = { bookID: "" };
const filesState = { path: "" };

function show(id) {
  const account = id === "view-account";
  $("auth-layout")?.classList.toggle("hidden", account);
  $("view-account")?.classList.toggle("hidden", !account);
  if (!account) {
    $("view-login")?.classList.toggle("hidden", id !== "view-login");
    $("view-mfa")?.classList.toggle("hidden", id !== "view-mfa");
  }
}

const APP_I18N = {
  mail: "nav_mail",
  calendar: "nav_calendar",
  contacts: "nav_contacts",
  files: "nav_files",
  profile: "nav_profile",
  security: "nav_security",
  appearance: "nav_appearance",
  filters: "nav_filters",
  language: "nav_language",
  monitor: "nav_monitor",
  tenants: "nav_tenants",
  tls: "nav_tls",
  server: "nav_server",
};

function setNavOpen(open) {
  const shell = $("view-account");
  const btn = $("btn-nav-toggle");
  const nav = $("app-nav");
  const backdrop = $("nav-backdrop");
  if (!shell) return;
  shell.classList.toggle("nav-open", open);
  if (btn) btn.setAttribute("aria-expanded", open ? "true" : "false");
  if (nav) nav.setAttribute("aria-hidden", open ? "false" : "true");
  if (backdrop) backdrop.hidden = !open;
  document.body.style.overflow = open ? "hidden" : "";
}

function showApp(name) {
  const app = APPS.includes(name) ? name : "mail";
  APPS.forEach((a) => $(`app-${a}`)?.classList.toggle("hidden", a !== app));
  document.querySelectorAll(".nav-btn").forEach((btn) => {
    btn.classList.toggle("is-active", btn.dataset.app === app);
  });
  const title = $("app-topbar-title");
  if (title) {
    const key = APP_I18N[app] || "nav_mail";
    title.setAttribute("data-i18n", key);
    title.textContent = t(key);
  }
  $("compose-backdrop")?.classList.add("hidden");
  setNavOpen(false);
  if (app === "mail") refreshMail();
  if (app === "calendar") refreshCalendar();
  if (app === "contacts") refreshContacts();
  if (app === "files") refreshFiles();
  if (app === "filters") refreshSieve();
  if (app === "monitor") refreshMonitor();
  if (app === "tenants") { refreshTenant(); refreshAdminUsers(); }
  if (app === "tls") refreshTLS();
  if (app === "server") refreshSettings();
  if (app === "security") refreshPasskeys();
  if (app === "profile") loadMe();
}

function setMsg(el, text, kind) {
  el.textContent = text || "";
  el.className = "msg" + (kind ? " " + kind : "");
}

async function api(path, opts = {}) {
  const headers = Object.assign({ "Content-Type": "application/json" }, opts.headers || {});
  if (state.tokens?.access_token) {
    headers.Authorization = "Bearer " + state.tokens.access_token;
  }
  const res = await fetch(path, Object.assign({}, opts, { headers }));
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = { error: text }; }
  if (!res.ok) {
    const err = new Error((data && data.error) || res.statusText || "request failed");
    err.status = res.status;
    err.data = data;
    throw err;
  }
  return data;
}

function b64urlToBuf(v) {
  const s = v.replace(/-/g, "+").replace(/_/g, "/");
  const pad = "=".repeat((4 - (s.length % 4)) % 4);
  const bin = atob(s + pad);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out.buffer;
}

function bufToB64url(buf) {
  const bytes = new Uint8Array(buf);
  let s = "";
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

function reviveCreation(publicKey) {
  const o = structuredClone(publicKey);
  o.challenge = b64urlToBuf(o.challenge);
  o.user.id = b64urlToBuf(o.user.id);
  if (o.excludeCredentials) {
    o.excludeCredentials = o.excludeCredentials.map((c) => Object.assign({}, c, { id: b64urlToBuf(c.id) }));
  }
  return o;
}

function reviveRequest(publicKey) {
  const o = structuredClone(publicKey);
  o.challenge = b64urlToBuf(o.challenge);
  if (o.allowCredentials) {
    o.allowCredentials = o.allowCredentials.map((c) => Object.assign({}, c, { id: b64urlToBuf(c.id) }));
  }
  return o;
}

function credentialToJSON(cred) {
  const r = cred.response;
  const out = {
    id: cred.id,
    rawId: bufToB64url(cred.rawId),
    type: cred.type,
    response: {},
    clientExtensionResults: cred.getClientExtensionResults?.() || {},
  };
  if (r.attestationObject) {
    out.response = {
      clientDataJSON: bufToB64url(r.clientDataJSON),
      attestationObject: bufToB64url(r.attestationObject),
      transports: r.getTransports?.() || [],
    };
  } else {
    out.response = {
      clientDataJSON: bufToB64url(r.clientDataJSON),
      authenticatorData: bufToB64url(r.authenticatorData),
      signature: bufToB64url(r.signature),
      userHandle: r.userHandle ? bufToB64url(r.userHandle) : null,
    };
  }
  return out;
}

function fmtBytes(n) {
  if (n == null) return "—";
  if (n < 1024) return n + " B";
  if (n < 1024*1024) return (n/1024).toFixed(1) + " KiB";
  if (n < 1024*1024*1024) return (n/(1024*1024)).toFixed(1) + " MiB";
  return (n/(1024*1024*1024)).toFixed(2) + " GiB";
}

async function loadMe() {
  try {
    const me = await api("/api/v1/me");
    state.email = me.email || state.email;
    $("acct-email").textContent = me.email;
    localStorage.setItem("tayga.email", state.email);
    const q = me.quota_bytes > 0
      ? `${fmtBytes(me.used_bytes)} / ${fmtBytes(me.quota_bytes)} used`
      : `${fmtBytes(me.used_bytes)} used · unlimited quota`;
    $("acct-quota").textContent = q;
    $("acct-role").textContent = me.is_admin ? "admin" : "live";
    $("nav-admin")?.classList.toggle("hidden", !me.is_admin);
  } catch (err) {
    $("acct-meta").textContent = err.message;
  }
}

function fmtNum(n) {
  if (n == null) return "—";
  return Number(n).toLocaleString();
}

async function refreshMonitor() {
  try {
    const d = await api("/api/v1/admin/status");
    const t = d.tenant || {};
    const s = d.server || {};
    const g = d.go || {};
    const rows = [
      ["Tenant", t.tenant_name || "—"],
      ["Uptime", (d.uptime_sec || 0) + "s"],
      ["Users (tenant)", fmtNum(t.users) + " / " + fmtNum(t.users_enabled) + " enabled"],
      ["Domains", fmtNum(t.domains)],
      ["Messages (tenant)", fmtNum(t.messages)],
      ["Bytes (tenant)", fmtBytes(t.bytes_stored || 0)],
      ["Users (server)", fmtNum(s.users)],
      ["Messages (server)", fmtNum(s.messages)],
      ["Bytes (server)", fmtBytes(s.bytes_stored || 0)],
      ["Outbound queued", fmtNum(s.outbound_queued)],
      ["Goroutines", fmtNum(g.goroutines)],
    ];
    $("monitor-grid").innerHTML = rows.map(([k, v]) =>
      `<div><span class="meta">${escapeHtml(k)}</span><br/><strong>${escapeHtml(String(v))}</strong></div>`
    ).join("");
    await refreshOutbound();
    await refreshQuarantine();
  } catch (err) {
    setMsg($("monitor-msg"), err.message, "err");
  }
}

async function refreshQuarantine() {
  const list = $("quarantine-list");
  const preview = $("q-preview");
  if (!list) return;
  if (preview) {
    preview.classList.add("hidden");
    preview.textContent = "";
  }
  try {
    const params = new URLSearchParams({ limit: "40" });
    const folder = $("q-folder")?.value || "";
    const kind = $("q-kind")?.value || "";
    const q = ($("q-search")?.value || "").trim();
    if (folder) params.set("folder", folder);
    if (kind) params.set("kind", kind);
    if (q) params.set("q", q);
    const d = await api("/api/v1/admin/quarantine?" + params.toString());
    const items = d.items || [];
    if (!items.length) {
      list.innerHTML = "<li class=\"meta\">No quarantined messages.</li>";
      return;
    }
    list.innerHTML = items.map((it) => {
      const subj = it.subject || "(no subject)";
      const from = it.from || "—";
      const chips = [];
      if (it.kind) chips.push(it.kind);
      if (it.spam_score) chips.push("score " + it.spam_score);
      if (it.virus_name) chips.push(it.virus_name);
      if (it.spam_status) chips.push(it.spam_status);
      if (it.virus_status && !it.virus_name) chips.push(it.virus_status);
      const chipHtml = chips.length
        ? `<br/><span class="meta">${chips.map((c) => escapeHtml(String(c))).join(" · ")}</span>`
        : "";
      return `<li>
        <strong>${escapeHtml(subj)}</strong>
        <span class="meta"> · ${escapeHtml(it.folder)} · ${escapeHtml(it.email)}</span><br/>
        <span class="meta">from ${escapeHtml(from)} · ${escapeHtml(it.date || "")} · ${fmtBytes(it.size || 0)}</span>
        ${chipHtml}
        <div class="actions" style="margin-top:0.35rem">
          <button type="button" class="btn-secondary" data-q-prev="${escapeHtml(it.id)}">Preview</button>
          <button type="button" class="btn-secondary" data-q-rel="${escapeHtml(it.id)}">Release</button>
          <button type="button" class="btn-secondary" data-q-del="${escapeHtml(it.id)}">Delete</button>
        </div>
      </li>`;
    }).join("");
    list.querySelectorAll("[data-q-prev]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          const it = await api("/api/v1/admin/quarantine/" + btn.getAttribute("data-q-prev"));
          const lines = [
            "From: " + (it.from || ""),
            "Subject: " + (it.subject || ""),
            "Kind: " + (it.kind || ""),
            it.spam_score ? "Spam-Score: " + it.spam_score : "",
            it.spam_status ? "Spam-Status: " + it.spam_status : "",
            it.virus_name ? "Virus: " + it.virus_name : "",
            it.auth_results ? "Auth: " + it.auth_results : "",
            "",
            it.preview || "(empty body)",
          ].filter((x, i, a) => x !== "" || (i > 0 && a[i - 1] !== ""));
          if (preview) {
            preview.textContent = lines.join("\n");
            preview.classList.remove("hidden");
          }
        } catch (err) { setMsg($("monitor-msg"), err.message, "err"); }
      });
    });
    list.querySelectorAll("[data-q-rel]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          await api("/api/v1/admin/quarantine/" + btn.getAttribute("data-q-rel") + "/release", { method: "POST" });
          setMsg($("monitor-msg"), "Released to INBOX.", "ok");
          refreshMonitor();
        } catch (err) { setMsg($("monitor-msg"), err.message, "err"); }
      });
    });
    list.querySelectorAll("[data-q-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!confirm("Permanently delete this quarantined message?")) return;
        try {
          await api("/api/v1/admin/quarantine/" + btn.getAttribute("data-q-del"), { method: "DELETE" });
          setMsg($("monitor-msg"), "Deleted.", "ok");
          refreshMonitor();
        } catch (err) { setMsg($("monitor-msg"), err.message, "err"); }
      });
    });
  } catch (err) {
    list.innerHTML = `<li class="meta">${escapeHtml(err.message)}</li>`;
  }
}

$("btn-q-refresh")?.addEventListener("click", () => refreshQuarantine());
$("q-search")?.addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    refreshQuarantine();
  }
});

async function refreshOutbound() {
  const list = $("outbound-list");
  if (!list) return;
  try {
    const d = await api("/api/v1/admin/outbound?limit=30");
    const items = d.items || [];
    if (!items.length) {
      list.innerHTML = "<li class=\"meta\">Queue empty.</li>";
      return;
    }
    list.innerHTML = items.map((it) => {
      const err = it.last_error ? `<br/><span class="meta">${escapeHtml(it.last_error)}</span>` : "";
      return `<li>
        <strong>${escapeHtml(it.envelope_to)}</strong>
        <span class="meta"> from ${escapeHtml(it.envelope_from)} · ${it.attempts}/${it.max_attempts} · next ${escapeHtml(it.next_attempt || "")}</span>
        ${err}
        <div class="actions" style="margin-top:0.35rem">
          <button type="button" class="btn-secondary" data-out-retry="${escapeHtml(it.id)}">Retry now</button>
          <button type="button" class="btn-secondary" data-out-del="${escapeHtml(it.id)}">Drop</button>
        </div>
      </li>`;
    }).join("");
    list.querySelectorAll("[data-out-retry]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          await api("/api/v1/admin/outbound/" + btn.getAttribute("data-out-retry") + "/retry", { method: "POST" });
          setMsg($("monitor-msg"), "Queued for immediate retry.", "ok");
          refreshMonitor();
        } catch (err) { setMsg($("monitor-msg"), err.message, "err"); }
      });
    });
    list.querySelectorAll("[data-out-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!confirm("Drop this queued message without DSN?")) return;
        try {
          await api("/api/v1/admin/outbound/" + btn.getAttribute("data-out-del"), { method: "DELETE" });
          setMsg($("monitor-msg"), "Dropped.", "ok");
          refreshMonitor();
        } catch (err) { setMsg($("monitor-msg"), err.message, "err"); }
      });
    });
  } catch (err) {
    list.innerHTML = `<li class="meta">${escapeHtml(err.message)}</li>`;
  }
}

$("btn-monitor-refresh").addEventListener("click", () => refreshMonitor());

async function downloadBackup(includeMail) {
  setMsg($("monitor-msg"), "Preparing backup…");
  try {
    const q = includeMail ? "?include_mail=1" : "";
    const headers = {};
    if (state.tokens?.access_token) headers.Authorization = "Bearer " + state.tokens.access_token;
    const res = await fetch("/api/v1/admin/backup" + q, { headers });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || ("HTTP " + res.status));
    }
    const blob = await res.blob();
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "tayga-backup.tar.gz";
    a.click();
    URL.revokeObjectURL(a.href);
    setMsg($("monitor-msg"), "Backup downloaded.", "ok");
  } catch (err) {
    setMsg($("monitor-msg"), err.message, "err");
  }
}
$("btn-backup").addEventListener("click", () => downloadBackup(false));
$("btn-backup-mail").addEventListener("click", () => downloadBackup(true));

$("btn-restore").addEventListener("click", async () => {
  const input = $("restore-file");
  if (!input.files || !input.files[0]) {
    setMsg($("monitor-msg"), "choose a backup file", "err");
    return;
  }
  if (!confirm("Restore will create missing tenants/users and may update existing accounts. Continue?")) return;
  setMsg($("monitor-msg"), "Restoring…");
  try {
    const fd = new FormData();
    fd.append("file", input.files[0]);
    fd.append("include_mail", "1");
    const headers = {};
    if (state.tokens?.access_token) headers.Authorization = "Bearer " + state.tokens.access_token;
    const res = await fetch("/api/v1/admin/backup/restore", { method: "POST", headers, body: fd });
    const text = await res.text();
    let data;
    try { data = JSON.parse(text); } catch { data = { error: text }; }
    if (!res.ok) throw new Error(data.error || ("HTTP " + res.status));
    setMsg($("monitor-msg"),
      "Restored: +" + (data.users_created || 0) + " users, " +
      (data.users_skipped || 0) + " skipped, " +
      (data.scripts_restored || 0) + " scripts, " +
      (data.mail_files || 0) + " mail files.", "ok");
    refreshMonitor();
    refreshTenant();
    refreshAdminUsers();
  } catch (err) {
    setMsg($("monitor-msg"), err.message, "err");
  }
});

async function refreshTenant() {
  try {
    const data = await api("/api/v1/admin/tenant");
    const t = data.tenant || {};
    $("tenant-meta").textContent = (t.name || "") + " · " + (t.user_count || 0) + " users · " + (t.id || "");
    const list = $("admin-domain-list");
    list.innerHTML = (data.domains || []).map((d) => `
      <li>
        <span>
          <strong>${escapeHtml(d.name)}</strong><br/>
          <span class="meta">${d.user_count || 0} users · ${escapeHtml(d.id)}</span>
        </span>
        <button type="button" class="btn-secondary" data-del-domain="${escapeHtml(d.name)}" ${d.user_count > 0 ? "disabled" : ""}>Remove</button>
      </li>`).join("") || "<li><span class=\"meta\">No domains yet.</span></li>";
    list.querySelectorAll("[data-del-domain]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!confirm("Remove domain " + btn.dataset.delDomain + "?")) return;
        try {
          await api("/api/v1/admin/domains/" + encodeURIComponent(btn.dataset.delDomain), { method: "DELETE" });
          setMsg($("admin-msg"), "Domain removed.", "ok");
          refreshTenant();
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
  } catch (err) {
    $("tenant-meta").textContent = err.message;
  }
}

$("form-create-domain").addEventListener("submit", async (e) => {
  e.preventDefault();
  setMsg($("admin-msg"), "Adding domain…");
  try {
    await api("/api/v1/admin/domains", {
      method: "POST",
      body: JSON.stringify({ name: $("nd-name").value.trim() }),
    });
    setMsg($("admin-msg"), "Domain added.", "ok");
    $("form-create-domain").reset();
    refreshTenant();
  } catch (err) {
    setMsg($("admin-msg"), err.message, "err");
  }
});

async function refreshAdminUsers() {
  try {
    const data = await api("/api/v1/admin/users");
    const list = $("admin-user-list");
    list.innerHTML = (data.users || []).map((u) => `
      <li>
        <span>
          <strong>${escapeHtml(u.email)}</strong>${u.enabled ? "" : " · disabled"}<br/>
          <span class="meta">${fmtBytes(u.used_bytes)} / ${u.quota_bytes > 0 ? fmtBytes(u.quota_bytes) : "∞"} · ${escapeHtml(u.id)}</span>
        </span>
        <span class="actions">
          <button type="button" class="btn-secondary" data-quota="${escapeHtml(u.id)}">Quota</button>
          <button type="button" class="btn-secondary" data-toggle="${escapeHtml(u.id)}" data-enabled="${u.enabled ? "1" : "0"}">${u.enabled ? "Disable" : "Enable"}</button>
          <button type="button" class="btn-secondary" data-pass="${escapeHtml(u.id)}">Reset pw</button>
        </span>
      </li>`).join("");
    list.querySelectorAll("[data-quota]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const raw = prompt("Quota bytes (0 = unlimited)", "0");
        if (raw == null) return;
        const quota_bytes = Number(raw);
        if (Number.isNaN(quota_bytes) || quota_bytes < 0) {
          setMsg($("admin-msg"), "invalid quota", "err");
          return;
        }
        try {
          await api("/api/v1/admin/users/" + encodeURIComponent(btn.dataset.quota) + "/quota", {
            method: "PUT",
            body: JSON.stringify({ quota_bytes }),
          });
          setMsg($("admin-msg"), "Quota updated.", "ok");
          refreshAdminUsers();
          loadMe();
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
    list.querySelectorAll("[data-toggle]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const enabled = btn.dataset.enabled !== "1";
        try {
          await api("/api/v1/admin/users/" + encodeURIComponent(btn.dataset.toggle), {
            method: "PATCH",
            body: JSON.stringify({ enabled }),
          });
          setMsg($("admin-msg"), enabled ? "User enabled." : "User disabled.", "ok");
          refreshAdminUsers();
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
    list.querySelectorAll("[data-pass]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const password = prompt("New password (min 8 characters)");
        if (password == null) return;
        if (password.length < 8) {
          setMsg($("admin-msg"), "password too short", "err");
          return;
        }
        try {
          await api("/api/v1/admin/users/" + encodeURIComponent(btn.dataset.pass) + "/password", {
            method: "PUT",
            body: JSON.stringify({ password }),
          });
          setMsg($("admin-msg"), "Password reset.", "ok");
        } catch (err) {
          setMsg($("admin-msg"), err.message, "err");
        }
      });
    });
  } catch (err) {
    setMsg($("admin-msg"), err.message, "err");
  }
}

function renderTLSStatus(info) {
  if (!info || !info.configured) {
    $("tls-status").textContent = "No certificate loaded. Upload PEM or generate a self-signed cert.";
    return;
  }
  const sans = [].concat(info.dns_names || [], info.ip_sans || []).join(", ") || "—";
  $("tls-status").innerHTML =
    "<strong>" + escapeHtml(info.subject || "") + "</strong><br/>" +
    "<span class=\"meta\">Valid " + escapeHtml(info.not_before || "") + " → " + escapeHtml(info.not_after || "") +
    " · " + (info.expires_in_hours != null ? info.expires_in_hours + "h left" : "") + "</span><br/>" +
    "<span class=\"meta\">SANs: " + escapeHtml(sans) + "</span><br/>" +
    "<span class=\"meta\">SHA-256: " + escapeHtml(info.fingerprint_sha256 || "") + "</span>";
}

async function refreshTLS() {
  try {
    const info = await api("/api/v1/admin/tls");
    renderTLSStatus(info);
    if (!$("tls-hosts").value && info) {
      const hosts = [].concat(info.dns_names || [], info.ip_sans || []);
      if (hosts.length) $("tls-hosts").value = hosts.join(", ");
    }
  } catch (err) {
    $("tls-status").textContent = err.message;
    setMsg($("tls-msg"), err.message, "err");
  }
}

$("btn-tls-refresh").addEventListener("click", () => refreshTLS());
$("btn-tls-upload").addEventListener("click", async () => {
  setMsg($("tls-msg"), "Installing…");
  try {
    const info = await api("/api/v1/admin/tls", {
      method: "PUT",
      body: JSON.stringify({
        certificate: $("tls-cert").value,
        private_key: $("tls-key").value,
      }),
    });
    renderTLSStatus(info);
    $("tls-key").value = "";
    setMsg($("tls-msg"), "Certificate installed (hot-reloaded).", "ok");
  } catch (err) {
    setMsg($("tls-msg"), err.message, "err");
  }
});
$("btn-tls-generate").addEventListener("click", async () => {
  const hosts = $("tls-hosts").value.split(",").map((s) => s.trim()).filter(Boolean);
  setMsg($("tls-msg"), "Generating…");
  try {
    const info = await api("/api/v1/admin/tls", {
      method: "POST",
      body: JSON.stringify({ hosts, days: 365 }),
    });
    renderTLSStatus(info);
    setMsg($("tls-msg"), "Self-signed certificate generated and activated.", "ok");
  } catch (err) {
    setMsg($("tls-msg"), err.message, "err");
  }
});

const settingsState = { all: null };

function fillSettingsSectionSelect(sections) {
  const sel = $("settings-section");
  const cur = sel.value;
  sel.innerHTML = "";
  (sections || []).forEach((name) => {
    const opt = document.createElement("option");
    opt.value = name;
    opt.textContent = name;
    sel.appendChild(opt);
  });
  if (cur && sections && sections.includes(cur)) sel.value = cur;
  else if (sections && sections.length) sel.value = sections.includes("spam") ? "spam" : sections[0];
}

function showSettingsSection(name) {
  if (!settingsState.all || !settingsState.all.settings) return;
  const val = settingsState.all.settings[name] || {};
  $("settings-json").value = JSON.stringify(val, null, 2);
}

async function refreshSettings() {
  try {
    const data = await api("/api/v1/admin/settings");
    settingsState.all = data;
    fillSettingsSectionSelect(data.sections || []);
    showSettingsSection($("settings-section").value);
    $("settings-restart").classList.toggle("hidden", !data.restart_required);
    setMsg($("settings-msg"), "");
  } catch (err) {
    setMsg($("settings-msg"), err.message, "err");
  }
}

$("settings-section").addEventListener("change", () => {
  showSettingsSection($("settings-section").value);
});
$("btn-settings-refresh").addEventListener("click", () => refreshSettings());
$("btn-settings-save").addEventListener("click", async () => {
  const section = $("settings-section").value;
  let parsed;
  try {
    parsed = JSON.parse($("settings-json").value);
  } catch (err) {
    setMsg($("settings-msg"), "Invalid JSON: " + err.message, "err");
    return;
  }
  setMsg($("settings-msg"), "Saving…");
  try {
    const out = await api("/api/v1/admin/settings/" + encodeURIComponent(section), {
      method: "PUT",
      body: JSON.stringify(parsed),
    });
    $("settings-restart").classList.toggle("hidden", !out.restart_required);
    setMsg($("settings-msg"), out.restart_required
      ? "Saved. Restart tayga-mail to apply."
      : "Saved.", "ok");
    await refreshSettings();
  } catch (err) {
    setMsg($("settings-msg"), err.message, "err");
  }
});

$("form-create-user").addEventListener("submit", async (e) => {
  e.preventDefault();
  setMsg($("admin-msg"), "Creating…");
  try {
    const body = {
      email: $("nu-email").value.trim(),
      display_name: $("nu-name").value.trim(),
      password: $("nu-pass").value,
      quota_bytes: Number($("nu-quota").value) || 0,
    };
    const created = await api("/api/v1/admin/users", {
      method: "POST",
      body: JSON.stringify(body),
    });
    setMsg($("admin-msg"), "Created " + created.email + " (" + created.id + ")", "ok");
    $("form-create-user").reset();
    $("nu-quota").value = "0";
    refreshAdminUsers();
  } catch (err) {
    setMsg($("admin-msg"), err.message, "err");
  }
});

async function refreshSieve() {
  try {
    const data = await api("/api/v1/sieve/scripts");
    const list = $("sieve-list");
    list.innerHTML = (data.scripts || []).map((sc) => `
      <li>
        <span>
          <strong>${escapeHtml(sc.name)}</strong>${sc.active ? " · active" : ""}<br/>
          <span class="meta">${escapeHtml((sc.script || "").split("\n")[0].slice(0, 72))}</span>
        </span>
        <span class="actions">
          <button type="button" class="btn-secondary" data-sieve-edit="${escapeHtml(sc.name)}">Edit</button>
          <button type="button" class="btn-secondary" data-sieve-act="${escapeHtml(sc.name)}">Activate</button>
          <button type="button" class="btn-secondary" data-sieve-del="${escapeHtml(sc.name)}">Delete</button>
        </span>
      </li>`).join("") || "<li><span class=\"meta\">No scripts yet.</span></li>";
    list.querySelectorAll("[data-sieve-edit]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const sc = (data.scripts || []).find((x) => x.name === btn.dataset.sieveEdit);
        if (!sc) return;
        $("sieve-name").value = sc.name;
        $("sieve-body").value = sc.script;
      });
    });
    list.querySelectorAll("[data-sieve-act]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          await api("/api/v1/sieve/scripts/" + encodeURIComponent(btn.dataset.sieveAct) + "/activate", { method: "POST", body: "{}" });
          setMsg($("sieve-msg"), "Activated.", "ok");
          refreshSieve();
        } catch (err) {
          setMsg($("sieve-msg"), err.message, "err");
        }
      });
    });
    list.querySelectorAll("[data-sieve-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!confirm("Delete script " + btn.dataset.sieveDel + "?")) return;
        try {
          await api("/api/v1/sieve/scripts/" + encodeURIComponent(btn.dataset.sieveDel), { method: "DELETE" });
          setMsg($("sieve-msg"), "Deleted.", "ok");
          refreshSieve();
        } catch (err) {
          setMsg($("sieve-msg"), err.message, "err");
        }
      });
    });
  } catch (err) {
    setMsg($("sieve-msg"), err.message, "err");
  }
}

$("btn-sieve-refresh").addEventListener("click", () => refreshSieve());
$("btn-sieve-save").addEventListener("click", async () => {
  const name = $("sieve-name").value.trim();
  const script = $("sieve-body").value;
  if (!name || !script.trim()) {
    setMsg($("sieve-msg"), "name and script required", "err");
    return;
  }
  setMsg($("sieve-msg"), "Saving…");
  try {
    await api("/api/v1/sieve/scripts/" + encodeURIComponent(name), {
      method: "PUT",
      body: JSON.stringify({ script, active: true }),
    });
    setMsg($("sieve-msg"), "Saved and activated.", "ok");
    refreshSieve();
  } catch (err) {
    setMsg($("sieve-msg"), err.message, "err");
  }
});

function enterAccount(tokens, email) {
  state.tokens = tokens;
  state.email = email || state.email;
  localStorage.setItem("tayga.tokens", JSON.stringify(tokens));
  localStorage.setItem("tayga.email", state.email);
  $("acct-email").textContent = state.email;
  show("view-account");
  loadMe();
  showApp("mail");
  applyLang(lang);
}

$("form-login").addEventListener("submit", async (e) => {
  e.preventDefault();
  setMsg($("login-msg"), "Signing in…");
  try {
    const data = await api("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({
        email: $("email").value.trim(),
        password: $("password").value,
      }),
    });
    state.email = data.email || $("email").value.trim();
    if (data.mfa_required) {
      state.challenge = data.challenge;
      state.methods = data.methods || ["totp"];
      renderMfaMethods();
      show("view-mfa");
      setMsg($("login-msg"), "");
      return;
    }
    enterAccount(data.tokens, state.email);
    setMsg($("login-msg"), "");
  } catch (err) {
    setMsg($("login-msg"), err.message, "err");
  }
});

function renderMfaMethods() {
  const wrap = $("mfa-methods");
  wrap.innerHTML = state.methods.map((m) => `<span class="chip">${m}</span>`).join(" ");
  const hasTotp = state.methods.includes("totp");
  const hasWA = state.methods.includes("webauthn");
  $("form-totp").classList.toggle("hidden", !hasTotp);
  $("mfa-webauthn-actions").classList.toggle("hidden", !hasWA);
}

$("form-totp").addEventListener("submit", async (e) => {
  e.preventDefault();
  setMsg($("mfa-msg"), "Verifying…");
  try {
    const tokens = await api("/api/v1/auth/mfa/verify", {
      method: "POST",
      body: JSON.stringify({ challenge: state.challenge, code: $("totp").value.trim() }),
    });
    enterAccount(tokens, state.email);
  } catch (err) {
    setMsg($("mfa-msg"), err.message, "err");
  }
});

$("btn-webauthn-login").addEventListener("click", async () => {
  setMsg($("mfa-msg"), "Waiting for authenticator…");
  try {
    const begin = await api("/api/v1/auth/webauthn/login/begin", {
      method: "POST",
      body: JSON.stringify({ challenge: state.challenge }),
    });
    const publicKey = reviveRequest(begin.options.publicKey);
    const cred = await navigator.credentials.get({ publicKey });
    const tokens = await api("/api/v1/auth/webauthn/login/finish", {
      method: "POST",
      body: JSON.stringify({
        challenge: state.challenge,
        session_id: begin.session_id,
        credential: credentialToJSON(cred),
      }),
    });
    enterAccount(tokens, state.email);
  } catch (err) {
    setMsg($("mfa-msg"), err.message, "err");
  }
});

$("btn-back-login").addEventListener("click", () => show("view-login"));

$("btn-logout").addEventListener("click", () => {
  state.tokens = null;
  localStorage.removeItem("tayga.tokens");
  setNavOpen(false);
  show("view-login");
});
$("btn-refresh-me").addEventListener("click", loadMe);

$("btn-totp-setup").addEventListener("click", async () => {
  setMsg($("totp-msg"), "Creating secret…");
  try {
    const data = await api("/api/v1/auth/mfa/setup", { method: "POST", body: "{}" });
    $("totp-setup-box").classList.remove("hidden");
    $("totp-uri").textContent = data.otpauth_uri;
    $("totp-backup").textContent = "Backup codes: " + (data.backup_codes || []).join(", ");
    setMsg($("totp-msg"), "Scan the URI, then confirm a code.", "ok");
  } catch (err) {
    setMsg($("totp-msg"), err.message, "err");
  }
});

$("btn-totp-confirm").addEventListener("click", async () => {
  try {
    await api("/api/v1/auth/mfa/confirm", {
      method: "POST",
      body: JSON.stringify({ code: $("totp-confirm").value.trim() }),
    });
    setMsg($("totp-msg"), "TOTP enabled.", "ok");
  } catch (err) {
    setMsg($("totp-msg"), err.message, "err");
  }
});

async function refreshPasskeys() {
  try {
    const data = await api("/api/v1/auth/webauthn/credentials");
    const list = $("cred-list");
    const creds = data.credentials || [];
    if (!creds.length) {
      list.innerHTML = "<li><span class='meta'>No passkeys yet</span></li>";
      return;
    }
    list.innerHTML = creds.map((c) => `
      <li>
        <span><strong>${escapeHtml(c.name || "Passkey")}</strong><br/><span class="meta">${escapeHtml(c.id)}</span></span>
        <button type="button" class="btn-secondary" data-del="${escapeHtml(c.id)}">Remove</button>
      </li>`).join("");
    list.querySelectorAll("[data-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          await api("/api/v1/auth/webauthn/credentials/" + encodeURIComponent(btn.dataset.del), { method: "DELETE" });
          refreshPasskeys();
        } catch (err) {
          setMsg($("passkey-msg"), err.message, "err");
        }
      });
    });
  } catch (err) {
    setMsg($("passkey-msg"), err.message, "err");
  }
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&":"&amp;", "<":"&lt;", ">":"&gt;", '"':"&quot;", "'":"&#39;" }[c]));
}

$("btn-passkey-refresh").addEventListener("click", refreshPasskeys);

$("btn-passkey-add").addEventListener("click", async () => {
  setMsg($("passkey-msg"), "Creating passkey…");
  try {
    const begin = await api("/api/v1/auth/webauthn/register/begin", { method: "POST", body: "{}" });
    const publicKey = reviveCreation(begin.options.publicKey);
    const cred = await navigator.credentials.create({ publicKey });
    const name = prompt("Name this passkey", "This device") || "Passkey";
    await api("/api/v1/auth/webauthn/register/finish", {
      method: "POST",
      body: JSON.stringify({
        session_id: begin.session_id,
        name,
        credential: credentialToJSON(cred),
      }),
    });
    setMsg($("passkey-msg"), "Passkey registered.", "ok");
    refreshPasskeys();
  } catch (err) {
    setMsg($("passkey-msg"), err.message, "err");
  }
});

/* —— Mail —— */
async function refreshMail() {
  try {
    const data = await api("/api/v1/mail/mailboxes");
    mailState.mailboxes = data.mailboxes || [];
    const list = $("mail-folder-list");
    if (!list) return;
    if (!mailState.mailboxID && mailState.mailboxes.length) {
      const inbox = mailState.mailboxes.find((m) => m.name === "INBOX") || mailState.mailboxes[0];
      mailState.mailboxID = inbox.id;
    }
    list.innerHTML = mailState.mailboxes.map((mb) => `
      <li>
        <button type="button" class="folder-btn${mb.id === mailState.mailboxID ? " is-active" : ""}" data-mb="${escapeHtml(mb.id)}">
          <span>${escapeHtml(mb.name)}</span>
          <span class="meta">${mb.unread ? mb.unread + " · " : ""}${mb.messages || 0}</span>
        </button>
      </li>`).join("") || `<li class="meta">${t("empty_mailbox")}</li>`;
    list.querySelectorAll("[data-mb]").forEach((btn) => {
      btn.addEventListener("click", () => {
        mailState.mailboxID = btn.dataset.mb;
        mailState.messageID = "";
        refreshMailMessages();
        list.querySelectorAll(".folder-btn").forEach((b) => b.classList.toggle("is-active", b.dataset.mb === mailState.mailboxID));
      });
    });
    await refreshMailMessages();
  } catch (err) {
    if ($("mail-folder-list")) $("mail-folder-list").innerHTML = `<li class="meta">${escapeHtml(err.message)}</li>`;
  }
}

async function refreshMailMessages() {
  const list = $("mail-msg-list");
  if (!list || !mailState.mailboxID) return;
  const mb = mailState.mailboxes.find((m) => m.id === mailState.mailboxID);
  if ($("mail-folder-title")) $("mail-folder-title").textContent = mb?.name || "INBOX";
  try {
    const data = await api("/api/v1/mail/mailboxes/" + encodeURIComponent(mailState.mailboxID) + "/messages?limit=80");
    const msgs = data.messages || [];
    if (!msgs.length) {
      list.innerHTML = `<li class="meta">${t("empty_mailbox")}</li>`;
      showMailReader(null);
      return;
    }
    list.innerHTML = msgs.map((m) => `
      <li>
        <button type="button" class="msg-item${!m.seen ? " unread" : ""}${m.id === mailState.messageID ? " is-active" : ""}" data-msg="${escapeHtml(m.id)}">
          <strong>${escapeHtml(m.subject || "(no subject)")}</strong>
          <span class="meta">${escapeHtml(m.from || "")}</span>
          <span class="meta">${escapeHtml((m.internal_date || "").slice(0, 16).replace("T", " "))}</span>
        </button>
      </li>`).join("");
    list.querySelectorAll("[data-msg]").forEach((btn) => {
      btn.addEventListener("click", () => openMailMessage(btn.dataset.msg));
    });
    if (mailState.messageID) openMailMessage(mailState.messageID);
    else showMailReader(null);
  } catch (err) {
    list.innerHTML = `<li class="meta">${escapeHtml(err.message)}</li>`;
  }
}

function showMailReader(msg) {
  const empty = $("mail-empty");
  const reader = $("mail-reader");
  if (!empty || !reader) return;
  if (!msg) {
    empty.classList.remove("hidden");
    reader.classList.add("hidden");
    return;
  }
  empty.classList.add("hidden");
  reader.classList.remove("hidden");
  $("mail-subject").textContent = msg.subject || "(no subject)";
  $("mail-meta").textContent = [msg.from, msg.to, msg.date || msg.internal_date].filter(Boolean).join(" · ");
  const body = $("mail-body");
  if (msg.html) {
    body.innerHTML = msg.html;
  } else {
    body.textContent = msg.text || "";
  }
}

async function openMailMessage(id) {
  mailState.messageID = id;
  try {
    const msg = await api("/api/v1/mail/messages/" + encodeURIComponent(id));
    showMailReader(msg);
    document.querySelectorAll(".msg-item").forEach((el) => {
      el.classList.toggle("is-active", el.dataset.msg === id);
      if (el.dataset.msg === id) el.classList.remove("unread");
    });
  } catch (err) {
    showMailReader(null);
    if ($("mail-empty")) $("mail-empty").textContent = err.message;
  }
}

$("btn-mail-refresh")?.addEventListener("click", () => refreshMail());
$("btn-compose")?.addEventListener("click", () => {
  $("compose-backdrop")?.classList.remove("hidden");
  $("compose-to").value = "";
  $("compose-subject").value = "";
  $("compose-body").value = "";
  setMsg($("compose-msg"), "");
});
$("btn-compose-close")?.addEventListener("click", () => $("compose-backdrop")?.classList.add("hidden"));
$("compose-backdrop")?.addEventListener("click", (e) => {
  if (e.target === $("compose-backdrop")) $("compose-backdrop").classList.add("hidden");
});
$("btn-compose-send")?.addEventListener("click", async () => {
  const to = ($("compose-to").value || "").split(/[,;]/).map((s) => s.trim()).filter(Boolean);
  setMsg($("compose-msg"), "…");
  try {
    await api("/api/v1/mail/send", {
      method: "POST",
      body: JSON.stringify({
        to,
        subject: $("compose-subject").value,
        text: $("compose-body").value,
      }),
    });
    setMsg($("compose-msg"), "OK", "ok");
    $("compose-backdrop")?.classList.add("hidden");
    refreshMail();
  } catch (err) {
    setMsg($("compose-msg"), err.message, "err");
  }
});
$("btn-mail-reply")?.addEventListener("click", () => {
  const meta = $("mail-meta")?.textContent || "";
  const from = (meta.split(" · ")[0] || "").trim();
  $("compose-backdrop")?.classList.remove("hidden");
  $("compose-to").value = from;
  $("compose-subject").value = "Re: " + ($("mail-subject")?.textContent || "");
  $("compose-body").value = "\n\n---\n" + ($("mail-body")?.textContent || "").slice(0, 2000);
});
$("btn-mail-delete")?.addEventListener("click", async () => {
  if (!mailState.messageID || !confirm("Delete?")) return;
  try {
    await api("/api/v1/mail/messages/" + encodeURIComponent(mailState.messageID), { method: "DELETE" });
    mailState.messageID = "";
    refreshMailMessages();
  } catch (err) {
    alert(err.message);
  }
});

/* —— Calendar —— */
async function refreshCalendar() {
  try {
    const data = await api("/api/v1/calendar/calendars");
    const cals = data.calendars || [];
    if (!calState.calendarID && cals.length) calState.calendarID = cals[0].id;
    if (!calState.calendarID) {
      $("cal-event-list").innerHTML = "<li class=\"meta\">No calendar</li>";
      return;
    }
    const ev = await api("/api/v1/calendar/calendars/" + encodeURIComponent(calState.calendarID) + "/events");
    const events = ev.events || [];
    $("cal-event-list").innerHTML = events.map((e) => `
      <li>
        <span>
          <strong>${escapeHtml(e.summary || "")}</strong><br/>
          <span class="meta">${escapeHtml(e.start || "")} → ${escapeHtml(e.end || "")}</span>
        </span>
        <button type="button" class="btn-secondary" data-cal-del="${escapeHtml(e.id)}">${t("delete")}</button>
      </li>`).join("") || "<li class=\"meta\">—</li>";
    $("cal-event-list").querySelectorAll("[data-cal-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          await api("/api/v1/calendar/events/" + encodeURIComponent(btn.dataset.calDel), { method: "DELETE" });
          refreshCalendar();
        } catch (err) { setMsg($("cal-msg"), err.message, "err"); }
      });
    });
  } catch (err) {
    setMsg($("cal-msg"), err.message, "err");
  }
}
$("btn-cal-refresh")?.addEventListener("click", () => refreshCalendar());
$("form-cal-event")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  if (!calState.calendarID) {
    try {
      const data = await api("/api/v1/calendar/calendars");
      calState.calendarID = (data.calendars || [])[0]?.id || "";
    } catch {}
  }
  if (!calState.calendarID) {
    setMsg($("cal-msg"), "no calendar", "err");
    return;
  }
  setMsg($("cal-msg"), "…");
  try {
    await api("/api/v1/calendar/calendars/" + encodeURIComponent(calState.calendarID) + "/events", {
      method: "POST",
      body: JSON.stringify({
        summary: $("cal-summary").value,
        start: $("cal-start").value,
        end: $("cal-end").value,
      }),
    });
    setMsg($("cal-msg"), "OK", "ok");
    $("form-cal-event").reset();
    refreshCalendar();
  } catch (err) {
    setMsg($("cal-msg"), err.message, "err");
  }
});

/* —— Contacts —— */
async function refreshContacts() {
  try {
    const data = await api("/api/v1/contacts/books");
    const books = data.books || [];
    if (!contactState.bookID && books.length) contactState.bookID = books[0].id;
    if (!contactState.bookID) {
      $("contact-list").innerHTML = "<li class=\"meta\">No address book</li>";
      return;
    }
    const cards = await api("/api/v1/contacts/books/" + encodeURIComponent(contactState.bookID) + "/cards");
    const list = cards.cards || [];
    $("contact-list").innerHTML = list.map((c) => `
      <li>
        <span>
          <strong>${escapeHtml(c.fn || "")}</strong><br/>
          <span class="meta">${escapeHtml(c.email || "")} ${escapeHtml(c.tel || "")}</span>
        </span>
        <button type="button" class="btn-secondary" data-card-del="${escapeHtml(c.id)}">${t("delete")}</button>
      </li>`).join("") || "<li class=\"meta\">—</li>";
    $("contact-list").querySelectorAll("[data-card-del]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        try {
          await api("/api/v1/contacts/cards/" + encodeURIComponent(btn.dataset.cardDel), { method: "DELETE" });
          refreshContacts();
        } catch (err) { setMsg($("contact-msg"), err.message, "err"); }
      });
    });
  } catch (err) {
    setMsg($("contact-msg"), err.message, "err");
  }
}
$("btn-contacts-refresh")?.addEventListener("click", () => refreshContacts());
$("form-contact")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  if (!contactState.bookID) {
    try {
      const data = await api("/api/v1/contacts/books");
      contactState.bookID = (data.books || [])[0]?.id || "";
    } catch {}
  }
  if (!contactState.bookID) {
    setMsg($("contact-msg"), "no book", "err");
    return;
  }
  setMsg($("contact-msg"), "…");
  try {
    await api("/api/v1/contacts/books/" + encodeURIComponent(contactState.bookID) + "/cards", {
      method: "POST",
      body: JSON.stringify({
        fn: $("contact-fn").value,
        email: $("contact-email").value,
        tel: $("contact-tel").value,
      }),
    });
    setMsg($("contact-msg"), "OK", "ok");
    $("form-contact").reset();
    refreshContacts();
  } catch (err) {
    setMsg($("contact-msg"), err.message, "err");
  }
});

/* —— Files —— */
function joinPath(base, name) {
  if (!base) return name;
  return base.replace(/\/+$/, "") + "/" + name;
}

async function refreshFiles() {
  const list = $("files-list");
  if (!list) return;
  if ($("files-path")) $("files-path").textContent = "/" + (filesState.path || "");
  try {
    const q = filesState.path ? "?path=" + encodeURIComponent(filesState.path) : "";
    const data = await api("/api/v1/files" + q);
    const entries = data.entries || [];
    let html = "";
    if (filesState.path) {
      html += `<li><button type="button" class="btn-secondary" id="files-up">..</button></li>`;
    }
    html += entries.map((e) => `
      <li>
        <span>
          <strong>${escapeHtml(e.name)}</strong>
          <span class="meta"> · ${e.is_dir ? "dir" : fmtBytes(e.size || 0)}</span>
        </span>
        <span class="actions">
          ${e.is_dir
            ? `<button type="button" class="btn-secondary" data-dir="${escapeHtml(e.name)}">Open</button>`
            : `<button type="button" class="btn-secondary" data-dl="${escapeHtml(e.name)}">Download</button>`}
          <button type="button" class="btn-secondary" data-rm="${escapeHtml(e.name)}">${t("delete")}</button>
        </span>
      </li>`).join("") || (!filesState.path ? "<li class=\"meta\">—</li>" : "");
    list.innerHTML = html;
    $("files-up")?.addEventListener("click", () => {
      const parts = filesState.path.split("/").filter(Boolean);
      parts.pop();
      filesState.path = parts.join("/");
      refreshFiles();
    });
    list.querySelectorAll("[data-dir]").forEach((btn) => {
      btn.addEventListener("click", () => {
        filesState.path = joinPath(filesState.path, btn.dataset.dir);
        refreshFiles();
      });
    });
    list.querySelectorAll("[data-dl]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const path = joinPath(filesState.path, btn.dataset.dl);
        const headers = {};
        if (state.tokens?.access_token) headers.Authorization = "Bearer " + state.tokens.access_token;
        const res = await fetch("/api/v1/files/content?path=" + encodeURIComponent(path), { headers });
        if (!res.ok) { setMsg($("files-msg"), "download failed", "err"); return; }
        const blob = await res.blob();
        const a = document.createElement("a");
        a.href = URL.createObjectURL(blob);
        a.download = btn.dataset.dl;
        a.click();
        URL.revokeObjectURL(a.href);
      });
    });
    list.querySelectorAll("[data-rm]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        if (!confirm("Delete " + btn.dataset.rm + "?")) return;
        try {
          await api("/api/v1/files?path=" + encodeURIComponent(joinPath(filesState.path, btn.dataset.rm)), {
            method: "DELETE",
          });
          refreshFiles();
        } catch (err) { setMsg($("files-msg"), err.message, "err"); }
      });
    });
  } catch (err) {
    setMsg($("files-msg"), err.message, "err");
  }
}
$("btn-files-refresh")?.addEventListener("click", () => refreshFiles());
$("form-mkdir")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  const name = ($("mkdir-name").value || "").trim();
  if (!name) return;
  try {
    await api("/api/v1/files/mkdir", {
      method: "POST",
      body: JSON.stringify({ path: joinPath(filesState.path, name) }),
    });
    $("mkdir-name").value = "";
    refreshFiles();
  } catch (err) { setMsg($("files-msg"), err.message, "err"); }
});
$("files-upload")?.addEventListener("change", async (e) => {
  const file = e.target.files?.[0];
  if (!file) return;
  try {
    const path = joinPath(filesState.path, file.name);
    const headers = { "Content-Type": "application/octet-stream" };
    if (state.tokens?.access_token) headers.Authorization = "Bearer " + state.tokens.access_token;
    const res = await fetch("/api/v1/files/content?path=" + encodeURIComponent(path), {
      method: "PUT", headers, body: file,
    });
    if (!res.ok) throw new Error(await res.text());
    setMsg($("files-msg"), "uploaded", "ok");
    refreshFiles();
  } catch (err) {
    setMsg($("files-msg"), err.message, "err");
  }
  e.target.value = "";
});

/* —— Nav / i18n —— */
document.querySelectorAll(".nav-btn").forEach((btn) => {
  btn.addEventListener("click", () => showApp(btn.dataset.app));
});
$("btn-nav-toggle")?.addEventListener("click", () => {
  setNavOpen(!$("view-account")?.classList.contains("nav-open"));
});
$("nav-backdrop")?.addEventListener("click", () => setNavOpen(false));
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && $("view-account")?.classList.contains("nav-open")) {
    setNavOpen(false);
  }
});
$("lang-select")?.addEventListener("change", (e) => applyLang(e.target.value));
applyLang(lang);

// Restore session
try {
  const tok = JSON.parse(localStorage.getItem("tayga.tokens") || "null");
  const email = localStorage.getItem("tayga.email") || "";
  if (tok?.access_token) enterAccount(tok, email);
} catch {}
