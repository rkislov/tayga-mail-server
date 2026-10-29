import {chromium} from 'playwright-core';
import {createServer} from 'node:http';
import fs from 'node:fs';
import assert from 'node:assert/strict';

// Exercise the actual token refresh/API wrapper, not a mock that always uses
// the original account. All credentials, keys and publications are fixtures.
const app=fs.readFileSync('frontend/src/app.js','utf8');
const authHelpers=app.slice(app.indexOf('let tokenRefreshPromise = null;'),app.indexOf('function b64urlToBuf'));
const assets={'/assets/libomemo.js':'internal/frontend/dist/assets/libomemo.js','/omemo.js':'frontend/src/omemo.js'};
const server=createServer((req,res)=>{const file=assets[req.url];res.setHeader('Content-Type',file?'text/javascript':'text/html');res.end(file?fs.readFileSync(file):'<!doctype html><title>OMEMO account isolation</title>')});
await new Promise(r=>server.listen(0,'127.0.0.1',r));
const base='http://127.0.0.1:'+server.address().port;
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 async function fixture(){
  const context=await browser.newContext(),page=await context.newPage(),errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.goto(base);
  await page.addScriptTag({content:`
   const state={email:'alice@example.com',tokens:{access_token:'alice-fixture',refresh_token:'alice-refresh-fixture'}};
   const filesState={};const chatBare=s=>String(s||'').split('/')[0].toLowerCase();
   const isAuthPublicPath=()=>false,t=s=>s;
   const forceLogin=()=>{if(typeof omemoCancelOperations==='function')omemoCancelOperations();state.tokens=null};
   const requests=[],nativeFetch=window.fetch.bind(window);let scenario='normal';
   function switchAccount(){state.email='bob@example.com';state.tokens={access_token:'bob-fixture',refresh_token:'bob-refresh-fixture'}}
   window.fetch=async(path,opts={})=>{
    if(!String(path).startsWith('/api/'))return nativeFetch(path,opts);
    requests.push({path,method:opts.method||'GET',authorization:opts.headers?.Authorization||''});
    if(path==='/api/v1/auth/token'){
     if(scenario==='refresh-switch')switchAccount();
     return new Response(JSON.stringify({access_token:'alice-refreshed-fixture',refresh_token:'alice-new-refresh-fixture'}));
    }
    if(opts.method==='PUT'&&scenario==='publish-switch')switchAccount();
    if(opts.method==='PUT'&&scenario==='refresh-switch')return new Response('{}',{status:401});
    if(opts.method==='PUT'&&scenario==='publish-logout'){
     forceLogin();state.tokens={access_token:'alice-new-login-fixture',refresh_token:'alice-new-login-refresh-fixture'};
    }
    return new Response(JSON.stringify(opts.method?{ok:true}:{items:[]}));
   };`+authHelpers});
  await page.addScriptTag({url:base+'/omemo.js'});
  assert.deepEqual(errors,[]);
  return {context,page};
 }
 for(const scenario of ['publish-switch','refresh-switch','publish-logout']){
  const {context,page}=await fixture();
  const result=await page.evaluate(async mode=>{
   scenario=mode;let blocked=false,error='';
   try{await omemoLocked(omemoInitialize)}catch(e){blocked=true;error=e.message}
   return {blocked,error,requests,email:state.email,token:state.tokens?.access_token,
    otherAccount:!!await omemoRead('accounts','bob@example.com')};
  },scenario);
  assert(result.requests.some(r=>r.method==='PUT'),scenario+': no key publication attempted: '+result.error);
  assert(result.blocked,scenario+': old operation was not cancelled');
  assert(!result.requests.some(r=>r.authorization==='Bearer bob-fixture'||r.authorization==='Bearer alice-new-login-fixture'),scenario+': operation crossed login boundary');
  assert(!result.otherAccount,scenario+': key store crossed account boundary');
  if(scenario==='refresh-switch')assert.equal(result.token,'bob-fixture','late refresh overwrote the new login');
  await context.close();
 }
 // An operation queued behind another tab must validate again when the lock
 // is granted; capturing the email before waiting is insufficient.
 {
  const {context,page}=await fixture();
  const result=await page.evaluate(async()=>{
   let release,entered;const ready=new Promise(r=>entered=r);
   const held=navigator.locks.request('tayga-omemo:alice@example.com',()=>{entered();return new Promise(r=>release=r)});
   await ready;const pending=omemoLocked(omemoInitialize).then(()=>false,()=>true);
   switchAccount();release();await held;
   return {blocked:await pending,requests:requests.length};
  });
  assert(result.blocked);assert.equal(result.requests,0);
  await context.close();
 }
 // A legitimate access-token refresh in the same login must still work.
 {
  const {context,page}=await fixture();
  const result=await page.evaluate(async()=>{
   const original=fetch;let unauthorized=true;
   window.fetch=async(path,opts)=>{if(opts?.method==='PUT'&&unauthorized){unauthorized=false;return new Response('{}',{status:401})}return original(path,opts)};
   await omemoLocked(omemoInitialize);
   return {token:state.tokens.access_token,published:requests.filter(r=>r.method==='PUT').length};
  });
  assert.equal(result.token,'alice-refreshed-fixture');assert.equal(result.published,2);
  await context.close();
 }
 // A new login must not join the old account's in-flight token refresh.
 {
  const {context,page}=await fixture();
  const result=await page.evaluate(async()=>{
   const replies=[];window.fetch=()=>new Promise(resolve=>replies.push(resolve));
   const old=refreshAccessToken();switchAccount();const current=refreshAccessToken();
   const count=replies.length;
   replies[0](new Response(JSON.stringify({access_token:'obsolete',refresh_token:'obsolete-refresh'})));
   if(count===2)replies[1](new Response(JSON.stringify({access_token:'bob-current',refresh_token:'bob-current-refresh'})));
   return {count,old:await old,current:await current,token:state.tokens.access_token};
  });
  assert.deepEqual(result,{count:2,old:false,current:true,token:'bob-current'});
  await context.close();
 }
 // Do not return cached plaintext after the account changes during IDB I/O.
 {
  const {context,page}=await fixture();
  const result=await page.evaluate(async()=>{
   await omemoWrite(state.email,{id:42,store:{},trust:{},enabled:{}},{key:'sender@example.com|cached',body:'private cached fixture'});
   const read=omemoRead;
   omemoRead=async(store,key)=>{const result=await read(store,key);if(store==='messages')switchAccount();return result};
   try{await omemoDecrypt({from:'sender@example.com',id:'cached'});return false}catch{return true}
  });
  assert(result,'cached plaintext escaped its original login');
  await context.close();
 }
 // Encrypted-file requests strip the secret fragment and browser credentials;
 // a late response cannot open the old account's document in the new session.
 for(const changeAccount of [false,true]){
  const {context,page}=await fixture();
  const result=await page.evaluate(async changeAccount=>{
   const encrypted=await omemoEncryptFile(new File(['private attachment fixture'],'test.pdf'));
   const body=omemoFileURI('https://files.example.com/test.pdf',encrypted);
   let opened=0,request;
   window.fetch=async(url,opts)=>{request={url,...opts};if(changeAccount)switchAccount();return new Response(encrypted.blob)};
   window.boundedPreviewBlob=async r=>r.blob();
   window.openDocumentPreview=async({blob})=>{if(await blob.text()!=='private attachment fixture')throw Error('wrong plaintext');opened++};
   const element=document.createElement('div');element.innerHTML='<p></p>';document.body.append(element);
   omemoAttachmentBubble({body},element);await element.querySelector('button').onclick();
   return {opened,request,error:element.textContent.includes('Account changed')};
  },changeAccount);
  assert.equal(result.opened,changeAccount?0:1);
  assert.equal(result.request.url,'https://files.example.com/test.pdf');
  assert.equal(result.request.credentials,'omit');assert.equal(result.request.referrerPolicy,'no-referrer');
  assert.equal(result.request.headers,undefined);
  if(changeAccount)assert(result.error);
  await context.close();
 }
 console.log('OMEMO account isolation: queued operations, key publication, logout/relogin, token refresh, cached plaintext and encrypted previews — OK');
}finally{await browser.close();await new Promise(r=>server.close(r))}
