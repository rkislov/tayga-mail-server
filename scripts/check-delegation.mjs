import {chromium} from 'playwright-core';
import fs from 'node:fs';
import assert from 'node:assert/strict';
const app=fs.readFileSync('frontend/src/app.js','utf8'),images=fs.readFileSync('frontend/src/mail-images.js','utf8'),html=fs.readFileSync('internal/frontend/dist/index.html','utf8');
const compose=html.slice(html.indexOf('<div id="compose-backdrop"'),html.indexOf('<!-- CALENDAR -->'));
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:1100,height:900}}),errors=[];page.on('pageerror',error=>errors.push(error.message));
 await page.route('http://localhost/**',route=>route.fulfill({contentType:'text/html; charset=utf-8',body:'<!doctype html><html lang="ru" data-color-mode="light"><body>'+compose+'</body></html>'}));await page.goto('http://localhost');await page.addStyleTag({path:'internal/frontend/dist/assets/app.css'});
 await page.addScriptTag({content:`
 const $=id=>document.getElementById(id),state={me:{id:'owner',email:'owner@example.com'}},signatureState={text:'',replies:false,new_messages:false};
 const t=s=>s,mailImageText=ru=>ru,setMsg=(el,text)=>{if(el)el.textContent=text},refreshMail=()=>{};
 let identities=null,delegates=[],delegateRequest=null;let requests=[],hold=null,fail=false,addresses=['owner@example.com','sales@example.com','owner@alias.example','sales@alias.example'];
 async function api(path,options){
  if(path.endsWith('/senders')){const result={addresses:[...addresses],default:state.me.email,...(identities?{identities}: {})};if(hold)await hold;return result}
  if(path.endsWith('/send')){const payload=JSON.parse(options.body);requests.push(payload);if(fail)throw Error('sender address is not authorized');return{}}
  if(path.startsWith('/api/v1/mail/delegates')){
   if(options?.method==='POST'){delegateRequest=JSON.parse(options.body);delegates=[{...delegateRequest,delegate_id:'secretary'}];return{}}
   if(options?.method==='DELETE'){delegates=[];return{}}
   return{delegates,acting_for:[]};
  }
  throw Error('Unexpected API '+path);
 }
 ${fs.readFileSync('frontend/src/mail-attachments.js','utf8')}
 ${images.slice(images.indexOf('function ensureComposeCC('),images.indexOf('\nfunction forwardMailMessage('))}
 ${app.slice(app.indexOf('function composeBodyEl('),app.indexOf('\n$("btn-mail-delete")'))}
 ${fs.readFileSync('frontend/src/design-pack.js','utf8').split('async function openMailboxAccess')[1].split('const mailboxAccessButton')[0].replace(/^/, 'async function openMailboxAccess')}
 `});
 await page.evaluate(()=>openCompose({to:'recipient@outside.example'}));
 await page.waitForFunction(()=>document.getElementById('compose-from').options.length===4);
 assert.equal(await page.locator('#compose-from').inputValue(),'owner@example.com');
 await page.locator('#compose-from').selectOption('sales@alias.example');await page.locator('#btn-compose-send').click();
 assert.equal(await page.evaluate(()=>requests[0].from),'sales@alias.example');
 assert.equal(await page.locator('#compose-backdrop').evaluate(el=>el.classList.contains('hidden')),true);
 await page.evaluate(()=>replyToMessage({from:'sender@outside.example',reply_from:'sales@alias.example',reply_all_to:['sender@outside.example'],subject:'Alias',text:'Hello'}));
 await page.waitForFunction(()=>document.getElementById('compose-from').value==='sales@alias.example');
 await page.evaluate(()=>fail=true);await page.locator('#btn-compose-send').click();
 await page.waitForFunction(()=>document.getElementById('compose-msg').textContent==='sender address is not authorized');
 assert.equal(await page.locator('#compose-backdrop').evaluate(el=>el.classList.contains('hidden')),false);
 await page.evaluate(()=>{fail=false;hold=new Promise(resolve=>window.release=resolve);openCompose({from:'sales@alias.example'})});
 assert.equal(await page.locator('#btn-compose-send').isDisabled(),true);
 // A late response for an earlier draft cannot replace the current sender list.
 await page.evaluate(()=>{hold=null;addresses=['owner@example.com'];openCompose()});await page.waitForFunction(()=>document.getElementById('compose-from').options.length===1);
 await page.evaluate(()=>release());await page.waitForTimeout(25);
 assert.equal(await page.locator('#compose-from option').count(),1);
 assert.equal(await page.locator('#compose-from').inputValue(),'owner@example.com');

 await page.evaluate(()=>{identities=[{email:'owner@example.com',mode:'send_as'},{email:'boss@example.com',mode:'send_as'},{email:'boss@example.com',mode:'on_behalf'}];openCompose({to:'recipient@outside.example'});});
 await page.waitForFunction(()=>document.getElementById('compose-from').options.length===3);
 await page.locator('#compose-from').selectOption({index:2});
 assert.match(await page.locator('#compose-from option:checked').textContent(),/от имени/);
 await page.locator('#btn-compose-send').click();
 assert.deepEqual(await page.evaluate(()=>({from:requests.at(-1).from,mode:requests.at(-1).sender_mode})),{from:'boss@example.com',mode:'on_behalf'});
 await page.evaluate(()=>openCompose({to:'recipient@outside.example'}));
 await page.waitForFunction(()=>!document.getElementById('compose-from').disabled);
 await page.locator('#compose-from').selectOption({index:1});await page.locator('#btn-compose-send').click();
 assert.equal(await page.evaluate(()=>requests.at(-1).sender_mode),'send_as');
 await page.evaluate(()=>{identities=[{email:'owner@example.com',mode:'send_as'},{email:'boss@example.com',mode:'on_behalf'}];openCompose({to:'recipient@outside.example',from:'boss@example.com'});});
 await page.waitForFunction(()=>!document.getElementById('compose-from').disabled);
 assert.equal(await page.locator('#compose-from option:checked').getAttribute('data-sender-mode'),'on_behalf');
 await page.locator('#btn-compose-send').click();
 await page.evaluate(()=>openMailboxAccess());
 const dialog=page.locator('dialog[aria-label="Делегирование почты и календаря"]');
 await dialog.locator('input[type=email]').fill('secretary@example.com');
 assert.equal(await dialog.locator('input[type=checkbox]').count(),7);
 await dialog.getByLabel('Создавать, переносить и отменять встречи руководителя',{exact:true}).check();
 await dialog.getByLabel('Отвечать на приглашения за руководителя',{exact:true}).check();
 await dialog.locator('button[type=submit]').click();
 await page.waitForFunction(()=>delegateRequest?.can_manage_calendar===true&&delegateRequest?.can_respond_invites===true);
 await dialog.getByRole('button',{name:'Изменить',exact:true}).click();
 assert.equal(await dialog.locator('input[type=email]').inputValue(),'secretary@example.com');
 assert.equal(await dialog.getByLabel('Отвечать на приглашения за руководителя',{exact:true}).isChecked(),true);
 await dialog.getByLabel('Отвечать на приглашения за руководителя',{exact:true}).uncheck();
 await dialog.locator('button[type=submit]').click();
 await page.waitForFunction(()=>delegateRequest?.can_respond_invites===false);
 await dialog.getByRole('button',{name:'Отозвать',exact:true}).click();
 await page.waitForFunction(()=>delegates.length===0);
 await page.setViewportSize({width:390,height:844});
 assert.ok(await dialog.evaluate(el=>el.getBoundingClientRect().right<=innerWidth));
 await page.screenshot({path:'/tmp/tms-delegation-mobile.png'});
 await dialog.getByRole('button',{name:'×',exact:true}).click();
 await page.evaluate(()=>identities=null);
 await page.setViewportSize({width:390,height:844});await page.evaluate(()=>{addresses=['owner@example.com','sales@example.com','owner@alias.example','sales@alias.example'];openCompose({from:'sales@alias.example'})});await page.waitForFunction(()=>!document.getElementById('compose-from').disabled);
 assert.ok(await page.locator('#compose-from').evaluate(el=>el.getBoundingClientRect().right<=innerWidth));
 fs.mkdirSync('/tmp/tms-alias-qa',{recursive:true});await page.screenshot({path:'/tmp/tms-alias-qa/compose-mobile.png'});
 assert.deepEqual(errors,[]);console.log('Delegation: explicit send modes, payload, grant editing/revocation and mobile layout — OK');
}finally{await browser.close()}
