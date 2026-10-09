#!/usr/bin/env node
/**
 * Capture UI screenshots into docs/screenshots/.
 * Requires: tayga on :18080, playwright-core + Chromium for Testing.
 *
 *   npm i -g playwright-core   # or NODE_PATH to a local install
 *   node scripts/capture-screenshots.mjs
 */
import { chromium } from "playwright-core";
import path from "node:path";
import fs from "node:fs";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, "..");
const outDir = path.join(root, "docs/screenshots");
const base = process.env.TMS_URL || "http://127.0.0.1:18080";
const chrome = process.env.CHROME_PATH || [
  `${process.env.HOME}/Library/Caches/ms-playwright/chromium-1208/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing`,
  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
].find((file) => fs.existsSync(file));

async function shot(page, name) {
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(600);
  await page.screenshot({ path: path.join(outDir, name), fullPage: false });
  console.log("wrote", name);
}

async function login(page) {
  await page.goto(base + "/", { waitUntil: "domcontentloaded" });
  await page.evaluate(() => {
    localStorage.clear();
    localStorage.setItem("tayga.theme", "tayga");
    localStorage.setItem("tayga.lang", "ru");
    localStorage.setItem("tayga.colorMode", "dark");
  });
  await page.reload({ waitUntil: "networkidle" });
  await page.waitForSelector("#email", { timeout: 15000 });
  await page.fill("#email", "admin@example.com");
  await page.fill("#password", "changeme");
  await page.click("#form-login button[type=submit]");
  await page.waitForSelector("#app-mail", { timeout: 20000 });
  await page.waitForTimeout(1000);
}

async function goApp(page, app) {
  await page.evaluate((a) => {
    const btn = document.querySelector(`button.nav-btn[data-app="${a}"]`);
    if (btn) btn.click();
    else if (typeof showApp === "function") showApp(a);
  }, app);
  await page.waitForTimeout(900);
}

async function theme(page, name) {
  await page.evaluate((t) => {
    localStorage.setItem("tayga.theme", t);
    if (typeof applyTheme === "function") applyTheme(t);
  }, name);
  await page.waitForTimeout(500);
}

