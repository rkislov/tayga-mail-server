import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
import {createCipheriv,createDecipheriv,randomBytes} from 'node:crypto';

// A disposable Tayga server must advertise https://127.0.0.1:PORT while this
// browser uses https://localhost:PORT: downloads then exercise real CORS.
const base=new URL(process.env.TMS_URL||'https://localhost:18443');
assert.equal(base.protocol,'https:');assert.equal(base.hostname,'localhost','Use a disposable loopback fixture only');
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const pages=[],errors=[],fileRequests=[];
 for(let i=0;i<2;i++){
  const context=await browser.newContext({ignoreHTTPSErrors:true,acceptDownloads:true}),page=await context.newPage();pages.push(page);
  page.on('pageerror',e=>errors.push(e.message));
  page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/chat-download/'))fileRequests.push({url:r.url(),headers:r.headers()})});
  await page.goto(base.href);await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForSelector('#view-account:not(.hidden)');
  await page.evaluate(async()=>{showApp('chat');await omemoLocked(omemoInitialize);chatState.peer=state.email;await renderOMEMOButton()});
 }
 for(const page of pages)await page.evaluate(()=>omemoLocked(async c=>{
  for(const id of await omemoDevices(c.account)){
   if(id===c.data.id)continue;const bundle=await omemoBundle(c.account,id),address=new c.lib.OMEMOAddress(c.account,id).toString();
   c.data.trust[address]=omemoB64(bundle.identityKey);c.store.saveIdentity(address,bundle.identityKey);
  }
  c.data.enabled[c.account]=true;await c.save();
 }));
 const [sender,receiver]=pages;
 async function showFile(message){
  return receiver.evaluate(async message=>{
   const body=await omemoDecrypt(message);appendChatBubble({...message,body,encrypted:false,decrypted:true});return body;
  },message);
 }
 async function openAndDownload(message,expected,preview){
  await showFile(message);
  await receiver.evaluate(id=>{
   const bubble=Array.from(document.querySelectorAll('#chat-messages .chat-bubble')).find(x=>JSON.parse(x.dataset.messageKey)[1]===id);
   bubble.querySelector('button').click();
  },message.id);
  const dialog=receiver.locator('.document-preview[open]');await dialog.waitFor();
  if(preview==='image')await receiver.waitForFunction(()=>{const image=document.querySelector('.document-preview img');return image?.complete&&image.naturalWidth>0});
  if(preview==='pdf')await dialog.locator('canvas').waitFor();
  const downloading=receiver.waitForEvent('download');await dialog.getByRole('button',{name:'Скачать',exact:true}).click();
  const download=await downloading,chunks=[];for await(const chunk of await download.createReadStream())chunks.push(chunk);
  assert.deepEqual(Buffer.concat(chunks),expected);
  await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();
  return download.suggestedFilename();
 }
 const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aHioAAAAASUVORK5CYII=','base64');
 function simplePDF(){
  const objects=['<< /Type /Catalog /Pages 2 0 R >>','<< /Type /Pages /Kids [3 0 R] /Count 1 >>','<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Contents 4 0 R >>','<< /Length 0 >>\nstream\n\nendstream'];
  let pdf='%PDF-1.4\n';const offsets=[0];for(let i=0;i<objects.length;i++){offsets.push(Buffer.byteLength(pdf));pdf+=`${i+1} 0 obj\n${objects[i]}\nendobj\n`}
  const xref=Buffer.byteLength(pdf);pdf+='xref\n0 5\n0000000000 65535 f \n'+offsets.slice(1).map(x=>String(x).padStart(10,'0')+' 00000 n \n').join('')+`trailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`;return Buffer.from(pdf);
 }
 for(const fixture of [{name:'Фото #1.png',mimeType:'image/png',buffer:png,preview:'image'},{name:'Билет.pdf',mimeType:'application/pdf',buffer:simplePDF(),preview:'pdf'},{name:'archive.zip',mimeType:'application/zip',buffer:Buffer.from([80,75,0,255,10,32])}]){
  const response=sender.waitForResponse(r=>new URL(r.url()).pathname==='/api/v1/chat/omemo'&&r.request().method()==='POST'&&r.ok());
  await sender.setInputFiles('#chat-file',fixture);
  const message=await (await response).json();
  const uri=await showFile(message),url=new URL('https:'+uri.slice(7));
  assert.equal(url.hostname,'127.0.0.1');assert.equal(url.hash.length,89,'new uploads must use a 12-byte IV');
  const material=Buffer.from(url.hash.slice(1),'hex');url.hash='';
  const downloaded=await sender.context().request.get(url.href);assert.equal(downloaded.status(),200);
  const wire=await downloaded.body();assert.equal(wire.length,fixture.buffer.length+16);assert.notDeepEqual(wire,fixture.buffer);
  const native=createDecipheriv('aes-256-gcm',material.subarray(12),material.subarray(0,12));native.setAuthTag(wire.subarray(-16));
  assert.deepEqual(Buffer.concat([native.update(wire.subarray(0,-16)),native.final()]),fixture.buffer);
  assert.equal(await openAndDownload(message,fixture.buffer,fixture.preview),fixture.name);
 }
 // Native-compatible uploads: both current and historical Conversations IVs.
 for(const ivLength of [12,16]){
  const plain=Buffer.from('native-compatible fixture '+ivLength),key=randomBytes(32),iv=randomBytes(ivLength),cipher=createCipheriv('aes-256-gcm',key,iv);
  const wire=Buffer.concat([cipher.update(plain),cipher.final(),cipher.getAuthTag()]);
  const slot=await sender.evaluate(size=>api('/api/v1/chat/upload',{method:'POST',body:JSON.stringify({name:'native.txt',size,type:'application/octet-stream'})}),wire.length);
  assert.equal((await sender.context().request.put(slot.put,{data:wire,headers:{'Content-Type':'application/octet-stream'}})).status(),201);
  const uri='aesgcm:'+slot.get.slice(6)+'#'+iv.toString('hex')+key.toString('hex');
  const message=await sender.evaluate(uri=>omemoSend(state.email,uri),uri);
  assert.equal(await openAndDownload(message,plain),'native.txt');
  // The one-time PUT URL cannot overwrite an already shared attachment.
  assert.equal((await sender.context().request.put(slot.put,{data:wire})).status(),404);
 }
 assert(fileRequests.length>=5);
 for(const request of fileRequests){assert(!request.url.includes('#'));assert(!request.headers.authorization);assert(!request.headers.cookie);assert(!request.headers.referer)}
 assert.deepEqual(errors,[]);
 console.log('HTTPS OMEMO attachments: UI upload, opaque ciphertext, PNG/PDF preview, exact downloads, 12/16-byte IV receive, cross-origin fetch and no leaked key/credentials — OK');
}finally{await browser.close()}
