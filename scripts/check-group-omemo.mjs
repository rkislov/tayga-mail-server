import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
import {createDecipheriv} from 'node:crypto';

// Four independent browser stores on a disposable, seeded HTTPS loopback server.
const base=new URL(process.env.TMS_URL||'https://localhost:18443');
assert.equal(base.protocol,'https:');assert.equal(base.hostname,'localhost');
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
const errors=[],downloads=[];
try {
 async function login(email,password){
  const context=await browser.newContext({ignoreHTTPSErrors:true,acceptDownloads:true,viewport:{width:1280,height:900}}),page=await context.newPage();
  page.on('pageerror',e=>errors.push(e.message));
  page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/chat-download/'))downloads.push({url:r.url(),headers:r.headers()})});
  await page.goto(base.href);await page.fill('#email',email);await page.fill('#password',password);await page.click('#form-login button[type=submit]');await page.waitForSelector('#view-account:not(.hidden)');
  await page.evaluate(()=>omemoLocked(omemoInitialize));return page;
 }
 const alice=await login('admin@example.com','changeme');
 const fixture=await alice.evaluate(async()=>{
  const stamp=Date.now(),bob=`omemo-bob-${stamp}@example.com`,carol=`omemo-carol-${stamp}@example.com`;
  for(const [email,display_name] of [[bob,'Борис Тестов'],[carol,'Карина Тестова']])await api('/api/v1/admin/users',{method:'POST',body:JSON.stringify({email,display_name,password:'fixture-password'})});
  const room=await api('/api/v1/chat/rooms',{method:'POST',body:JSON.stringify({name:'Защищённая команда',members:[bob,carol]})});return {bob,carol,room:room.jid};
 });
 const alicePhone=await login('admin@example.com','changeme'),bob=await login(fixture.bob,'fixture-password'),carol=await login(fixture.carol,'fixture-password');
 const pages=[alice,alicePhone,bob,carol];
 const ids=await Promise.all(pages.map(p=>p.evaluate(()=>omemoLocked(c=>c.data.id))));
 async function trust(page){
  // Only the disposable test fixtures explicitly trust each other's public keys.
  await page.evaluate(room=>omemoLocked(async c=>{
   const audience=await omemoAudience(room);
   for(const {jid} of audience.members)for(const id of await omemoDevices(jid)){
    if(jid===c.account&&id===c.data.id)continue;
    const bundle=await omemoBundle(jid,id),address=new c.lib.OMEMOAddress(jid,id).toString();
    c.data.trust[address]=omemoB64(bundle.identityKey);c.store.saveIdentity(address,bundle.identityKey);
   }
   c.data.enabled[room]=true;await c.save();
  }),fixture.room);
 }
 async function open(page){
  await page.evaluate(async room=>{showApp('chat');await refreshChat();await openChatPeer(room);await renderOMEMOButton()},fixture.room);
  await page.waitForFunction(room=>chatState.peer===room&&chatState.history?.loaded&&!chatState.history.loading,fixture.room);
 }
 // A valid group membership does not implicitly trust any encryption key.
 await alice.evaluate(room=>omemoLocked(async c=>{c.data.enabled[room]=true;await c.save()}),fixture.room);
 let sentPosts=0;alice.on('request',r=>{if(r.method()==='POST'&&new URL(r.url()).pathname==='/api/v1/chat/rooms/omemo')sentPosts++});
 await assert.rejects(()=>alice.evaluate(room=>omemoSend(room,'must not send without verification'),fixture.room),/Verify device key/);
 assert.equal(sentPosts,0);
 for(const page of pages)await trust(page);
 for(const page of pages)await open(page);
 async function send(page,body){
  const response=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/api/v1/chat/rooms/omemo'&&r.ok());
  await page.fill('#chat-body',body);await page.locator('#form-chat-send button[type=submit]').click();
  const message=await (await response).json();assert.equal(message.body,'');assert.equal(message.encrypted,true);assert(message.omemo);return message;
 }
 async function visible(page,message,body){
  await page.waitForFunction(({id,body})=>[...document.querySelectorAll('#chat-messages .chat-bubble')].some(n=>n.dataset.messageKey?.includes(id)&&n.dataset.encryption==='omemo'&&n.textContent.includes(body)),{id:message.id,body});
 }
 const first=await send(alice,'Четыре устройства · первая секретная запись');
 for(const page of pages)await visible(page,first,'первая секретная запись');
 assert.equal(await bob.locator('#chat-messages .chat-bubble strong').first().innerText(),'Роман Кислов');
 const reply=await send(bob,'Ответ Бориса всем участникам');
 for(const page of pages)await visible(page,reply,'Ответ Бориса');
 const history=await alice.evaluate(room=>api('/api/v1/chat/rooms/history?room='+encodeURIComponent(room)),fixture.room);
 assert.equal(history.messages.length,2);assert(history.messages.every(m=>m.body===''&&m.encrypted&&m.omemo));
 // Browser reload retains both ratchets and decrypted history; no shared stores.
 await bob.reload();await bob.waitForSelector('#view-account:not(.hidden)');await open(bob);
 await visible(bob,first,'первая секретная запись');await visible(bob,reply,'Ответ Бориса');
 console.log('Group text: explicit trust, four-device fanout, names, ciphertext-only archive and reload — OK');
 // Upload through the actual compose UI. Both directions must download exact bytes.
 const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aHioAAAAASUVORK5CYII=','base64');
 function pdf(){
  const objects=['<< /Type /Catalog /Pages 2 0 R >>','<< /Type /Pages /Kids [3 0 R] /Count 1 >>','<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Contents 4 0 R >>','<< /Length 0 >>\nstream\n\nendstream'];
  let data='%PDF-1.4\n';const offsets=[];for(let i=0;i<objects.length;i++){offsets.push(Buffer.byteLength(data));data+=`${i+1} 0 obj\n${objects[i]}\nendobj\n`}
  const xref=Buffer.byteLength(data);return Buffer.from(data+'xref\n0 5\n0000000000 65535 f \n'+offsets.map(x=>String(x).padStart(10,'0')+' 00000 n \n').join('')+`trailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`);
 }
 for(const [sender,receiver,file,preview] of [[alice,carol,{name:'Команда.png',mimeType:'image/png',buffer:png},'image'],[bob,alicePhone,{name:'Документ.pdf',mimeType:'application/pdf',buffer:pdf()},'pdf']]){
  const response=sender.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname==='/api/v1/chat/rooms/omemo'&&r.ok());
  await sender.setInputFiles('#chat-file',file);const message=await (await response).json();
  const uri=await receiver.evaluate(m=>omemoDecrypt(m),message),url=new URL('https:'+uri.slice(7)),material=Buffer.from(url.hash.slice(1),'hex');url.hash='';
  const wire=await (await receiver.context().request.get(url.href)).body();assert.notDeepEqual(wire,file.buffer);
  const cipher=createDecipheriv('aes-256-gcm',material.subarray(12),material.subarray(0,12));cipher.setAuthTag(wire.subarray(-16));assert.deepEqual(Buffer.concat([cipher.update(wire.subarray(0,-16)),cipher.final()]),file.buffer);
  await receiver.waitForFunction(id=>[...document.querySelectorAll('#chat-messages .chat-bubble')].some(n=>n.dataset.messageKey?.includes(id)&&n.dataset.encryption==='omemo'&&n.querySelector('button')),message.id);
  await receiver.evaluate(id=>[...document.querySelectorAll('#chat-messages .chat-bubble')].find(n=>n.dataset.messageKey?.includes(id)).querySelector('button').click(),message.id);
  const dialog=receiver.locator('.document-preview[open]');await dialog.waitFor();
  if(preview==='image')await receiver.waitForFunction(()=>{const i=document.querySelector('.document-preview img');return i?.complete&&i.naturalWidth>0});else await dialog.locator('canvas').waitFor();
  const pending=receiver.waitForEvent('download');await dialog.getByRole('button',{name:'Скачать',exact:true}).click();const downloaded=await pending,chunks=[];for await(const chunk of await downloaded.createReadStream())chunks.push(chunk);
  assert.deepEqual(Buffer.concat(chunks),file.buffer);assert.equal(downloaded.suggestedFilename(),file.name);await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();
 }
 console.log('Group attachments: PNG and PDF UI upload/preview/download, opaque bytes, both directions — OK');
 // A fifth, newly published device must stop sending until explicitly verified.
 const newBob=await login(fixture.bob,'fixture-password');
 const countBefore=sentPosts;
 await assert.rejects(()=>alice.evaluate(room=>omemoSend(room,'new untrusted device'),fixture.room),/Verify device key/);assert.equal(sentPosts,countBefore);
 await trust(alice);await trust(newBob);
 // Decrypt in reverse order and try an incorrect matching RID before the real one.
 // The failed candidate must not consume the real prekey or ratchet state.
 const older=await alice.evaluate(room=>omemoSend(room,'первое для нового устройства'),fixture.room);
 const newer=await alice.evaluate(room=>omemoSend(room,'второе для нового устройства'),fixture.room);
 assert.equal(await newBob.evaluate(m=>{
  const d=omemoXML(m.omemo),header=omemoElement(d,'header');
  return omemoLocked(c=>c.data.id).then(id=>{const key=omemoElements(d,'key').find(k=>Number(k.getAttribute('rid'))===id).cloneNode(true);key.textContent='YWJj';header.prepend(key);return omemoDecrypt({...m,omemo:new XMLSerializer().serializeToString(d)})});
 },newer),'второе для нового устройства');
 assert.equal(await newBob.evaluate(m=>omemoDecrypt(m),older),'первое для нового устройства');
 await assert.rejects(()=>newBob.evaluate(m=>omemoDecrypt(m),first),/Not encrypted for this device/);
 console.log('New device: blocked until verified, old history inaccessible, reverse-order receive and RID trial isolation — OK');
 // Pause a prepared POST, remove Carol, then send its stale membership revision.
 let arrived,release;const ready=new Promise(r=>arrived=r),gate=new Promise(r=>release=r);
 const matcher=url=>url.pathname==='/api/v1/chat/rooms/omemo';
 await alice.route(matcher,async route=>{if(route.request().method()!=='POST')return route.continue();arrived();await gate;await route.continue()});
 const stale=alice.evaluate(room=>omemoSend(room,'must not reach removed member'),fixture.room).then(()=>null,e=>e.message);await ready;
 await alicePhone.evaluate(({room,carol})=>api('/api/v1/chat/rooms/'+encodeURIComponent(room),{method:'PATCH',body:JSON.stringify({email:carol,affiliation:'none'})}),fixture);
 release();assert.match(await stale,/Состав группы изменился/);await alice.unroute(matcher);
 const after=await alice.evaluate(room=>omemoSend(room,'после исключения'),fixture.room);
 assert(!after.omemo.includes(`rid="${ids[3]}"`));
 await assert.rejects(()=>carol.evaluate(room=>api('/api/v1/chat/rooms/history?room='+encodeURIComponent(room)),fixture.room));
 await assert.rejects(()=>carol.evaluate(m=>omemoDecrypt(m),after),/Not encrypted for this device/);
 assert.equal(await bob.evaluate(m=>omemoDecrypt(m),after),'после исключения');
 const finalHistory=await alice.evaluate(room=>api('/api/v1/chat/rooms/history?room='+encodeURIComponent(room)),fixture.room);
 assert.equal(finalHistory.messages.length,7,'rejected stale message must not enter the archive');
 console.log('Membership change during send: stale POST rejected, no archive entry, removed device omitted, remaining devices decrypt — OK');
 await alice.setViewportSize({width:390,height:844});await open(alice);await alice.evaluate(()=>omemoSettings());
 await alice.getByRole('button',{name:'Выключить шифрование чата'}).waitFor();
 assert((await alice.locator('.omemo-dialog').innerText()).includes('Борис Тестов'));
 assert(await alice.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
 await alice.screenshot({path:'/private/tmp/tayga-group-omemo-mobile.png'});
 assert(downloads.length>=2);for(const r of downloads){assert(!r.url.includes('#'));assert(!r.headers.authorization);assert(!r.headers.cookie);assert(!r.headers.referer)}
 assert.deepEqual(errors,[]);console.log('Mobile group settings and attachment key/credential isolation — OK');
}finally{await browser.close()}
