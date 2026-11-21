import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import {chromium} from 'playwright-core';
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
const out='output/ui-fixes/mobile';await fs.mkdir(out,{recursive:true});
try {
 const page=await browser.newPage({viewport:{width:390,height:844},isMobile:true,hasTouch:true});
 let seeded=false;const errors=[];page.on('pageerror',e=>errors.push(e.message));
 // Exercise current compiled assets with the disposable seed database.
 for(const name of ['app.js','app.css'])await page.route('**/assets/'+name+'*',r=>r.fulfill({path:'internal/frontend/dist/assets/'+name}));
 await page.goto('http://127.0.0.1:18080');await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForFunction(()=>state.me?.id);
 await page.evaluate(()=>showApp('chat'));await page.waitForSelector('.mm-chat-channel');
 for(const width of [320,390,440,768]) {
  await page.setViewportSize({width,height:844});
  const list=await page.locator('#mm-chat-channels').boundingBox();assert.ok(list.height>300,JSON.stringify(list));
  await page.locator('.mm-chat-channel').filter({hasText:'Town Square'}).click();await page.waitForSelector('#mm-chat-thread:not(.hidden)');
  await page.waitForFunction(()=>document.querySelector('#mm-chat-posts').childElementCount>0);
  if(!seeded){await page.fill('#mm-chat-body','Доброе утро! Обновления проекта теперь можно получать прямо в чате.');await page.click('#mm-chat-send button[type=submit]');await page.waitForFunction(()=>!document.querySelector('#mm-chat-body').value);seeded=true;}
  assert.equal(await page.locator('.chat-roster-pane').isVisible(),false);
  assert.equal(await page.locator('#mm-chat-send').isVisible(),true);
  const send=await page.locator('#mm-chat-send').boundingBox();assert.ok(send.y+send.height<=844);
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  if(width===390){await page.screenshot({path:out+'/thread.png'});await page.setViewportSize({width,height:420});await page.waitForFunction(()=>document.querySelector('#mm-chat-send').getBoundingClientRect().bottom<=420);await page.setViewportSize({width,height:844})}
  await page.locator('#mm-chat-thread [data-pane-back]').click();assert.equal(await page.locator('#mm-chat-thread').isVisible(),false);
  if(width===390)await page.screenshot({path:out+'/list.png'});
 }
 await page.setViewportSize({width:1440,height:960});assert.equal(await page.locator('.chat-roster-pane').isVisible(),true);assert.equal(await page.locator('.chat-thread-pane').isVisible(),true);
 assert.deepEqual(errors,[]);console.log('Mobile chat: 320/390/440/768px roster, navigation, composer, narrow viewport and desktop passed.');
} finally {await browser.close()}
