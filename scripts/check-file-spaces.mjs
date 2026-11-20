#!/usr/bin/env node
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
import {resolve} from 'node:path';
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:390,height:844}}),errors=[];page.on('pageerror',error=>errors.push(error.message));
 await page.setContent('<html lang="ru"><body><button id="btn-files-refresh">Обновить</button><button id="btn-files-mkdir">Папка</button><div id="files-upload-panel"></div><ul id="files-list"></ul><div id="files-settings-menu"></div></body></html>');
 await page.addStyleTag({path:resolve('internal/frontend/dist/assets/app.css')});
 await page.evaluate(()=>{
  window.$=id=>document.getElementById(id);window.state={email:'manager@example.com'};window.filesState={ownerID:'',spaceID:'',spaceRights:'',path:''};window.requests=[];window.opened=[];window.conflict=false;
  window.mailImageText=(ru,en)=>ru;window.t=x=>x;window.designIcon=()=>'';window.fmtBytes=n=>n+' B';window.openPendingFileUploads=()=>{};window.refreshFiles=async()=>opened.push({...filesState});window.escapeHtml=x=>x;
  window.space={id:'space-id',tenant_id:'tenant',name:'Project Alpha',kind:'project',description:'',quota_bytes:100,version_days:30,trash_days:7,keep_versions:2,automatic:false,revision:1,rights:'manage',groups:[{group_id:'g1',name:'Managers',rights:'manage'}]};
  window.api=async(path,options={})=>{
   const method=options.method||'GET';requests.push({path,method,body:options.body?JSON.parse(options.body):null});
   if(method!=='GET'){if(conflict){const error=Error('conflict');error.status=409;throw error}await new Promise(resolve=>setTimeout(resolve,25));space={...JSON.parse(options.body),id:'space-id',rights:'manage',revision:space.revision+1};return space}
   if(path==='/api/v1/files/spaces')return {spaces:[space],can_create:true,tenant_id:'tenant'};
   if(path.includes('/spaces/groups'))return {groups:[{id:'g1',name:'Managers',email:'managers@example.com'},{id:'g2',name:'Writers',email:'writers@example.com'}]};
   if(path.includes('/spaces/'))return space;
   if(path.includes('/usage'))return {used_bytes:20};throw Error('Unexpected '+path);
  };
 });
 await page.addScriptTag({path:resolve('frontend/src/file-library.js')});await page.addScriptTag({path:resolve('frontend/src/file-spaces.js')});
 await page.getByRole('button',{name:'Общие пространства',exact:true}).click();await page.getByRole('button',{name:'Настройки',exact:true}).click();
 const settings=page.locator('dialog').last();await settings.getByLabel('Добавить группу',{exact:true}).selectOption('g2');await settings.getByLabel('Права группы: Writers',{exact:true}).selectOption('write');
 await settings.getByLabel('Квота в байтах (0 — без ограничения)',{exact:true}).fill('200');
 await settings.getByLabel('Хранить версии, дней (0 — бессрочно)',{exact:true}).fill('60');
 await settings.locator('form').evaluate(form=>{form.requestSubmit();form.requestSubmit()});
 await page.waitForFunction(()=>space.quota_bytes===200);assert.equal(await page.evaluate(()=>requests.filter(r=>r.method==='PUT').length),1);
 assert.equal(await page.evaluate(()=>space.groups.find(g=>g.group_id==='g2').rights),'write');assert.equal(await page.evaluate(()=>space.version_days),60);
 await page.evaluate(()=>conflict=true);await settings.getByLabel('Название',{exact:true}).fill('Unsaved name');await settings.getByRole('button',{name:'Сохранить пространство',exact:true}).click();
 await page.waitForFunction(()=>[...document.querySelectorAll('dialog')].at(-1).textContent.includes('другим управляющим'));assert.equal(await settings.getByLabel('Название',{exact:true}).inputValue(),'Unsaved name');
 assert.equal(await settings.evaluate(node=>node.scrollWidth<=node.clientWidth),true);await page.screenshot({path:'/tmp/tayga-file-spaces-mobile.png'});
 await settings.locator('header button').click();await page.evaluate(()=>openFileSpaces());await page.locator('dialog').getByRole('button',{name:'Открыть',exact:true}).click();await page.waitForSelector('dialog',{state:'detached'});
 assert.equal(await page.evaluate(()=>opened[0].ownerID),'space-id');assert.equal(await page.evaluate(()=>opened[0].spaceRights),'manage');assert.equal(await page.evaluate(()=>opened[0].sharedRights),'write');
 await page.evaluate(()=>{conflict=false;return openFileSpaceSettings('','tenant')});await page.locator('dialog').getByLabel('Название',{exact:true}).fill('New department');await page.locator('dialog').getByLabel('Тип пространства',{exact:true}).selectOption('department');await page.locator('dialog').getByRole('button',{name:'Сохранить пространство',exact:true}).click();
 await page.waitForFunction(()=>requests.some(r=>r.method==='POST'));assert.equal(await page.evaluate(()=>requests.find(r=>r.method==='POST').body.kind),'department');
 await page.evaluate(()=>state.email='different@example.com');await page.waitForSelector('dialog',{state:'detached'});
 assert.deepEqual(errors,[]);console.log('Shared spaces UI: list, roles, quota/retention, one save, conflict preservation, navigation, create, mobile and account switch — OK');
}finally{await browser.close()}
