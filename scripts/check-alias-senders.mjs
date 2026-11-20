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
 let requests=[],hold=null,fail=false,addresses=['owner@example.com','sales@example.com','owner@alias.example','sales@alias.example'];
 async function api(path,options){
  if(path.endsWith('/senders')){const result={addresses:[...addresses],default:state.me.email};if(hold)await hold;return result}
  if(path.endsWith('/send')){const payload=JSON.parse(options.body);requests.push(payload);if(fail)throw Error('sender address is not authorized');return{}}
  throw Error('Unexpected API '+path);
 }
 ${fs.readFileSync('frontend/src/mail-attachments.js','utf8')}
 ${images.slice(images.indexOf('function ensureComposeCC('),images.indexOf('\nfunction forwardMailMessage('))}
 ${app.slice(app.indexOf('function composeBodyEl('),app.indexOf('\n$("btn-mail-delete")'))}
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
 await page.setViewportSize({width:390,height:844});await page.evaluate(()=>{addresses=['owner@example.com','sales@example.com','owner@alias.example','sales@alias.example'];openCompose({from:'sales@alias.example'})});await page.waitForFunction(()=>!document.getElementById('compose-from').disabled);
 assert.ok(await page.locator('#compose-from').evaluate(el=>el.getBoundingClientRect().right<=innerWidth));
 fs.mkdirSync('/tmp/tms-alias-qa',{recursive:true});await page.screenshot({path:'/tmp/tms-alias-qa/compose-mobile.png'});
 assert.deepEqual(errors,[]);console.log('Alias sender choice, outgoing payload, reply identity, revoked sender error, loading and stale draft handling — OK');
}finally{await browser.close()}
