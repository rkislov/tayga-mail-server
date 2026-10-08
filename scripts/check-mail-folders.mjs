#!/usr/bin/env node
// Run only against a disposable seeded server: creates, renames and deletes a test folder.
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage({viewport:{width:1440,height:900}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(process.env.TMS_URL||'http://127.0.0.1:18080');await page.evaluate(()=>{localStorage.clear();localStorage.setItem('tayga.colorMode','light');localStorage.setItem('tayga.lang','ru');});await page.reload();
 await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForSelector('#view-account:not(.hidden)');
 await page.waitForFunction(()=>document.querySelector('#mail-folder-list button')?.textContent.includes('Входящие'));
 await page.evaluate(async()=>{await refreshMail();for(const m of mailState.mailboxes.filter(m=>['Проверка папок','Рабочая папка'].includes(m.name)))await api('/api/v1/mail/mailboxes/'+m.id,{method:'DELETE'});await refreshMail();});
 await page.click('#btn-mail-folders');assert.equal(await page.locator('dialog[open] [data-delete]').count(),0);assert.equal(await page.locator('dialog[open] [data-rename]').count(),0);
 await page.click('[data-create]');await page.fill('#dialog-input','Проверка папок');await page.click('#dialog-ok');await page.waitForSelector('dialog[open] [data-rename]');
 const id=await page.locator('[data-rename]').getAttribute('data-rename');const previousIndex=await page.evaluate(id=>mailState.mailboxes.findIndex(m=>m.id===id),id);const step=await page.locator(`[data-down="${id}"]`).isEnabled()?1:-1;await page.click(`[data-${step===1?"down":"up"}="${id}"]`);await page.waitForFunction(({id,i,step})=>mailState.mailboxes[i+step]?.id===id,{id,i:previousIndex,step});
 assert.equal(await page.evaluate(()=>mailState.mailboxes[0].name),'INBOX');
 await page.click(`[data-rename="${id}"]`);await page.fill('#dialog-input','Рабочая папка');await page.click('#dialog-ok');await page.waitForFunction(()=>mailState.mailboxes.some(m=>m.name==='Рабочая папка'));
 await page.screenshot({path:'docs/screenshots/ui-mail-folders-tayga-light.png'});
 await page.setViewportSize({width:390,height:844});assert.equal(await page.locator('dialog[open]').evaluate(el=>el.scrollWidth>el.clientWidth),false);await page.screenshot({path:'docs/screenshots/ui-mail-folders-tayga-light-mobile.png'});
 await page.click(`[data-delete="${id}"]`);await page.click('#dialog-ok');await page.waitForFunction(id=>!mailState.mailboxes.some(m=>m.id===id),id);
 await page.click('dialog[open] [data-close]');await page.evaluate(()=>{applyLang('en');return refreshMail();});assert.match(await page.locator('#mail-folder-list button').first().textContent(),/Inbox/);
 await page.reload();await page.waitForSelector('#view-account:not(.hidden)');await page.waitForFunction(()=>mailState.mailboxes.length===6);assert.equal(await page.evaluate(()=>mailState.mailboxes[0].name),'INBOX');
 assert.deepEqual(errors,[]);console.log('Mail folders: localization, system protection, create/rename/delete, order persistence, mobile — OK');
} finally {await browser.close();}
