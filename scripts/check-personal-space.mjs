// Browser regression check and real screenshots against a disposable local installation.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import {chromium} from 'playwright-core';
const base=process.env.TMS_URL||'http://127.0.0.1:18080';
const capture=process.env.TMS_CAPTURE_SCREENSHOTS==='1';
const output='docs/screenshots/v1.0.0';
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:1440,height:960}}),errors=[];
 page.on('pageerror',e=>errors.push(e.message));
 await page.goto(base);await page.evaluate(()=>{localStorage.clear();localStorage.setItem('tayga.lang','ru');localStorage.setItem('tayga.theme','tayga');localStorage.setItem('tayga.colorMode','light')});await page.reload();
 if(capture){await fs.mkdir(output,{recursive:true});await page.screenshot({path:output+'/login.png'});}
 await page.fill('#email',process.env.TMS_EMAIL||'admin@example.com');await page.fill('#password',process.env.TMS_PASSWORD||'changeme');await page.click('#form-login button[type=submit]');await page.waitForFunction(()=>state.me?.id);await page.waitForSelector('#workspace-compose');await page.waitForSelector('#mail-pagination select');assert.equal(await page.locator('#mail-pagination select').inputValue(),'25','default mail page size');
 await page.locator('#sidebar-collapse').click();assert.equal(await page.locator('#app-nav').evaluate(e=>Math.round(e.getBoundingClientRect().width)),64);
 await page.locator('.nav-btn[data-app=calendar]').focus();await page.waitForSelector('#sidebar-tooltip:not([hidden])');assert.match(await page.locator('#sidebar-tooltip').innerText(),/Календарь/);
 await page.reload();await page.waitForFunction(()=>state.me?.id);assert.equal(await page.locator('#view-account').evaluate(el=>el.classList.contains('sidebar-collapsed')),true);await page.locator('#sidebar-collapse').click();
 await page.locator('#workspace-admin-entry').click();await page.waitForFunction(()=>document.getElementById('view-account').dataset.workspaceArea==='admin');assert.equal(await page.locator('#workspace-apps-entry').isVisible(),true);await page.locator('#workspace-apps-entry').click();await page.waitForFunction(()=>document.getElementById('view-account').dataset.workspaceArea==='apps');
 await page.evaluate(()=>{showApp('appearance');applyLang('en')});assert.equal(await page.locator('#app-nav .nav-btn[data-app=calendar]').innerText(),'Calendar');await page.evaluate(()=>applyLang('ru'));
 await page.locator('[data-density=compact]').click();assert.equal(await page.evaluate(()=>document.documentElement.dataset.density),'compact');await page.locator('[data-density=comfortable]').click();await page.locator('[data-accent=amber]').click();assert.equal(await page.evaluate(()=>localStorage.getItem('tayga.accent')),'amber');await page.reload();await page.waitForFunction(()=>state.me?.id);assert.equal(await page.evaluate(()=>document.documentElement.style.getPropertyValue('--work-primary')),'#b9873b');await page.evaluate(()=>showApp('appearance'));await page.locator('[data-accent=pine]').click();
 const favicon=await page.request.get(base+'/assets/favicon.svg?v=1.0.0');assert.equal(favicon.status(),200);assert.match(await favicon.text(),/id="tayga-tree"/);assert.match(await page.locator('.workspace-logo use').getAttribute('href'),/tayga-tree\.svg/);
 const apps=await page.evaluate(()=>APPS.filter(app=>!['security','xmpp'].includes(app)&&document.getElementById('app-'+app)));
 const overflows=[];
 for(const width of [1920,1440,1200,1024,768,390]){
  await page.setViewportSize({width,height:960});
  for(const app of apps){await page.evaluate(a=>showApp(a),app);await page.waitForTimeout(70);
   const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1);
   if(overflow)overflows.push({width,app,elements:await page.evaluate(()=>[...document.querySelectorAll('.workspace-main *')].filter(e=>e.getClientRects().length&&e.getBoundingClientRect().right>innerWidth+1).slice(0,5).map(e=>e.id||e.className))});
   assert.equal(await page.locator('.create-fab:visible').count(),0,'floating create action');
  }
 }
 assert.deepEqual(overflows,[],'horizontal overflow');
 await page.setViewportSize({width:390,height:844});await page.evaluate(()=>showApp('mail'));await page.click('#btn-nav-toggle');assert.equal(await page.locator('#app-nav').getAttribute('aria-hidden'),'false');assert.equal(await page.locator('#app-nav').evaluate(e=>e.inert),false);if(capture)await page.screenshot({path:output+'/mobile-navigation.png'});await page.keyboard.press('Escape');assert.equal(await page.locator('#app-nav').getAttribute('aria-hidden'),'true');assert.equal(await page.locator('#app-nav').evaluate(e=>e.inert),true);
 if(capture){
  await page.screenshot({path:output+'/mobile-mail.png'});await page.setViewportSize({width:1440,height:960});
  for(const app of apps){await page.evaluate(a=>showApp(a),app);await page.waitForTimeout(450);
   await page.evaluate(async a=>{if(a==='mail'){await refreshMail();const id=document.querySelector('#mail-msg-list [data-msg]')?.dataset.msg;if(id)await openMailMessage(id)}if(a==='calendar'){await refreshCalendar();if(calState.events[0])await openCalEvent(calState.events[0].id)}if(a==='files'){await refreshFiles();const entry=filesState.entries.find(e=>!e.is_dir&&e.name.endsWith('.pdf'));if(entry)await selectFileEntry(entry.name)}if(a==='contacts'){await refreshContacts();const id=document.querySelector('[data-card]')?.dataset.card;if(id)await openContactCard(id)}if(a==='notes'){await refreshNotes();const id=document.querySelector('#note-list [data-note]')?.dataset.note;if(id)await openNote(id)}if(a==='chat'){await mmChat.refresh?.();document.querySelector('#mm-chat-channels .mm-chat-channel')?.click()}},app);
   if(app==='chat')await page.waitForFunction(()=>document.getElementById('mm-chat-connection').textContent==='Подключено',null,{timeout:10000});await page.evaluate(()=>document.fonts.ready);await page.waitForTimeout(200);await page.screenshot({path:output+'/'+app+'.png'});
  }
  await page.evaluate(async()=>{showApp('tenants');await refreshTenantNav();document.querySelector('[data-directory-row]')?.click()});await page.waitForSelector('#admin-user-list tr td');await page.waitForTimeout(350);await page.screenshot({path:output+'/users.png'});
  await page.evaluate(()=>refreshTenantDomains());await page.waitForTimeout(350);await page.screenshot({path:output+'/domains.png'});
  await page.evaluate(async()=>{showApp('server');await refreshSettings();await openAdminConfig('smtp')});await page.waitForTimeout(250);await page.screenshot({path:output+'/server-smtp.png'});await page.evaluate(()=>closeAdminConfig());
  await page.evaluate(async()=>{showApp('mail');await refreshMail();const id=document.querySelector('#mail-msg-list [data-msg]')?.dataset.msg;if(id)await openMailMessage(id)});await page.locator('#sidebar-collapse').click();await page.screenshot({path:output+'/mail-compact.png'});await page.locator('#sidebar-collapse').click();
  await page.evaluate(()=>openCompose({to:'anna@example.com',subject:'Обсуждение интерфейса',bodyHTML:'<p>Анна, привет!</p><p>Спасибо за материалы. Давай обсудим новый интерфейс на встрече в четверг.</p><p>С уважением,<br>Роман</p>'}));await page.waitForTimeout(1000);await page.screenshot({path:output+'/compose.png'});await page.keyboard.press('Escape');
  await page.evaluate(()=>{document.getElementById('compose-backdrop').classList.add('hidden');applyColorMode('dark');showApp('mail')});await page.waitForTimeout(200);await page.screenshot({path:output+'/mail-dark.png'});
 }
 assert.deepEqual(errors,[],'browser exceptions');console.log('Personal space: all apps at six sizes, persistent icon rail, keyboard tooltip, admin navigation, mobile drawer/inert/Escape and no floating actions — OK');
}finally{await browser.close()}
