import {chromium} from 'playwright-core';
import {createServer} from 'node:http';
import fs from 'node:fs';
import assert from 'node:assert/strict';

// Disposable browser accounts only. Exercise real ratchets and the settings UI;
// PEP and message delivery are the only mocked boundaries.
const ns='eu.siacs.conversations.axolotl',pep=new Map(),sent=[],reads=[],errors=[];
const assets={'/assets/libomemo.js':'internal/frontend/dist/assets/libomemo.js','/omemo.js':'frontend/src/omemo.js','/style.css':'internal/frontend/dist/assets/app.css'};
const server=createServer((req,res)=>{const file=assets[req.url];res.setHeader('Content-Type',file?(req.url.endsWith('.css')?'text/css':'text/javascript'):'text/html');res.end(file?fs.readFileSync(file):'<!doctype html><html data-color-mode="light"><head><meta charset="utf-8"><link rel="stylesheet" href="/style.css"></head><body><h2 id="chat-peer">Bob</h2></body></html>')});
await new Promise(r=>server.listen(0,'127.0.0.1',r));const base='http://127.0.0.1:'+server.address().port;
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 async function setup(page,email){
  await page.goto(base);
  await page.addScriptTag({content:`const state={email:${JSON.stringify(email)}};let lang='ru';const chatBare=s=>String(s||'').split('/')[0].toLowerCase();const api=async(p,o)=>{const {_guard,...opts}=o||{};_guard?.();const result=await window.transport(p,Object.keys(opts).length?opts:undefined);_guard?.();return result};const $=id=>document.getElementById(id);const chatState={peer:'bob@example.com',roster:[]};`});
  await page.addScriptTag({url:base+'/omemo.js'});
 }
 async function device(email){
  const context=await browser.newContext({viewport:{width:1000,height:900}}),page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
  await page.exposeFunction('transport',async(path,options)=>{const u=new URL(path,base);if(!options){const key=u.searchParams.get('owner')+'|'+u.searchParams.get('node');reads.push(key);const xml=pep.get(key);return{items:xml?[{id:'current',xml}]:[]}}const req=JSON.parse(options.body);if(options.method==='PUT'){pep.set(email+'|'+req.node,req.xml);return{ok:true}}assert(!req.body);const ev={id:String(sent.length+1),from:email+'/web',to:req.to,omemo:req.xml,encrypted:true};sent.push(ev);return ev});
  await setup(page,email);const id=await page.evaluate(()=>omemoLocked(async c=>{await omemoInitialize(c);return c.data.id}));return{page,email,id};
 }
 async function trust(device,peer){await device.page.evaluate(peer=>omemoLocked(async c=>{for(const jid of [peer,c.account])for(const id of await omemoDevices(jid)){if(jid===c.account&&id===c.data.id)continue;const bundle=await omemoBundle(jid,id),address=new c.lib.OMEMOAddress(jid,id).toString();c.data.trust[address]=omemoB64(bundle.identityKey);c.store.saveIdentity(address,bundle.identityKey)}c.data.enabled[peer]=true;await c.save()}),peer)}
 const a=await device('alice@example.com'),a2=await device(a.email),b=await device('bob@example.com'),b2=await device(b.email),page=a.page;
 for(const d of [a,a2])await trust(d,b.email);for(const d of [b,b2])await trust(d,a.email);
 const row=id=>page.locator(`.omemo-device[data-device-id="${id}"]`);
 async function open(){await page.evaluate(()=>omemoSettings());await row(a.id).locator('[data-status=current]').waitFor()}
 async function refresh(){await page.getByRole('button',{name:'Обновить устройства'}).click();await page.waitForFunction(()=>!document.querySelector('.omemo-footer button').disabled)}
 async function action(id,name){if(!await row(id).locator(':scope > .omemo-device-details').evaluate(e=>e.open))await row(id).locator(':scope > .omemo-device-details > summary').click();await row(id).getByRole('button',{name,exact:true}).click()}
 async function exclude(id){await action(id,'Не отправлять на устройство');await row(id).locator('[data-status=excluded]').waitFor()}
 async function restore(id){await action(id,'Отпечатки совпадают — включить');await row(id).locator('[data-status=trusted]').waitFor()}
 async function send(d,peer,body){await d.page.evaluate(({peer,body})=>omemoSend(peer,body),{peer,body});return sent.at(-1)}
 const decrypt=(d,ev)=>d.page.evaluate(ev=>omemoDecrypt(ev),ev);
 const ids=ev=>[...ev.omemo.matchAll(/<key rid="(\d+)"/g)].map(m=>Number(m[1])).sort((x,y)=>x-y);
 const original=await send(a,b.email,'all three devices');assert.deepEqual(ids(original),[b.id,b2.id,a2.id].sort((x,y)=>x-y));
 for(const d of [b,b2,a2])assert.equal(await decrypt(d,original),'all three devices');
 const oldReply=await send(b2,a.email,'cached history');assert.equal(await decrypt(a,oldReply),'cached history');
 await open();assert.equal(await row(a.id).locator('[data-omemo-action=exclude]').count(),0);
 await assert.rejects(()=>page.evaluate(()=>omemoLocked(c=>omemoExcludeDevice(c,c.account,c.data.id))),/cannot be excluded|нельзя исключить/);
 const listKey=b.email+'|'+ns+'.devicelist',list=pep.get(listKey),bundleKey=b.email+'|'+ns+'.bundles:'+b2.id,bundle=pep.get(bundleKey);
 await exclude(b2.id);assert.equal(pep.get(listKey),list,'exclusion must not change the global device list');
 const active=await send(a,b.email,'remaining devices');assert.deepEqual(ids(active),[b.id,a2.id].sort((x,y)=>x-y));
 for(const d of [b,a2])assert.equal(await decrypt(d,active),'remaining devices');
 await assert.rejects(()=>decrypt(b2,active),/Not encrypted for this device/);
 assert.equal(await decrypt(a,oldReply),'cached history');
 const newReply=await send(b2,a.email,'new excluded sender');await assert.rejects(()=>decrypt(a,newReply),/исключено|excluded/);
 assert.equal(await decrypt(a2,newReply),'new excluded sender','another browser keeps its independent trust');

 // Absent bundles must not block delivery to remaining verified devices.
 pep.delete(bundleKey);reads.length=0;
 const missing=await send(a,b.email,'missing excluded bundle');assert.equal(await decrypt(b,missing),'missing excluded bundle');assert(!reads.includes(bundleKey));
 pep.set(listKey,`<list xmlns="${ns}"><device id="${b.id}"/></list>`);
 await refresh();await row(b2.id).locator('[data-status=excluded]').waitFor();
 await action(b2.id,'Сохранить название'); // Card remains usable while absent.
 assert.equal(await row(b2.id).getByRole('button',{name:'Отпечатки совпадают — включить'}).count(),0);
 await setup(page,a.email);await open();await row(b2.id).locator('[data-status=excluded]').waitFor();
 const absent=await send(a,b.email,'absent excluded device');assert.equal(await decrypt(b,absent),'absent excluded device');

 // Reappearing IDs (even with different keys) do not silently clear exclusion.
 pep.set(listKey,list);pep.set(bundleKey,pep.get(b.email+'|'+ns+'.bundles:'+b.id));
 await refresh();await row(b2.id).locator('[data-status=excluded]').waitFor();
 const rotated=await send(a,b.email,'reappearing excluded device');assert(!ids(rotated).includes(b2.id));assert.equal(await decrypt(b,rotated),'reappearing excluded device');
 pep.set(bundleKey,bundle);
 await action(b2.id,'Отпечатки совпадают — включить');await page.getByRole('status').filter({hasText:'Ключ изменился во время проверки'}).waitFor();
 await row(b2.id).locator('[data-status=excluded]').waitFor();
 await refresh();

 // Every recipient excluded: own-device copies cannot masquerade as delivery.
 await exclude(b.id);let count=sent.length;
 await assert.rejects(()=>send(a,b.email,'must not send'),/Нет доступных устройств/);assert.equal(sent.length,count);
 await page.getByText('Нет доступных устройств получателя.',{exact:false}).first().waitFor();
 await page.setViewportSize({width:390,height:844});await page.evaluate(()=>document.documentElement.dataset.colorMode='dark');
 if(!await row(b2.id).locator(':scope > .omemo-device-details').evaluate(e=>e.open))await row(b2.id).locator(':scope > .omemo-device-details > summary').click();
 await row(b2.id).scrollIntoViewIfNeeded();
 const box=await page.locator('.omemo-dialog').boundingBox();assert(box.x>=0&&box.y>=0&&box.x+box.width<=390&&box.y+box.height<=844);
 assert(await page.locator('.omemo-body').evaluate(e=>e.scrollWidth<=e.clientWidth+1));
  await page.screenshot({path:'/private/tmp/tayga-omemo-excluded-mobile.png',animations:'disabled'});
 await restore(b.id);await exclude(a2.id);
 const one=await send(a,b.email,'only recipient');assert.deepEqual(ids(one),[b.id]);assert.equal(await decrypt(b,one),'only recipient');

 // Unknown active devices continue to block, rather than being skipped.
 const unknown=2147483647;assert(![a.id,a2.id,b.id,b2.id].includes(unknown));
 pep.set(listKey,list.replace('</list>',`<device id="${unknown}"/></list>`));pep.set(b.email+'|'+ns+'.bundles:'+unknown,bundle);
 count=sent.length;await assert.rejects(()=>send(a,b.email,'unknown must block'),/Verify device key/);assert.equal(sent.length,count);pep.set(listKey,list);
 await restore(b2.id);
 const restored=await send(a,b.email,'restored after explicit verification');assert.deepEqual(ids(restored),[b.id,b2.id].sort((x,y)=>x-y));for(const d of [b,b2])assert.equal(await decrypt(d,restored),'restored after explicit verification');
 // A stale verification dialog cannot undo an exclusion saved in another tab.
 await action(b2.id,'Отозвать доверие');await row(b2.id).locator('[data-status=unverified]').waitFor();
 await page.evaluate(({jid,id})=>omemoLocked(async c=>{omemoExcludeDevice(c,jid,id);await c.save()}),{jid:b2.email,id:b2.id});
 await action(b2.id,'Отпечатки совпадают — доверять');await page.getByRole('status').filter({hasText:'Настройка устройства изменилась'}).waitFor();
 await refresh();await row(b2.id).locator('[data-status=excluded]').waitFor();
 await page.setViewportSize({width:1000,height:900});await page.evaluate(()=>document.documentElement.dataset.colorMode='light');await page.locator('.omemo-body').evaluate(e=>e.scrollTop=0);
  await page.screenshot({path:'/private/tmp/tayga-omemo-excluded-desktop.png',animations:'disabled'});
 assert.deepEqual(errors,[]);
 console.log('OMEMO device exclusion: recipient and own devices, absent/rotated keys, persisted exclusions, blocked empty delivery, cached history, fresh verification, stale dialogs, account/browser scope and mobile UI — OK');
}finally{await browser.close();await new Promise(r=>server.close(r))}
