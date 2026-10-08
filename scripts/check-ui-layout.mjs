#!/usr/bin/env node
// Visual/interaction regression check against a disposable seeded instance.
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage({viewport:{width:1440,height:900}});page.setDefaultTimeout(10000);const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(process.env.TMS_URL||'http://127.0.0.1:18080');await page.evaluate(()=>{localStorage.clear();localStorage.setItem('tayga.lang','ru');localStorage.setItem('tayga.colorMode','light');});await page.reload();
 await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForSelector('#view-account:not(.hidden)');
 await page.waitForFunction(()=>state.me?.id);
 const overflows=[];
 for (const width of [1920,1440,1024,768,390]) {
  await page.setViewportSize({width,height:900});
  for (const app of await page.evaluate(()=>APPS)) {
   await page.evaluate(app=>showApp(app),app);await page.waitForTimeout(120);
   const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1);
   if(overflow)overflows.push({app,width,elements:await page.evaluate(()=>[...document.querySelectorAll('.app-main *')].filter(el=>el.getClientRects().length&&el.getBoundingClientRect().right>innerWidth+1).slice(0,6).map(el=>el.id||el.className))});
  }
  if(width>=1440){await page.evaluate(()=>showApp('mail'));const columns=await page.locator('#app-mail').evaluate(el=>[...el.children].map(child=>child.getBoundingClientRect().width));assert.ok(Math.abs(columns[1]-columns[2])<2,`unbalanced mail columns ${columns}`);}
 }
 assert.deepEqual(overflows,[],'page overflow');
 await page.setViewportSize({width:1440,height:900});await page.evaluate(()=>showApp('cos'));await page.click('#btn-cos-new');
 assert.equal(await page.locator('#editor-sheet-body').evaluate(el=>el.scrollHeight>el.clientHeight+1),false,'CoS form should fit desktop');
 assert.equal(await page.locator('#editor-sheet-close').isVisible(),false,'remove duplicate close action');
 await page.screenshot({path:'docs/screenshots/ui-cos-editor-tayga-light.png'});
 await page.keyboard.press('Escape');assert.equal(await page.locator('#editor-sheet').isVisible(),false);
 await page.evaluate(()=>{showApp('migration');state.me.features.migration=true;document.getElementById('mig-forms').classList.remove('hidden');});
 await page.click('[data-mig-open="imap"]');assert.equal(await page.locator('#mig-dialog[open]').count(),1);assert.equal(await page.locator('#mig-form-cal').isVisible(),false);
 let posted;
 await page.route('**/api/v1/migration/jobs',async route=>{posted=route.request().postDataJSON();await route.fulfill({status:400,json:{error:'Test connection error'}});});
 await page.fill('#mig-imap-host','imap.example.com');await page.fill('#mig-imap-user','old@example.com');await page.fill('#mig-imap-pass','temporary-password');await page.click('#btn-mig-imap');
 await page.waitForFunction(()=>document.getElementById('mig-dialog-msg').textContent==='Test connection error');assert.equal(posted.kind,'imap');assert.equal(posted.port,993);assert.equal(await page.locator('#mig-dialog[open]').count(),1);
 await page.evaluate(()=>setMsg(document.getElementById('mig-dialog-msg'),''));await page.screenshot({path:'docs/screenshots/ui-migration-dialog-tayga-light.png'});
 await page.keyboard.press('Escape');await page.waitForFunction(()=>document.getElementById('mig-imap-pass').value==='');
 await page.screenshot({path:'docs/screenshots/ui-migration-tayga-light.png'});
 for(const kind of ['cal','card']){await page.click(`[data-mig-open="${kind}"]`);assert.equal(await page.locator(`#mig-form-${kind}`).isVisible(),true);await page.keyboard.press('Escape');}
 await page.setViewportSize({width:390,height:844});await page.click('[data-mig-open="imap"]');assert.equal(await page.locator('#mig-dialog').evaluate(el=>el.scrollWidth>el.clientWidth+1),false);await page.keyboard.press('Escape');
 await page.evaluate(()=>showApp('cos'));await page.click('#btn-cos-new');assert.equal(await page.locator('#editor-sheet').evaluate(el=>el.scrollWidth>el.clientWidth+1),false);await page.screenshot({path:'docs/screenshots/ui-cos-editor-tayga-light-mobile.png'});await page.click('#editor-sheet-cancel');assert.equal(await page.locator('#editor-sheet').isVisible(),false);
 assert.deepEqual(errors,[]);console.log('UI: all pages at five viewport sizes, proportional mail columns, CoS layout, migration modal validation/error/Escape — OK');
}finally{await browser.close();}
