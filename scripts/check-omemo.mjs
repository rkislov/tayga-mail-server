import {chromium} from 'playwright-core';
import {createServer} from 'node:http';
import fs from 'node:fs';
import assert from 'node:assert/strict';
import {createCipheriv,createDecipheriv,randomBytes} from 'node:crypto';
const pep=new Map(),sent=[];
const server=createServer((req,res)=>{if(req.url==='/assets/libomemo.js'){res.setHeader('Content-Type','text/javascript');res.end(fs.readFileSync('internal/frontend/dist/assets/libomemo.js'))}else if(req.url==='/omemo.js'){res.setHeader('Content-Type','text/javascript');res.end(fs.readFileSync('frontend/src/omemo.js'))}else{res.setHeader('Content-Type','text/html');res.end('<!doctype html><title>OMEMO test</title>')}});
await new Promise(r=>server.listen(0,'127.0.0.1',r));const base='http://127.0.0.1:'+server.address().port;
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 async function setup(page,email){await page.goto(base);await page.addScriptTag({content:`const state={email:${JSON.stringify(email)}};const chatBare=s=>String(s||'').split('/')[0].toLowerCase();const api=async(p,o)=>{const {_guard,...opts}=o||{};_guard?.();const result=await window.transport(p,Object.keys(opts).length?opts:undefined);_guard?.();return result};`});await page.addScriptTag({url:base+'/omemo.js'})}
 async function device(email){const context=await browser.newContext(),page=await context.newPage();await page.exposeFunction('transport',async(path,options)=>{const u=new URL(path,base);if(!options){const xml=pep.get(u.searchParams.get('owner')+'|'+u.searchParams.get('node'));return{items:xml?[{id:'current',xml}]:[]}}const req=JSON.parse(options.body);if(options.method==='PUT'){pep.set(email+'|'+req.node,req.xml);return{ok:true}}assert(!req.body);assert(!req.xml.includes('secret text'));const ev={id:String(sent.length+1),from:email+'/web',to:req.to,omemo:req.xml,encrypted:true};sent.push(ev);return ev});await setup(page,email);await page.evaluate(()=>omemoLocked(omemoInitialize));return{page,context,email}}
 async function trust(device,peer){await device.page.evaluate(peer=>omemoLocked(async c=>{for(const jid of [peer,c.account])for(const id of await omemoDevices(jid)){if(jid===c.account&&id===c.data.id)continue;const bundle=await omemoBundle(jid,id),a=new c.lib.OMEMOAddress(jid,id).toString();c.data.trust[a]=omemoB64(bundle.identityKey);c.store.saveIdentity(a,bundle.identityKey)}c.data.enabled[peer]=true;await c.save()}),peer)}
 const a=await device('alice@example.com'),b=await device('bob@example.com'),b2=await device('bob@example.com');
 await trust(a,b.email);await trust(b,a.email);await trust(b2,a.email);
 await a.page.evaluate(()=>omemoSend('bob@example.com','secret text — Привет'));
 const first=sent[0];assert.equal(await b.page.evaluate(m=>omemoDecrypt(m),first),'secret text — Привет');assert.equal(await b2.page.evaluate(m=>omemoDecrypt(m),first),'secret text — Привет');
 await b.page.evaluate(()=>omemoSend('alice@example.com','Ответ'));assert.equal(await a.page.evaluate(m=>omemoDecrypt(m),sent[1]),'Ответ');
 await a.page.evaluate(()=>omemoSend('bob@example.com','second'));await a.page.evaluate(()=>omemoSend('bob@example.com','third'));
 assert.equal(await b.page.evaluate(m=>omemoDecrypt(m),sent[3]),'third');assert.equal(await b.page.evaluate(m=>omemoDecrypt(m),sent[2]),'second');
 await setup(b.page,b.email);assert.equal(await b.page.evaluate(m=>omemoDecrypt(m),first),'secret text — Привет');
 await a.page.evaluate(()=>omemoSend('bob@example.com','integrity'));const corrupted={...sent[4],id:'tampered',omemo:sent[4].omemo.replace(/(<payload>)[A-Za-z0-9+/]/,'$1!')};await assert.rejects(()=>b.page.evaluate(m=>omemoDecrypt(m),corrupted));assert.equal(await b.page.evaluate(m=>omemoDecrypt(m),sent[4]),'integrity');
 await a.page.evaluate(()=>omemoLocked(async c=>{c.data.trust={};await c.save()}));await assert.rejects(()=>a.page.evaluate(()=>omemoSend('bob@example.com','blocked')));assert.equal(sent.length,5);
 const fileResult=await a.page.evaluate(async()=>{const file=new File(['private PDF bytes'],'test.pdf');const encrypted=await omemoEncryptFile(file);const uri=omemoFileURI('https://example.com/chat-download/token/test.pdf',encrypted),descriptor=omemoFileDescriptor(uri);const decoded=await omemoDecryptFile(descriptor,encrypted.blob);const bytes=new Uint8Array(await encrypted.blob.arrayBuffer());bytes[0]^=1;let rejected=false;try{await omemoDecryptFile(descriptor,new Blob([bytes]))}catch{rejected=true}return {text:await decoded.text(),url:descriptor.url,rejected,uri}});
 assert.equal(fileResult.text,'private PDF bytes');assert(!fileResult.url.includes('#'));assert(fileResult.rejected);assert(fileResult.uri.startsWith('aesgcm://'));
 // Independent AES-GCM implementation checks the wire format in both
 // directions: 12-byte IV, 32-byte key, ciphertext followed by a 16-byte tag.
 const nativeKey=randomBytes(32),nativeIV=randomBytes(12),nativeCipher=createCipheriv('aes-256-gcm',nativeKey,nativeIV);
 const nativeBytes=Buffer.concat([nativeCipher.update('native attachment fixture'),nativeCipher.final(),nativeCipher.getAuthTag()]);
 const nativeURI='aesgcm://files.example.com/ticket.pdf#'+nativeIV.toString('hex')+nativeKey.toString('hex');
 assert.equal(await a.page.evaluate(async({uri,bytes})=>await (await omemoDecryptFile(omemoFileDescriptor(uri),new Blob([Uint8Array.from(bytes)]))).text(),{uri:nativeURI,bytes:[...nativeBytes]}),'native attachment fixture');
 const browserFile=await a.page.evaluate(async()=>{const f=await omemoEncryptFile(new File(['browser attachment fixture'],'ticket.pdf'));return {key:[...f.key],iv:[...f.iv],bytes:[...new Uint8Array(await f.blob.arrayBuffer())]}});
 const wire=Buffer.from(browserFile.bytes),decipher=createDecipheriv('aes-256-gcm',Buffer.from(browserFile.key),Buffer.from(browserFile.iv));
 decipher.setAuthTag(wire.subarray(-16));assert.equal(Buffer.concat([decipher.update(wire.subarray(0,-16)),decipher.final()]).toString(),'browser attachment fixture');
 console.log('OMEMO: independent devices, multi-device delivery, bidirectional ratchet, out-of-order delivery, persistent history, tamper rejection and trust blocking — OK');
}finally{await browser.close();await new Promise(r=>server.close(r))}
