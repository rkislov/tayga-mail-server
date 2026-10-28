"use strict";
document
  .getElementById("open-options")
  .addEventListener("click", () => browser.runtime.openOptionsPage());
document.getElementById("open-web").addEventListener("click", async () => {
  try {
    await browser.runtime.sendMessage({ type: "tayga.openWeb" });
    window.close();
  } catch (error) {
    document.getElementById("status").textContent = error.message;
  }
});
browser.runtime
  .sendMessage({ type: "tayga.status" })
  .then((cfg) => {
    document.getElementById("status").textContent = cfg.signedIn
      ? `${cfg.email} · ${cfg.baseUrl}`
      : "Вход не выполнен";
    document.getElementById("open-web").disabled = !cfg.baseUrl;
  })
  .catch((error) => {
    document.getElementById("status").textContent = error.message;
  });
