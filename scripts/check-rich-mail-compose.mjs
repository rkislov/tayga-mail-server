import fs from 'node:fs';
import assert from 'node:assert/strict';
import {chromium} from 'playwright-core';
const app=fs.readFileSync('frontend/src/app.js','utf8'),markup=fs.readFileSync('internal/frontend/dist/index.html','utf8');
const compose=markup.slice(markup.indexOf('<div id="compose-backdrop"'),markup.indexOf('<!-- CALENDAR -->'));
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage({viewport:{width:1200,height:900}}),errors=[],external=[];
 page.on('pageerror',e=>errors.push(e.message));
 await page.route('http://localhost/**',route=>{
  const url=new URL(route.request().url());
  if(url.pathname.startsWith('/assets/ckeditor5/'))return route.fulfill({body:fs.readFileSync('internal/frontend/dist'+url.pathname),contentType:url.pathname.endsWith('.css')?'text/css':'text/javascript'});
  return route.fulfill({body:'<!doctype html><html><head><meta charset="utf-8"></head><body>'+compose+'</body></html>',contentType:'text/html; charset=utf-8'});
 });
 page.on('request',r=>{if(!r.url().startsWith('http://localhost/')&&!r.url().startsWith('data:'))external.push(r.url())});
 await page.goto('http://localhost');
 await page.addStyleTag({path:'internal/frontend/dist/assets/app.css'});await page.addStyleTag({path:'internal/frontend/dist/assets/ckeditor5/ckeditor5.css'});
 await page.addScriptTag({content:`
 const $=id=>document.getElementById(id),state={me:{id:'owner',email:'owner@example.com'}},signatureState={text:'Support\\nExample',new_messages:true,replies:true};
 const t=k=>k,mailImageText=ru=>ru,setMsg=(el,text)=>{if(el)el.textContent=text},fmtBytes=n=>n+' B',WEB_UPLOAD_BYTES=10*1024**3;
 function ensureComposeCC(){if(!$('compose-cc')){const cc=document.createElement('input');cc.id='compose-cc';$('compose-panel').append(cc)}}
 function refreshMail(){};
 let sends=[];
 async function api(path,options){
  if(path.endsWith('/senders'))return {addresses:['owner@example.com'],default:'owner@example.com'};
  if(path.endsWith('/limits'))return {message_bytes:25*1024**2,large_attachment_bytes:10*1024**2,attachment_bytes:10*1024**2,share_ttl_hours:24};
  if(path.endsWith('/send')){sends.push(JSON.parse(options.body));return {status:'accepted'}};
  throw Error('Unexpected API '+path);
 }
 ${app.slice(app.indexOf('function composeBodyEl('),app.indexOf('\n$("btn-mail-delete")'))}
 ${fs.readFileSync('frontend/src/mail-attachments.js','utf8')}
 ${fs.readFileSync('frontend/src/rich-mail-editor.js','utf8')}
 openCompose({to:'reader@example.com',subject:'Photos',bodyHTML:'<p>Hello</p><table style="width:100%"><tbody><tr><td style="background-color:#eeeeee">Quoted cell</td></tr></tbody></table>'});
 `});
 const body=page.locator('#compose-rte .ck-editor__editable_root');await body.waitFor();
 await page.waitForFunction(()=>composeRichEditor && !composeRichEditorBoot);
 for(const text of ['Hello','Support','Example'])assert.ok((await body.innerText()).includes(text));
 assert.match(await page.evaluate(()=>getComposeHTML()),/Quoted cell/);
 assert.match(await page.evaluate(()=>getComposeHTML()),/background-color/);
 const photo=fs.readFileSync('docs/screenshots/ui-notes-tayga.png');
 await page.locator('#compose-rte .ck-file-dialog-button input[type=file]').setInputFiles({name:'team.png',mimeType:'image/png',buffer:photo});
 await page.waitForFunction(()=>getComposeHTML().includes('data:image/'));
 await page.waitForFunction(()=>!mailEditorPending(composeRichEditor));
 assert.equal(await page.locator('#btn-compose-send').isEnabled(),true);
 const dimensions=await body.locator('img').evaluate(img=>({width:img.naturalWidth,height:img.naturalHeight}));assert.ok(dimensions.width<=1600&&dimensions.height<=1600);
 // The existing Files attachment link stays part of the editor model and can be removed.
 await page.evaluate(()=>addComposeFileLink({id:'link-test',file:{name:'report.pdf',size:12},share:{url:'https://example.com/report',expires_at:'2026-12-01'}}));
 assert.match(await page.evaluate(()=>getComposeHTML()),/data-upload-link="link-test"/);
 await page.evaluate(()=>mutateComposeHTML(body=>body.querySelector('[data-upload-link="link-test"]').remove()));
 assert.doesNotMatch(await page.evaluate(()=>getComposeHTML()),/report.pdf/);
 await page.setViewportSize({width:390,height:844});
 assert.ok(await page.locator('#compose-panel').evaluate(el=>el.scrollWidth<=el.clientWidth+1),'compose mobile overflow');
 await page.screenshot({path:'/tmp/tms-ckeditor-compose-mobile.png'});
 await page.locator('#btn-compose-send').click();
 await page.waitForFunction(()=>sends.length===1);
 const sent=await page.evaluate(()=>sends[0]);assert.match(sent.html,/data:image\/(png|jpeg);base64,/);assert.match(sent.text,/Hello/);assert.deepEqual(sent.to,['reader@example.com']);
 await page.evaluate(()=>openCompose({to:'other@example.com',body:'New draft'}));
 assert.doesNotMatch(await page.evaluate(()=>getComposeHTML()),/data:image\//);assert.match(await body.innerText(),/New draft/);
 await page.setViewportSize({width:1200,height:900});
 // An unfinished photo cannot enable Send or leak into a replacement draft.
 await page.evaluate(()=>{
  const original=prepareMailPhoto;let release;
  window.releasePhoto=()=>release();
  const gate=new Promise(resolve=>{release=resolve});
  prepareMailPhoto=async(...args)=>{await gate;return original(...args)};
 });
 await page.locator('#compose-rte .ck-file-dialog-button input[type=file]').setInputFiles({name:'pending.png',mimeType:'image/png',buffer:photo});
 await page.waitForFunction(()=>mailEditorPending(composeRichEditor));
 assert.equal(await page.locator('#btn-compose-send').isDisabled(),true);
 await page.evaluate(()=>{openCompose({to:'last@example.com',body:'Replacement draft'});releasePhoto()});
 await page.waitForFunction(()=>!mailEditorPending(composeRichEditor));
 assert.doesNotMatch(await page.evaluate(()=>getComposeHTML()),/data:image\//);
 assert.match(await body.innerText(),/Replacement draft/);
 assert.deepEqual(errors,[]);assert.deepEqual(external,[]);
 console.log('CKEditor compose: local photo, resize, signature, Files links, send payload, next draft, mobile and no CDN — OK');
}finally{await browser.close()}
