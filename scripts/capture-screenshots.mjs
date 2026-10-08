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
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, "..");
const outDir = path.join(root, "docs/screenshots");
const base = process.env.TMS_URL || "http://127.0.0.1:18080";
const chrome =
  process.env.CHROME_PATH ||
  `${process.env.HOME}/Library/Caches/ms-playwright/chromium-1208/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing`;

async function shot(page, name) {
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
  });
  await page.reload({ waitUntil: "networkidle" });
  await page.waitForTimeout(700);
  await shot(page, "ui-login-tayga.png");

  await login(page);
  await theme(page, "tayga");

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

  await browser.close();
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
