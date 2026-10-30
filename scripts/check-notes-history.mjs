#!/usr/bin/env node
// Run only against a disposable seeded instance. Creates and removes test notes.
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 for (const viewport of [{width:1440,height:900},{width:390,height:844}]) {
  const page=await browser.newPage({viewport}); const errors=[]; page.on('pageerror',e=>errors.push(e.message));
  page.setDefaultTimeout(12000);
  await page.goto(process.env.TMS_URL||'http://127.0.0.1:18080');
  await page.evaluate(()=>{localStorage.clear();localStorage.setItem('tayga.lang','ru');}); await page.reload();
  await page.fill('#email','admin@example.com'); await page.fill('#password','changeme'); await page.click('#form-login button[type=submit]');
  await page.waitForFunction(()=>state.me?.id); await page.evaluate(()=>showApp('notes')); await page.waitForFunction(()=>notesState.folderID);
  await page.click('#section-create .create-fab'); await page.waitForFunction(()=>notesState.noteID);
  await page.fill('#note-title','Before restore'); await page.locator('#note-body').fill('Keep this checklist'); await page.click('#btn-note-checklist'); await page.locator('[data-task-text]').first().fill('Preserved'); await page.locator('#note-body input[type=checkbox]').first().check();
  await page.click('#btn-note-save'); await page.waitForFunction(()=>!notesState.dirty);
  await page.fill('#note-title','After edit'); await page.click('#btn-note-save'); await page.waitForFunction(()=>!notesState.dirty);
  await page.click('#btn-note-history');
  const history=page.locator('dialog').filter({has:page.locator('h3', {hasText:'История версий'})});
  await history.locator('details').first().waitFor(); await history.locator('summary').first().click();
  assert.match(await history.locator('details').first().textContent(),/Before restore/);
  await history.getByRole('button',{name:'Восстановить',exact:true}).first().click();
  await page.waitForFunction(()=>document.querySelector('#note-title').value==='Before restore');
  assert.equal(await page.locator('#note-body input[type=checkbox]').first().isChecked(),true);
  await page.click('#btn-note-history'); await history.locator('details').first().waitFor(); assert.match(await history.locator('summary').first().textContent(),/After edit/);
  await history.getByRole('button',{name:'Закрыть',exact:true}).click();
  await page.click('#btn-note-delete'); await page.click('#dialog-ok'); await page.waitForFunction(()=>notesState.noteID==='');
  assert.deepEqual(errors,[]); await page.close();
 }
 console.log('Notes history: desktop/mobile preview, checklist restore and preservation of the replaced version — OK');
} finally { await browser.close(); }
