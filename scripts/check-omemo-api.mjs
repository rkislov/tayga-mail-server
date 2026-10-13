import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const pages=[],errors=[];
 for(let i=0;i<2;i++){
  const context=await browser.newContext({viewport:{width:1280,height:900}}),page=await context.newPage();pages.push(page);page.on('pageerror',e=>errors.push(e.message));
  await page.goto(process.env.TMS_URL||'http://127.0.0.1:18090');await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForSelector('#view-account:not(.hidden)');
  await page.evaluate(async()=>{showApp('chat');await omemoLocked(omemoInitialize);chatState.peer=state.email;await renderOMEMOButton()});
 }
 for(const page of pages)await page.evaluate(()=>omemoLocked(async c=>{for(const id of await omemoDevices(c.account)){if(id===c.data.id)continue;const bundle=await omemoBundle(c.account,id),address=new c.lib.OMEMOAddress(c.account,id).toString();c.data.trust[address]=omemoB64(bundle.identityKey);c.store.saveIdentity(address,bundle.identityKey)}c.data.enabled[c.account]=true;await c.save()}));
 const sent=await pages[0].evaluate(async()=>{const ev=await omemoSend(state.email,'Browser API encrypted test');appendChatBubble(ev);return ev});
 const plain=await pages[1].evaluate(m=>omemoDecrypt(m),sent);assert.equal(plain,'Browser API encrypted test');
 const count=await pages[0].evaluate(id=>Array.from(document.querySelectorAll('#chat-messages .chat-bubble')).filter(x=>x.dataset.messageKey===JSON.stringify([state.email,id])).length,sent.id);assert.equal(count,1);
 await pages[0].evaluate(()=>omemoSettings());await pages[0].getByRole('button',{name:'Отключить OMEMO для диалога'}).waitFor();await pages[0].screenshot({path:'docs/screenshots/ui-web-omemo.png'});
 assert.deepEqual(errors,[]);console.log('Actual HTTP API: key publication, independent browser devices, ciphertext delivery, decrypt, outgoing dedup and settings UI — OK');
}finally{await browser.close()}
