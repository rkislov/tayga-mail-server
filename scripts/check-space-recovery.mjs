#!/usr/bin/env node
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
import {resolve} from 'node:path';
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:390,height:844}}),errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.setContent('<html lang="ru"><body><div id="files-upload-panel"></div><ul id="files-list"></ul><div id="files-settings-menu"></div></body></html>');
 await page.addStyleTag({path:resolve('internal/frontend/dist/assets/app.css')});
 await page.evaluate(()=>{
  window.$=id=>document.getElementById(id);window.state={email:'manager@example.com'};window.filesState={ownerID:'space',path:''};window.requests=[];window.stale=false;window.refreshes=0;
  window.mailImageText=(ru,en)=>ru;window.t=x=>x;window.designIcon=()=>'';window.fmtBytes=n=>n+' B';window.escapeHtml=x=>x;window.fileActivityLabel=x=>x;window.fileSpaceRoleLabel=x=>x;window.openPendingFileUploads=()=>{};window.askConfirm=async()=>true;window.refreshFiles=async()=>refreshes++;
  window.plan={plan_id:'plan',space_id:'space',checkpoint_at:'2026-11-20T10:00:00Z',mode:'full',total_changes:1,before_bytes:9,after_bytes:3,projected_used_bytes:10,quota_bytes:100,metadata_restored:true,settings:{name:'Project',version_days:30,trash_days:7,keep_versions:2,automatic:false,groups:[{name:'Managers',rights:'manage'}]},access_count:2,link_count:1,changes:[{kind:'modified',path:'folder/a.txt',before_bytes:9,after_bytes:3}]};
  window.api=async(path,options={})=>{
   const method=options.method||'GET',body=options.body instanceof File?'FILE':options.body?JSON.parse(options.body):null;requests.push({path,method,body});await new Promise(r=>setTimeout(r,10));
   if(path.endsWith('/preview'))return {...plan,mode:body.mode,metadata_restored:body.mode==='full'};
   if(path.endsWith('/apply')){if(stale){let e=Error('stale');e.status=409;throw e}return {space_id:'space',safety_checkpoint_id:'safety'}}
   if(path.endsWith('/import'))return {preview:plan};
   if(path.endsWith('/checkpoints'))return {id:'new'};
   if(path.endsWith('/recovery'))return {checkpoints:[{id:'point',at:'2026-11-20T10:00:00Z',label:'Baseline',automatic:false}]};
   if(path.endsWith('/spaces/space'))return {name:'Project',rights:'manage'};
   throw Error('Unexpected '+path);
  };
 });
 await page.addScriptTag({path:resolve('frontend/src/file-library.js')});await page.addScriptTag({path:resolve('frontend/src/file-space-recovery.js')});
 await page.evaluate(()=>openSpaceRecovery('space'));const dialog=page.locator('dialog');
 assert.equal(await dialog.getByRole('button',{name:'Применить восстановление',exact:true}).count(),0);
 await dialog.getByRole('button',{name:'Предпросмотр восстановления',exact:true}).click();await dialog.getByRole('button',{name:'Применить восстановление',exact:true}).waitFor();
 assert.equal(await page.evaluate(()=>requests.filter(r=>r.path.endsWith('/apply')).length),0);assert.match(await dialog.textContent(),/folder\/a.txt/);
 await dialog.getByRole('button',{name:'Применить восстановление',exact:true}).click();await dialog.getByRole('button',{name:'Предпросмотр отмены восстановления',exact:true}).waitFor();
 await page.waitForFunction(()=>refreshes===1);
 await dialog.getByRole('button',{name:'Предпросмотр отмены восстановления',exact:true}).click();await dialog.getByRole('button',{name:'Применить восстановление',exact:true}).waitFor();
 assert.equal(await page.evaluate(()=>requests.filter(r=>r.path.endsWith('/preview')).at(-1).body.checkpoint_id),'safety');
 await dialog.getByLabel('Объём восстановления',{exact:true}).selectOption('folder');await dialog.getByLabel('Путь папки от корня пространства',{exact:true}).fill('folder');
 await dialog.getByLabel('Выбрать по времени (необязательно)',{exact:true}).fill('2026-11-20T12:30');await dialog.getByRole('button',{name:'Предпросмотр восстановления',exact:true}).click();
 await page.waitForFunction(()=>requests.filter(r=>r.path.endsWith('/preview')).at(-1).body.mode==='folder');await page.waitForFunction(()=>document.querySelector('dialog').textContent.includes('прямые разрешения'));
 assert.equal(await page.evaluate(()=>requests.filter(r=>r.path.endsWith('/preview')).at(-1).body.path),'folder');assert.ok(await page.evaluate(()=>requests.filter(r=>r.path.endsWith('/preview')).at(-1).body.at));
 await page.evaluate(()=>stale=true);await dialog.getByRole('button',{name:'Применить восстановление',exact:true}).click();await page.waitForFunction(()=>document.querySelector('dialog').textContent.includes('изменилось после предпросмотра'));
 await dialog.getByLabel('Импорт архива пространства (.tar.gz)',{exact:true}).setInputFiles({name:'backup.tar.gz',mimeType:'application/gzip',buffer:Buffer.from('stub archive')});await dialog.getByRole('button',{name:'Загрузить архив и проверить',exact:true}).click();await page.waitForFunction(()=>requests.some(r=>r.body==='FILE'));
 await page.waitForFunction(()=>!document.querySelector('dialog').querySelector('[inert]'));assert.equal(await dialog.evaluate(n=>n.scrollWidth<=n.clientWidth),true);await page.screenshot({path:'/tmp/tayga-space-recovery-mobile.png'});
 await page.setViewportSize({width:1440,height:1000});await page.screenshot({path:'/tmp/tayga-space-recovery-desktop.png'});
 await page.evaluate(()=>state.email='other@example.com');await page.waitForSelector('dialog',{state:'detached'});assert.deepEqual(errors,[]);
 console.log('Space recovery UI: preview before apply, safety undo, folder/time scope, stale plan, archive import, mobile/desktop, identity switch — OK');
}finally{await browser.close()}
