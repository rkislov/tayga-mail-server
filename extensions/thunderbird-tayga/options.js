async function load() {
  const s = await browser.storage.local.get(["baseUrl", "email", "theme"]);
  if (s.baseUrl) document.getElementById("baseUrl").value = s.baseUrl;
  if (s.email) document.getElementById("email").value = s.email;
  if (s.theme) document.getElementById("theme").value = s.theme;
  try {
    const vac = await browser.runtime.sendMessage({ type: "tayga.vacation.get" });
    document.getElementById("vac-on").checked = !!vac.enabled;
    document.getElementById("vac-subj").value = vac.subject || "";
    document.getElementById("vac-body").value = vac.body || "";
  } catch (_) {}
}

document.getElementById("btn-login").addEventListener("click", async () => {
  const msg = document.getElementById("msg");
  msg.textContent = "Signing in…";
  try {
    await browser.storage.local.set({ theme: document.getElementById("theme").value });
    const res = await browser.runtime.sendMessage({
      type: "tayga.login",
      baseUrl: document.getElementById("baseUrl").value.trim(),
      email: document.getElementById("email").value.trim(),
      password: document.getElementById("password").value,
    });
    if (res.mfa) {
      msg.textContent = "MFA required — complete login in the web UI, then paste an access token via storage (advanced).";
      return;
    }
    msg.textContent = "OK. Autoconfig: " + res.autoconfig;
    await browser.runtime.sendMessage({ type: "tayga.applyTheme" });
  } catch (e) {
    msg.textContent = e.message || String(e);
  }
});

document.getElementById("btn-theme").addEventListener("click", async () => {
  await browser.storage.local.set({ theme: document.getElementById("theme").value });
  await browser.runtime.sendMessage({ type: "tayga.applyTheme" });
  document.getElementById("msg").textContent = "Theme applied";
});

document.getElementById("btn-vac").addEventListener("click", async () => {
  const msg = document.getElementById("msg");
  try {
    await browser.runtime.sendMessage({
      type: "tayga.vacation.put",
      body: {
        enabled: document.getElementById("vac-on").checked,
        subject: document.getElementById("vac-subj").value,
        body: document.getElementById("vac-body").value,
      },
    });
    msg.textContent = "Vacation saved";
  } catch (e) {
    msg.textContent = e.message || String(e);
  }
});

document.getElementById("btn-sieve").addEventListener("click", async () => {
  const msg = document.getElementById("msg");
  try {
    await browser.runtime.sendMessage({
      type: "tayga.sieve.put",
      name: document.getElementById("sieve-name").value.trim() || "default",
      script: document.getElementById("sieve-body").value,
    });
    msg.textContent = "Sieve saved";
  } catch (e) {
    msg.textContent = e.message || String(e);
  }
});

document.getElementById("btn-sieve-load").addEventListener("click", async () => {
  try {
    const data = await browser.runtime.sendMessage({ type: "tayga.sieve.list" });
    document.getElementById("sieve-list").textContent = JSON.stringify(data.scripts || data, null, 2);
  } catch (e) {
    document.getElementById("sieve-list").textContent = e.message || String(e);
  }
});

load();
