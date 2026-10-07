document.getElementById("open-options").addEventListener("click", () => {
  browser.runtime.openOptionsPage();
});
browser.storage.local.get(["email", "baseUrl"]).then((s) => {
  document.getElementById("status").textContent = s.email
    ? `${s.email} @ ${s.baseUrl || ""}`
    : "Not signed in";
});
