/* Tayga Mail Thunderbird companion — large attachments → share links */

const DEFAULTS = {
  baseUrl: "https://mail.example.com",
  token: "",
  largeAttachBytes: 10 * 1024 * 1024,
  theme: "taiga",
};

async function settings() {
  const stored = await browser.storage.local.get(DEFAULTS);
  return Object.assign({}, DEFAULTS, stored);
}

async function api(path, opts = {}) {
  const cfg = await settings();
  const headers = Object.assign({ "Content-Type": "application/json" }, opts.headers || {});
  if (cfg.token) headers.Authorization = "Bearer " + cfg.token;
  const res = await fetch(cfg.baseUrl.replace(/\/$/, "") + path, Object.assign({}, opts, { headers }));
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = { error: text }; }
  if (!res.ok) throw new Error((data && data.error) || res.statusText);
  return data;
}

async function applyTheme() {
  const cfg = await settings();
  try {
    await browser.theme.update({
      colors: {
        frame: cfg.theme === "cosmos" ? "#0b1020" : "#1a2418",
        toolbar: cfg.theme === "cosmos" ? "#121a2e" : "#243022",
        toolbar_text: "#f5f5f0",
        tab_background_text: "#f5f5f0",
        tab_line: "#e8b84a",
        bookmark_text: "#f5f5f0",
        toolbar_field: "#0f1610",
        toolbar_field_text: "#f5f5f0",
      },
    });
  } catch (e) {
    console.warn("Tayga theme:", e);
  }
}

browser.runtime.onInstalled.addListener(() => { applyTheme(); });
browser.storage.onChanged.addListener(() => { applyTheme(); });

browser.compose.onBeforeSend.addListener(async (tab, details) => {
  const cfg = await settings();
  if (!cfg.token || !cfg.baseUrl) return {};
  // Attachments API varies by TB version; best-effort body append via compose.update if share created from options.
  return {};
});

browser.runtime.onMessage.addListener(async (msg) => {
  if (msg?.type === "tayga.login") {
    const res = await fetch(msg.baseUrl.replace(/\/$/, "") + "/api/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: msg.email, password: msg.password }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "login failed");
    if (data.mfa_required) return { mfa: true, challenge: data.challenge };
    const tokens = data.tokens || data;
    await browser.storage.local.set({
      baseUrl: msg.baseUrl.replace(/\/$/, ""),
      token: tokens.access_token,
      email: msg.email,
    });
    await applyTheme();
    return { ok: true, autoconfig: msg.baseUrl.replace(/\/$/, "") + "/.well-known/autoconfig/mail/config-v1.1.xml" };
  }
  if (msg?.type === "tayga.shareFile") {
    const cfg = await settings();
    const path = "tb-out/" + Date.now() + "-" + (msg.name || "file").replace(/[^\w.\-]+/g, "_");
    const put = await fetch(cfg.baseUrl.replace(/\/$/, "") + "/api/v1/files/content?path=" + encodeURIComponent(path), {
      method: "PUT",
      headers: {
        Authorization: "Bearer " + cfg.token,
        "Content-Type": "application/octet-stream",
      },
      body: msg.blob,
    });
    if (!put.ok) throw new Error(await put.text());
    const sh = await api("/api/v1/files/shares", {
      method: "POST",
      body: JSON.stringify({ path, ttl_hours: 168 }),
    });
    return { url: sh.url };
  }
  if (msg?.type === "tayga.sieve.list") return api("/api/v1/sieve/scripts");
  if (msg?.type === "tayga.sieve.put") {
    return api("/api/v1/sieve/scripts/" + encodeURIComponent(msg.name), {
      method: "PUT",
      body: JSON.stringify({ script: msg.script, active: true }),
    });
  }
  if (msg?.type === "tayga.vacation.get") return api("/api/v1/mail/vacation");
  if (msg?.type === "tayga.vacation.put") {
    return api("/api/v1/mail/vacation", { method: "PUT", body: JSON.stringify(msg.body) });
  }
  if (msg?.type === "tayga.applyTheme") {
    await applyTheme();
    return { ok: true };
  }
});
