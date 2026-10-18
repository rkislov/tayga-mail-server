import {chromium} from 'playwright-core';
import {createServer} from 'node:http';
import fs from 'node:fs';
import assert from 'node:assert/strict';
const pep=new Map(),sent=[];
const assets={'/assets/libomemo.js':'internal/frontend/dist/assets/libomemo.js','/omemo.js':'frontend/src/omemo.js','/style.css':'internal/frontend/dist/assets/app.css'};
const server=createServer((req,res)=>{const file=assets[req.url];if(file){res.setHeader('Content-Type',req.url.endsWith('.css')?'text/css':'text/javascript');res.end(fs.readFileSync(file))}else{res.setHeader('Content-Type','text/html');res.end('<!doctype html><html data-color-mode="light"><head><meta charset="utf-8"><link rel="stylesheet" href="/style.css"></head><body><h2 id="chat-peer">Bob</h2></body></html>')}});
await new Promise(r=>server.listen(0,'127.0.0.1',r));const base='http://127.0.0.1:'+server.address().port;
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const errors=[];
 async function device(email){
  const context=await browser.newContext({viewport:{width:1000,height:900}}),page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
  await page.exposeFunction('transport',async(path,options)=>{const u=new URL(path,base);if(!options){const xml=pep.get(u.searchParams.get('owner')+'|'+u.searchParams.get('node'));return{items:xml?[{id:'current',xml}]:[]}}const req=JSON.parse(options.body);if(options.method==='PUT'){pep.set(email+'|'+req.node,req.xml);return{ok:true}}sent.push(req);return{id:String(sent.length),from:email,to:req.to,omemo:req.xml}});
  await page.goto(base);await page.addScriptTag({content:`const state={email:${JSON.stringify(email)}};let lang='ru';const chatBare=s=>String(s||'').split('/')[0].toLowerCase();const api=(p,o)=>window.transport(p,o);const $=id=>document.getElementById(id);let chatRefreshes=0;const openChatPeer=async()=>{chatRefreshes++};const chatState={peer:'bob@example.com',roster:[{jid:'bob@example.com',name:'Bob'}]};`});await page.addScriptTag({url:base+'/omemo.js'});
  const id=await page.evaluate(()=>omemoLocked(async c=>{await omemoInitialize(c);return c.data.id}));return{page,context,email,id};
 }
 const a=await device('alice@example.com'),a2=await device('alice@example.com'),b=await device('bob@example.com'),b2=await device('bob@example.com'),page=a.page;
 const bundleKey=b.email+'|eu.siacs.conversations.axolotl.bundles:'+b.id;
 const originalBundle=pep.get(bundleKey);
 await page.evaluate(()=>renderOMEMOButton());await page.getByRole('button',{name:'Шифрование: выкл.'}).click();
 await page.locator('.omemo-device').nth(3).waitFor();
 assert.equal(await page.locator('.omemo-device').count(),4);
 assert.equal(await page.locator('.omemo-fingerprint:visible').count(),0);
 assert.equal(await page.locator('[data-key-status=unverified]').count(),3);
 assert.equal(await page.locator('[data-key-status=current]').count(),1);
 await page.getByRole('button',{name:'Включить шифрование чата',exact:true}).click();
 await page.getByRole('button',{name:'Выключить шифрование чата',exact:true}).waitFor();
 await assert.rejects(()=>page.evaluate(()=>omemoSend('bob@example.com','must stay encrypted')));assert.equal(sent.length,0);
 const row=id=>page.locator(`.omemo-device[data-device-id="${id}"]`);
 async function refresh(){await page.getByRole('button',{name:'Обновить устройства'}).click();await page.waitForFunction(()=>!document.querySelector('.omemo-footer button').disabled)}
 await row(b.id).locator('summary').click();
 pep.set(bundleKey,pep.get(b2.email+'|eu.siacs.conversations.axolotl.bundles:'+b2.id));
 await row(b.id).getByRole('button',{name:'Отпечатки совпадают — доверять'}).click();
 await page.getByRole('status').filter({hasText:'Ключ изменился во время проверки'}).waitFor();
 assert.equal(await page.evaluate(()=>omemoLocked(c=>Object.keys(c.data.trust).length)),0);
 pep.set(bundleKey,originalBundle);
 await row(b.id).getByRole('button',{name:'Отпечатки совпадают — доверять'}).click();
 await row(b.id).locator('[data-status=trusted]').waitFor();
 await row(b.id).locator('summary').click();
 await row(b.id).getByLabel('Название для вас').fill('Телефон собеседника');
 await row(b.id).getByRole('button',{name:'Сохранить название'}).click();
 await row(b.id).getByText('Телефон собеседника',{exact:true}).waitFor();
 for(const id of [a2.id,b2.id]){await row(id).locator('summary').click();await row(id).getByRole('button',{name:'Отпечатки совпадают — доверять'}).click();await row(id).locator('[data-status=trusted]').waitFor()}
 await page.getByText('Все устройства проверены. Можно отправлять зашифрованные сообщения.').waitFor();
 await refresh();await row(b.id).getByText('Телефон собеседника',{exact:true}).waitFor();
 await page.locator('.omemo-body').evaluate(e=>e.scrollTop=0);
 await page.screenshot({path:'/private/tmp/tayga-omemo-settings-desktop.png'});
 pep.set(bundleKey,pep.get(b2.email+'|eu.siacs.conversations.axolotl.bundles:'+b2.id));
 await refresh();await row(b.id).locator('[data-status=changed]').waitFor();
 await assert.rejects(()=>page.evaluate(()=>omemoSend('bob@example.com','changed key')));
 pep.set(bundleKey,originalBundle);await refresh();await row(b.id).locator('[data-status=trusted]').waitFor();
 await row(b.id).locator('summary').click();await row(b.id).getByRole('button',{name:'Отозвать доверие'}).click();await row(b.id).locator('[data-status=unverified]').waitFor();
 await assert.rejects(()=>page.evaluate(()=>omemoSend('bob@example.com','blocked again')));assert.equal(sent.length,0);
 // A missing bundle must leave other devices and the encryption switch usable.
 const missingKey=b2.email+'|eu.siacs.conversations.axolotl.bundles:'+b2.id;pep.delete(missingKey);
 await refresh();await row(b2.id).locator('[data-status=unavailable]').waitFor();
 assert.equal(await page.locator('.omemo-device').count(),4);
 assert(await page.getByRole('button',{name:'Выключить шифрование чата',exact:true}).isEnabled());
 await page.getByRole('button',{name:'Закрыть',exact:true}).click();await page.locator('.omemo-dialog').waitFor({state:'detached'});
 assert.equal(await page.evaluate(()=>chatRefreshes),1);
 await page.getByRole('button',{name:'Шифрование: вкл.'}).click();await row(b2.id).locator('[data-status=unavailable]').waitFor();
 await page.setViewportSize({width:390,height:844});await page.evaluate(()=>document.documentElement.dataset.colorMode='dark');
 await row(b.id).locator('summary').click();
 const box=await page.locator('.omemo-dialog').boundingBox();assert(box.x>=0&&box.y>=0&&box.x+box.width<=390&&box.y+box.height<=844);
 assert(await page.locator('.omemo-body').evaluate(e=>e.scrollWidth<=e.clientWidth+1));
 await page.screenshot({path:'/private/tmp/tayga-omemo-settings-mobile.png'});
 // Do not apply an old dialog's trust decisions to a different signed-in account.
 await page.evaluate(()=>state.email='different@example.com');
 await row(b.id).getByRole('button',{name:'Отпечатки совпадают — доверять'}).click();
 assert.equal(await page.evaluate(()=>omemoRead('accounts','different@example.com')),undefined);
 await page.getByRole('button',{name:'Закрыть',exact:true}).click();
 await page.locator('.omemo-dialog').waitFor({state:'detached'});
 assert.equal(await page.evaluate(()=>chatRefreshes),1);
 assert.deepEqual(errors,[]);
 console.log('OMEMO settings: explicit trust, key rotation rejection, revocation, local labels, partial bundle failure, account isolation and mobile layout — OK');
}finally{await browser.close();await new Promise(r=>server.close(r))}