async function main() {
  fs.mkdirSync(outDir, { recursive: true });
  const browser = await chromium.launch({
    executablePath: chrome,
    headless: true,
    args: ["--disable-dev-shm-usage", "--no-sandbox"],
  });
  const page = await (
    await browser.newContext({ viewport: { width: 1440, height: 900 } })
  ).newPage();

  await page.goto(base + "/", { waitUntil: "networkidle" });
  await page.evaluate(() => {
    localStorage.clear();
    localStorage.setItem("tayga.theme", "tayga");
    localStorage.setItem("tayga.lang", "ru");
    localStorage.setItem("tayga.colorMode", "dark");
  });
  await page.reload({ waitUntil: "networkidle" });
  await page.waitForTimeout(700);
  await shot(page, "ui-login-tayga.png");

  await login(page);
  await theme(page, "tayga");
  if (process.env.TMS_SHOT_ONLY === "dkim") {
    await page.evaluate(() => applyColorMode("light"));
    await goApp(page, "server");
    await page.getByRole("button", {name:"DKIM: ключи и DNS",exact:true}).click();
    await page.locator("dialog[open] input").first().fill("example.com");
    await page.getByRole("button", {name:"Сгенерировать ключ",exact:true}).click();
    await page.getByRole("button", {name:"Скачать закрытый ключ (.pem)",exact:true}).waitFor();
    await shot(page,"ui-dkim-generator-tayga-light.png");
    await browser.close();
    return;
  }

  for (const [app, file] of [
    ["mail", "ui-mail-tayga.png"],
    ["calendar", "ui-calendar-tayga.png"],
    ["contacts", "ui-contacts-tayga.png"],
    ["notes", "ui-notes-tayga.png"],
    ["files", "ui-files-tayga.png"],
    ["chat", "ui-chat-tayga.png"],
    ["tls", "ui-ca-tayga.png"],
    ["xmpp", "ui-xmpp-tayga.png"],
    ["server", "ui-server-tayga.png"],
    ["cos", "ui-cos-tayga.png"],
    ["monitor", "ui-admin-tayga.png"],
  ]) {
    await goApp(page, app);
    await shot(page, file);
  }

  await theme(page, "cosmos");
  await goApp(page, "monitor");
  await shot(page, "ui-admin-cosmos.png");

  await theme(page, "moscow");
  await goApp(page, "monitor");
  await shot(page, "ui-admin-moscow.png");

  await theme(page, "street");
  await goApp(page, "chat");
  await shot(page, "ui-chat-street.png");

  await theme(page, "teriberka");
  await goApp(page, "mail");
  await shot(page, "ui-mail-teriberka.png");

  await theme(page, "tayga");
  await page.evaluate(() => applyColorMode("light"));
  for (const app of ["mail", "calendar", "contacts", "notes", "files", "chat", "tls", "xmpp", "server", "cos", "monitor", "appearance"]) {
    await goApp(page, app);
    const name = app === "tls" ? "ca" : app === "monitor" ? "admin" : app;
    await shot(page, `ui-${name}-tayga-light.png`);
  }
  await goApp(page, "server");
  await page.evaluate(() => openAdminConfig("smtp"));
  await shot(page, "ui-settings-smtp-tayga-light.png");
  await page.evaluate(() => closeAdminConfig());
  await page.evaluate(() => applyColorMode("dark"));
  await page.evaluate(() => openAdminConfig("smtp"));
  await shot(page, "ui-settings-smtp-tayga.png");
  await page.evaluate(() => closeAdminConfig());
  await page.evaluate(() => applyColorMode("light"));
  await goApp(page, "calendar");
  await page.evaluate(() => openCalDay(calDayKey(new Date())));
  await shot(page, "ui-calendar-day-tayga-light.png");
  await page.evaluate(() => {calState.view = "month"; renderCalMonth();});
  await page.evaluate(() => openCalInviteInbox());
  await shot(page, "ui-calendar-invites-tayga-light.png");
  await page.evaluate(() => closeCalInviteInbox());
  await page.evaluate(() => showCalCompose(true));
  await shot(page, "ui-event-editor-tayga-light.png");
  await page.evaluate(() => showCalCompose(false));
  await goApp(page, "contacts");
  await page.evaluate(() => showContactCompose(true));
  await shot(page, "ui-contact-editor-tayga-light.png");
  await page.keyboard.press("Escape");
  await goApp(page, "server");
  await page.getByRole("button", {name:"DKIM: ключи и DNS",exact:true}).click();
  await page.locator("dialog[open] input").first().fill("example.com");
  await page.getByRole("button", {name:"Сгенерировать ключ",exact:true}).click();
  await page.getByRole("button", {name:"Скачать закрытый ключ (.pem)",exact:true}).waitFor();
  await shot(page,"ui-dkim-generator-tayga-light.png");
  await page.keyboard.press("Escape");
  await goApp(page, "mail");
  await page.evaluate(() => openCompose());
  await shot(page, "ui-compose-tayga-light.png");
  await page.evaluate(() => document.getElementById("compose-backdrop").classList.add("hidden"));
  await goApp(page, "mail");
  await page.setViewportSize({ width: 390, height: 844 });
  await shot(page, "ui-mail-tayga-light-mobile.png");
  await page.evaluate(() => applyColorMode("dark"));
  await shot(page, "ui-mail-tayga-mobile.png");
  await page.evaluate(() => setNavOpen(true));
  await shot(page, "ui-navigation-tayga-mobile.png");
  await page.evaluate(() => setNavOpen(false));
  await page.setViewportSize({ width: 1440, height: 900 });
  await goApp(page, "appearance");
  await shot(page, "ui-appearance-tayga.png");

  await page.evaluate(() => { localStorage.clear(); localStorage.setItem("tayga.lang", "ru"); localStorage.setItem("tayga.colorMode", "light"); });
  await page.reload({ waitUntil: "networkidle" });
  await shot(page, "ui-login-tayga-light.png");
  await browser.close();
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
