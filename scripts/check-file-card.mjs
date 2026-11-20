#!/usr/bin/env node
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
import {resolve} from 'node:path';
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage({viewport:{width:1440,height:1000}}),errors=[];
 page.on('pageerror',e=>errors.push(e.message));
 await page.setContent('<html lang="ru"><body><button id="btn-files-mkdir">Папка</button><div id="files-upload-panel"></div><ul id="files-list"></ul><div id="files-settings-menu"></div></body></html>');
 await page.addStyleTag({path:resolve('internal/frontend/dist/assets/app.css')});
 await page.evaluate(()=>{
  window.$=id=>document.getElementById(id);window.state={email:'owner@example.com'};window.filesState={ownerID:'',path:''};
  window.mailImageText=(ru,en)=>ru;window.t=x=>x;window.designIcon=()=>'';window.fmtBytes=x=>x+' B';window.refreshFiles=async()=>{};window.openPendingFileUploads=()=>{};window.escapeHtml=x=>x;
  window.cardIsDir=false;window.requests=[];window.confirmResult=true;window.askConfirm=async()=>confirmResult;
  window.grants=[{path:'team',email:'reader@example.com',rights:'read',grantee_id:'reader',inherited:true},{path:'team/report.txt',email:'editor@example.com',rights:'write',grantee_id:'editor',inherited:false}];window.shares=[];
  window.events=[{id:'one',action:'created',at:new Date().toISOString(),actor:{source:'web'},actor_email:'owner@example.com',subject:'<img src=x onerror=alert(1)>'}];
  window.api=async(path,options={})=>{
   const method=options.method||'GET';requests.push({path,method,body:options.body?JSON.parse(options.body):null});
   if(path.startsWith('/api/v1/files/card')){
    const recipient=path.includes('owner_id=');
    if(path.includes('before='))return {events:[{id:'old',action:'updated',at:new Date().toISOString(),actor:{source:'collabora'}}],next_cursor:''};
    return {path:'team/report.txt',name:'report.txt',size:42,mtime:new Date().toISOString(),is_dir:cardIsDir,can_manage:!recipient,effective_rights:recipient?'read':'owner',access:recipient?[grants[0]]:grants,shares:recipient?[]:shares,events:recipient?[]:events,next_cursor:recipient?'':'one'};
   }
   await new Promise(resolve=>setTimeout(resolve,30));
   const body=options.body?JSON.parse(options.body):null;
   if(path==='/api/v1/files/access'){
    if(method==='POST'){grants=grants.filter(g=>g.email!==body.email||g.inherited);grants.push({path:body.path,email:body.email,rights:body.rights,grantee_id:body.email,inherited:false})}
    else grants=grants.filter(g=>g.grantee_id!==body.grantee_id);
    return {};
   }
   if(path==='/api/v1/files/shares'){
    shares.push({id:'link',path:body.path,url:'https://mail.example.com/s/secret-token',rights:body.rights,inherited:false,active:true,password_protected:!!body.password,expires_at:new Date(Date.now()+86400000).toISOString(),download_count:0,max_downloads:body.max_downloads});return {};
   }
   if(path.startsWith('/api/v1/files/shares/')){shares=[];return {}}
   throw Error('Unexpected '+path);
  };
 });
 await page.addScriptTag({path:resolve('frontend/src/file-library.js')});
 await page.addScriptTag({path:resolve('frontend/src/file-card.js')});
 await page.evaluate(()=>openFileCard('team/report.txt'));
 assert.match(await page.locator('dialog').innerText(),/Из папки: team/);
 assert.equal(await page.locator('dialog img').count(),0);
 const select=page.getByLabel('Права: editor@example.com',{exact:true});await select.selectOption('read');
 await page.getByRole('button',{name:'Изменить',exact:true}).click();
 await page.waitForFunction(()=>grants.find(g=>g.email==='editor@example.com').rights==='read');
 await page.waitForFunction(()=>!document.querySelector('dialog .file-library-body').inert);
 const email=page.locator('dialog form input[type=email]');await email.fill('new@example.com');
 await page.locator('dialog form').first().evaluate(form=>{form.requestSubmit();form.requestSubmit()});
 await page.waitForFunction(()=>grants.some(g=>g.email==='new@example.com'));
 await page.waitForFunction(()=>!document.querySelector('dialog .file-library-body').inert);
 assert.equal(await page.evaluate(()=>requests.filter(r=>r.method==='POST'&&r.body?.email==='new@example.com').length),1);
 await page.getByLabel('Пароль (необязательно)',{exact:true}).fill('secret-password');
 await page.getByLabel('Лимит скачиваний (0 — без лимита)',{exact:true}).fill('3');
 await page.getByRole('button',{name:'Создать ссылку',exact:true}).click();
 await page.getByLabel('Публичная ссылка',{exact:true}).waitFor();
 assert.equal(await page.getByLabel('Пароль (необязательно)',{exact:true}).inputValue(),'');
 assert.match(await page.locator('dialog').innerText(),/С паролем/);
 await page.getByRole('button',{name:'Отозвать ссылку',exact:true}).click();
 await page.waitForFunction(()=>shares.length===0&&!document.querySelector('dialog .file-library-body').inert);
 await page.evaluate(()=>cardIsDir=true);
 await page.getByRole('button',{name:'Обновить карточку',exact:true}).click();
 await page.getByRole('button',{name:'Создать запрос файлов',exact:true}).waitFor();
 await page.getByLabel('Срок запроса, часов',{exact:true}).fill('24');
 await page.getByRole('button',{name:'Создать запрос файлов',exact:true}).click();
 await page.getByLabel('Ссылка для запроса файлов',{exact:true}).waitFor();
 assert.equal(await page.evaluate(()=>requests.filter(r=>r.body?.rights==='upload').length),1);
 await page.getByRole('button',{name:'Закрыть приём',exact:true}).click();
 await page.waitForFunction(()=>shares.length===0&&!document.querySelector('dialog .file-library-body').inert);
 await page.getByRole('button',{name:'Ранее',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('dialog').textContent.includes('Collabora'));
 await page.setViewportSize({width:390,height:844});
 assert.equal(await page.locator('dialog').evaluate(node=>node.scrollWidth<=node.clientWidth),true);
 for(const node of await page.locator('dialog input,dialog select,dialog button').all()){const box=await node.boundingBox();if(box)assert.ok(box.x>=0&&box.x+box.width<=390,'control fits mobile viewport')}
 await page.screenshot({path:'/tmp/tayga-file-card-mobile.png'});
 await page.locator('dialog header button').click();
 await page.waitForSelector('dialog',{state:'detached'});
 await page.evaluate(()=>openFileCard('team/report.txt','owner-id'));
 assert.equal(await page.locator('dialog form').count(),0);
 assert.equal(await page.getByRole('heading',{name:'Журнал событий',exact:true}).count(),0);
 assert.match(await page.locator('dialog').innerText(),/Показаны только ваши разрешения/);
 await page.evaluate(()=>state.email='different@example.com');
 await page.waitForSelector('dialog',{state:'detached'});
 assert.deepEqual(errors,[]);
 console.log('File card: inherited access, rights changes, one grant submit, link create/revoke, pagination, escaping, mobile and account switch — OK');
} finally {await browser.close()}
