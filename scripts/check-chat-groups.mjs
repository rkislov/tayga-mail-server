import {chromium} from 'playwright-core';
import fs from 'node:fs';
import assert from 'node:assert/strict';
const chrome=['/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',`${process.env.HOME}/Library/Caches/ms-playwright/chromium-1208/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing`].find(p=>fs.existsSync(p));
const browser=await chromium.launch({executablePath:chrome,headless:true});
try{
 const page=await browser.newPage({viewport:{width:1440,height:960}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(process.env.TMS_URL||'http://127.0.0.1:18089');
 await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');
 await page.waitForSelector('#view-account:not(.hidden)');
 await page.addStyleTag({path:'internal/frontend/dist/assets/app.css'});
 await page.evaluate(()=>{applyColorMode('light');showApp('chat')});
 await page.locator('#section-create .create-fab').click();
 await page.getByRole('button',{name:'Групповой чат',exact:true}).click();
 await page.locator('.chat-group-dialog input[name=name]').fill('Команда проекта');
 await page.locator('.chat-group-dialog textarea').fill('alice@example.com, bob@example.com');
 await page.screenshot({path:'docs/screenshots/ui-chat-group-material3.png'});
 assert.equal(await page.locator('.chat-group-dialog').count(),1);
 await page.locator('.chat-group-dialog header button').click();
 await page.evaluate(()=>showApp('mail'));await page.screenshot({path:'docs/screenshots/ui-mail-material3.png'});
 await page.evaluate(()=>applyColorMode('dark'));await page.screenshot({path:'docs/screenshots/ui-mail-material3-dark.png'});
 await page.setViewportSize({width:390,height:844});await page.evaluate(()=>showApp('chat'));await page.locator('#section-create .create-fab').click();await page.getByRole('button',{name:'Групповой чат',exact:true}).click();
 const bounds=await page.locator('.chat-group-dialog').boundingBox();assert(bounds.x>=0&&bounds.x+bounds.width<=391);
 assert.deepEqual(errors,[]);console.log('Group menu, dialog, mobile bounds and Material 3 screenshots: OK');
}finally{await browser.close()}
