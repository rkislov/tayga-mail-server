import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';

// Real API on a disposable seeded loopback server only. Never use production.
const base=process.env.TMS_URL||'http://127.0.0.1:18090';
assert(['127.0.0.1','localhost','[::1]'].includes(new URL(base).hostname));
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const context=await browser.newContext({viewport:{width:1280,height:1000}}),page=await context.newPage(),errors=[];
 page.on('pageerror',error=>errors.push(error.message));
 await page.goto(base);await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForSelector('#view-account:not(.hidden)');
 await page.evaluate(()=>showApp('profile'));
 const card=page.locator('#xmpp-devices-card');await card.getByRole('button',{name:'Создать пароль устройства',exact:true}).waitFor();
 await card.getByRole('button',{name:'Создать пароль устройства',exact:true}).click();
 let modal=page.locator('.xmpp-device-dialog');await modal.getByLabel('Название устройства').fill('Monal · iPhone');await modal.getByRole('button',{name:'Создать пароль',exact:true}).click();
 await modal.locator('.xmpp-device-secret').waitFor();
 const secret=await modal.locator('.xmpp-device-secret').inputValue();assert(secret.startsWith('tayga-xmpp.'));
 assert.equal(await page.evaluate(secret=>JSON.stringify(localStorage).includes(secret)||JSON.stringify(sessionStorage).includes(secret),secret),false);
 await modal.getByRole('button',{name:'Готово',exact:true}).click();await page.locator('.xmpp-device-secret').waitFor({state:'detached'});
 const listed=await page.evaluate(()=>api('/api/v1/me/xmpp-devices'));assert(!JSON.stringify(listed).includes(secret));assert(!JSON.stringify(listed).includes('secret_hash'));
 // Server and UI both keep migration opt-in and require a tested connection.
 assert.equal(listed.devices_only,false);
 page.once('dialog',dialog=>dialog.accept());await card.getByLabel('Только пароли устройств для XMPP',{exact:true}).click();
 await card.getByText('Сначала подключите хотя бы один клиент с новым паролем устройства.',{exact:true}).waitFor();
 assert.equal(await card.getByLabel('Только пароли устройств для XMPP',{exact:true}).isChecked(),false);
 await page.evaluate(async()=>{for(let i=0;i<9;i++)await api('/api/v1/me/xmpp-devices',{method:'POST',body:JSON.stringify({name:`Browser fixture ${i}`})});await refreshXMPPDevices()});
 const list=card.locator('.xmpp-device-list').first();assert.equal(await list.locator('tbody tr').count(),8);
 await list.getByRole('button',{name:'Далее',exact:true}).click();assert.equal(await list.locator('tbody tr').count(),2);
 await card.getByRole('searchbox').fill('Monal');assert.equal(await list.locator('tbody tr').count(),1);
 page.once('dialog',dialog=>dialog.accept());await list.getByRole('button',{name:'Отозвать',exact:true}).click();
 await card.getByText('Отозван',{exact:true}).waitFor();
 assert(await page.evaluate(()=>api('/api/v1/me/xmpp-devices').then(data=>data.devices.some(d=>d.name==='Monal · iPhone'&&d.revoked_at))));
 await card.evaluate(e=>e.scrollIntoView({block:'start'}));await page.screenshot({path:'/private/tmp/tayga-xmpp-devices-desktop.png'});
 await page.setViewportSize({width:390,height:844});await card.evaluate(e=>e.scrollIntoView({block:'start'}));
 assert(await card.evaluate(e=>e.scrollWidth<=e.clientWidth+1),'Card overflows mobile viewport');
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'Page overflows');
 await page.screenshot({path:'/private/tmp/tayga-xmpp-devices-mobile.png'});
 // Closing a login/account clears the one-time secret synchronously.
 await card.getByRole('button',{name:'Создать пароль устройства',exact:true}).click();modal=page.locator('.xmpp-device-dialog');
 await page.screenshot({path:'/private/tmp/tayga-xmpp-device-modal-mobile.png'});
 await modal.getByLabel('Название устройства').fill('Logout fixture');await modal.getByRole('button',{name:'Создать пароль',exact:true}).click();await modal.locator('.xmpp-device-secret').waitFor();
 await page.evaluate(()=>forceLogin('Fixture logout'));assert.equal(await page.locator('.xmpp-device-dialog').count(),0);
 assert.deepEqual(errors,[]);
 console.log('XMPP device UI: one-time secret, no persistence/disclosure, guarded policy, search/pagination, revoke, mobile layout and logout cleanup — OK');
} finally {await browser.close()}
