import fs from 'node:fs';
import assert from 'node:assert/strict';
import { chromium } from 'playwright-core';
const app=fs.readFileSync('frontend/src/app.js','utf8');
const images=fs.readFileSync('frontend/src/mail-images.js','utf8');
const design=fs.readFileSync('frontend/src/design-pack.js','utf8');
const markup=fs.readFileSync('internal/frontend/dist/index.html','utf8');
const compose=markup.slice(markup.indexOf('<div id="compose-backdrop"'),markup.indexOf('<!-- CALENDAR -->'));
const readerStart=markup.indexOf('<div class="mail-read panel-pane">');
const reader=markup.slice(readerStart,markup.indexOf('<div id="compose-backdrop"')).replace(/\s*<\/div>\s*$/, '');
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage();
 await page.route('https://test.local/**',route=>route.fulfill({contentType:'text/html; charset=utf-8',body:'<!doctype html><html lang="ru" data-color-mode="light"><head><meta charset="utf-8"></head><body><main id="reader-fixture">'+reader+'</main>'+compose+'</body></html>'}));
 await page.goto('https://test.local/');
 await page.addStyleTag({path:'internal/frontend/dist/assets/app.css'});
 // Keep the real reader markup/styles; only size the isolated screenshot host.
 await page.addStyleTag({content:'body{margin:0}#reader-fixture{max-width:1080px;margin:20px auto;padding:0 12px}#reader-fixture .mail-read{height:auto}'});
 await page.addScriptTag({content:`
 const $=id=>document.getElementById(id), state={me:{id:'owner'}}, signatureState={text:'',replies:false,new_messages:false},mailState={mailboxes:[]};
 const WEB_UPLOAD_BYTES=10*1024**3;
 const t=s=>s,mailImageText=(ru,en)=>ru,fmtBytes=n=>n<1024?n+' B':(n/1024).toFixed(1)+' KiB';
 function setMsg(el,text){if(el)el.textContent=text};function refreshMail(){};
 function setMobilePane(){};function renderSenderAvatar(){};function mailDateFull(s){return s};
 const mailImagePreferences={mode:'always',senders:[]};
 let hold=null,fail=false,sends=[],shares=0;
 async function api(path,options){
   if(path.endsWith('/prepare')) {if(hold)await hold;if(fail)throw Error('Preparation failed');return {name:'report.pdf',path:'mail-out/copy/report.pdf',etag:'v1',size:100};}
   if(path.endsWith('/senders'))return {addresses:['owner@example.com'],default:'owner@example.com'};
   if(path.endsWith('/limits'))return {message_bytes:1000000,large_attachment_bytes:500,attachment_bytes:10000,share_ttl_hours:24};
   if(path.endsWith('/shares')){shares++;return {url:'https://test.local/s/file',expires_at:'2026-10-20T00:00:00Z'};}
   if(path.endsWith('/send')){sends.push(JSON.parse(options.body));return {ok:true};}
   throw Error('Unexpected API '+path);
 }
 `+design.slice(design.indexOf('function designIcon('),design.indexOf('\nfunction documentPreviewType'))+images.slice(images.indexOf('function ensureComposeCC('),images.indexOf('\nfunction renderMailQuickFilters('))+app.slice(app.indexOf('function composeBodyEl('),app.indexOf('\n$("btn-mail-delete")'))+app.slice(app.indexOf('function showMailReader('),app.indexOf('\nasync function openMailMessage'))+fs.readFileSync('frontend/src/mail-attachments.js','utf8')});
 const message={id:'one',from:'Sender <sender@example.com>',to:'Me <owner@example.com>',cc:'cc@example.com',subject:'Тема письма',date:'2026-10-19',html:'<p>First line</p><script>window.compromised=true</script><p>Second line</p>',attachments:[{id:'0',filename:'report.pdf',size:100}]};
 const preview={...message,from:'Анна Смирнова <anna@example.com>',to:'Роман <roman@example.com>',subject:'Материалы к встрече',date:'19 октября 2026 г., 14:55',attachments:[{id:'0',filename:'Программа встречи.pdf',size:148000}],html:'<h2>Здравствуйте, Роман!</h2><p>Направляю программу встречи и материалы для обсуждения.</p><p>Документ приложен к письму. Его можно переслать коллегам вместе с этим сообщением.</p><table style="width:100%;border-collapse:collapse"><tr><th style="text-align:left;padding:12px;background:#edf0ff">Время</th><th style="text-align:left;padding:12px;background:#edf0ff">Тема</th></tr><tr><td style="padding:12px;border-bottom:1px solid #e0e4ed">10:00</td><td style="padding:12px;border-bottom:1px solid #e0e4ed">Обсуждение проекта</td></tr><tr><td style="padding:12px;border-bottom:1px solid #e0e4ed">10:30</td><td style="padding:12px;border-bottom:1px solid #e0e4ed">Следующие шаги</td></tr></table><p>С уважением,<br>Анна Смирнова</p>'};
 await page.evaluate(msg=>showMailReader(msg),preview);
 for(const mode of ['light','dark']) for(const width of [390,1440]) {
   await page.evaluate(mode=>document.documentElement.dataset.colorMode=mode,mode);
   await page.setViewportSize({width,height:900});
   const box=await page.locator('[data-mail-action="forward"]').boundingBox();assert(box&&box.x>=0&&box.x+box.width<=width);
   assert(await page.locator('#mail-subject').isVisible());assert(await page.locator('.mail-attachments button').isVisible());
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
   await page.frameLocator('.mail-content-frame').getByText('Здравствуйте, Роман!').waitFor();
   await page.screenshot({path:'/tmp/tayga-forward-reader-'+mode+'-'+width+'.png',fullPage:true});
 }
 await page.evaluate(msg=>{window.message=msg;showMailReader(msg);hold=new Promise(resolve=>window.release=resolve)},message);
 await page.getByRole('button',{name:'Переслать',exact:true}).click();
 assert.equal(await page.locator('#compose-to').inputValue(),'');assert.equal(await page.locator('#compose-cc').inputValue(),'');
 assert.equal(await page.locator('#compose-subject').inputValue(),'Fwd: Тема письма');
 assert(await page.locator('#compose-body').innerText().then(s=>s.includes('First line')&&s.includes('Second line')&&s.includes('sender@example.com')));
 assert.equal(await page.evaluate(()=>!!window.compromised),false);
 assert.equal(await page.locator('#btn-compose-send').isDisabled(),true);
 await page.evaluate(()=>release());await page.evaluate(()=>composeAttachments.queue);
 assert.equal(await page.locator('#btn-compose-send').isDisabled(),false);
 await page.locator('#compose-to').fill('recipient@example.com');await page.locator('#btn-compose-send').click();
 const sent=await page.evaluate(()=>sends);assert.equal(sent.length,1);assert.deepEqual(sent[0].to,['recipient@example.com']);assert.deepEqual(sent[0].cc,[]);assert.equal(sent[0].attachments[0].path,'mail-out/copy/report.pdf');
 // A failed preparation blocks Send until a retry succeeds or the file is removed.
 await page.evaluate(()=>{fail=true;hold=null;forwardMailMessage(message)});await page.evaluate(()=>composeAttachments.queue);
 assert.equal(await page.locator('#btn-compose-send').isDisabled(),true);
 await page.evaluate(()=>fail=false);await page.getByRole('button',{name:'Повторить',exact:true}).click();await page.evaluate(()=>composeAttachments.queue);
 assert.equal(await page.locator('#btn-compose-send').isDisabled(),false);
 // Large forwarded files use the existing link flow and cannot be reattached as raw bytes.
 await page.evaluate(async()=>{const item=composeAttachments.entries[0];item.file.size=1000;await prepareComposeAttachments()});
 assert.equal(await page.locator('#compose-body a').getAttribute('href'),'https://test.local/s/file');
 await page.locator('#btn-compose-send').click();assert.equal(await page.evaluate(()=>sends[1].attachments.length),0);
 // Closing a reader dialog opens the same forwarding flow; existing Fwd stays single.
 await page.evaluate(()=>{const dialog=document.createElement('dialog');document.body.append(dialog);dialog.showModal();window.testDialog=dialog;forwardMailMessage({...message,subject:'Fwd: Existing'},dialog)});
 assert.equal(await page.evaluate(()=>testDialog.open),false);assert.equal(await page.locator('#compose-subject').inputValue(),'Fwd: Existing');await page.evaluate(()=>composeAttachments.queue);
 // Late attachment responses cannot modify a different draft or account.
 await page.evaluate(()=>{hold=new Promise(resolve=>window.release=resolve);forwardMailMessage(message)});
 await page.waitForTimeout(30);await page.evaluate(()=>openCompose({subject:'New draft'}));await page.evaluate(()=>release());await page.waitForTimeout(50);
 assert.equal(await page.evaluate(()=>composeAttachments.entries.length),0);
 assert.equal(await page.locator('#compose-subject').inputValue(),'New draft');
 await page.evaluate(()=>{hold=new Promise(resolve=>window.release=resolve);forwardMailMessage(message)});await page.waitForTimeout(30);await page.evaluate(()=>{state.me.id='other';release()});await page.evaluate(()=>composeAttachments.queue);
 assert.equal(await page.evaluate(()=>composeAttachments.entries[0].ref),undefined);
 console.log('PASS: forward pane/modal, empty recipients, text safety, attachment preparation/retry, large-file link, stale draft/account, 390/1440 action bar');
} finally { await browser.close(); }
