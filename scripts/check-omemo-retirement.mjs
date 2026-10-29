import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';

// Real API, real IndexedDB and ratchets; ONLY a disposable local seeded server.
const base=process.env.TMS_URL||'http://127.0.0.1:18090';
assert(['127.0.0.1','localhost','[::1]'].includes(new URL(base).hostname),'Use a disposable local server');
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const errors=[];
 async function newBrowser(){
  const context=await browser.newContext({viewport:{width:1000,height:900}}),page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
  await page.goto(base);await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForSelector('#view-account:not(.hidden)');
  await page.evaluate(async()=>{showApp('chat');chatState.peer=state.email;await renderOMEMOButton();await omemoSettings()});
  await page.getByRole('button',{name:'Создать ключ для этого браузера',exact:true}).waitFor();
  assert.equal(await page.evaluate(()=>omemoLocked(c=>c.data.id||0)),0,'Opening settings silently created a key');
  return page;
 }
 async function connect(page){
  await page.getByRole('button',{name:'Создать ключ для этого браузера',exact:true}).click();await page.locator('[data-status=current]').waitFor();
  const id=await page.evaluate(()=>omemoLocked(c=>c.data.id));await page.getByRole('button',{name:'Закрыть',exact:true}).click();return id;
 }
 async function trust(page){await page.evaluate(()=>omemoLocked(async c=>{
  for(const id of await omemoDevices(c.account)){if(id===c.data.id)continue;const bundle=await omemoBundle(c.account,id),address=new c.lib.OMEMOAddress(c.account,id).toString();c.data.trust[address]=omemoB64(bundle.identityKey);c.store.saveIdentity(address,bundle.identityKey)}c.data.enabled[c.account]=true;await c.save();
 }))}
 const first=await newBrowser();const firstID=await connect(first);
 const lost=await newBrowser();const lostID=await connect(lost);
 for(const page of [first,lost])await trust(page);
 const original=await first.evaluate(()=>omemoSend(state.email,'Cached before retirement'));
 assert.equal(await lost.evaluate(ev=>omemoDecrypt(ev),original),'Cached before retirement');
 await first.evaluate(()=>omemoSettings());
 assert.equal(await first.locator(`.omemo-device[data-device-id="${firstID}"] [data-omemo-action=retire]`).count(),0);
 const row=first.locator(`.omemo-device[data-device-id="${lostID}"]`);
 await row.locator(':scope > .omemo-device-details > summary').click();
 await row.locator('.omemo-device-retirement > summary').click();
 await row.getByRole('button',{name:'Отозвать этот ключ на сервере',exact:true}).click();await row.locator('[data-status=retired]').waitFor();
 assert.equal(await row.locator('[data-omemo-action=verify]').count(),0,'Retired key could be restored');
 assert(!await first.evaluate(id=>omemoDevices(state.email).then(ids=>ids.includes(id)),lostID));
 await assert.rejects(()=>lost.evaluate(()=>omemoLocked(omemoInitialize)),/retired|отозван/);
 assert.equal(await lost.evaluate(ev=>omemoDecrypt(ev),original),'Cached before retirement');
 await lost.evaluate(()=>omemoSettings());await lost.locator(`.omemo-device[data-device-id="${lostID}"] [data-status=retired]`).waitFor();
 await lost.getByText('Ключ этого браузера отозван на сервере.',{exact:false}).first().waitFor();
 await assert.rejects(()=>first.evaluate(()=>omemoSend(state.email,'No remaining recipient')),/Нет доступных устройств|No recipient/);
 const replacement=await newBrowser();
 await replacement.setViewportSize({width:390,height:844});
 await replacement.getByText('Если данные прежнего браузера потеряны',{exact:false}).waitFor();
 assert(await replacement.locator('.omemo-body').evaluate(e=>e.scrollWidth<=e.clientWidth+1));
 await replacement.screenshot({path:'/private/tmp/tayga-omemo-recovery-mobile.png',animations:'disabled'});
 const replacementID=await connect(replacement);assert.notEqual(replacementID,lostID);
 await assert.rejects(()=>replacement.evaluate(ev=>omemoDecrypt(ev),original),/Not encrypted for this device/);
 await assert.rejects(()=>first.evaluate(()=>omemoSend(state.email,'Must verify replacement')),/Verify device key/);
 for(const page of [first,replacement])await trust(page);
 const fresh=await first.evaluate(()=>omemoSend(state.email,'Verified replacement'));
 assert.equal(await replacement.evaluate(ev=>omemoDecrypt(ev),fresh),'Verified replacement');
 assert(!fresh.omemo.includes(`rid="${lostID}"`));
 await assert.rejects(()=>lost.evaluate(ev=>omemoDecrypt(ev),fresh),/Not encrypted for this device/);
 await first.getByRole('button',{name:'Обновить устройства',exact:true}).click();await row.locator('[data-status=retired]').waitFor();
 await first.screenshot({path:'/private/tmp/tayga-omemo-retired-desktop.png',animations:'disabled'});
 assert.deepEqual(errors,[]);
 console.log('OMEMO retirement: explicit onboarding, account-wide retirement, rejected republication, no silent key reset, cached history, replacement requires verification and new messages only reach active devices — OK');
}finally{await browser.close()}
