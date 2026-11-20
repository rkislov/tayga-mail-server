#!/usr/bin/env node
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
import {resolve} from 'node:path';
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:1440,height:900}}),errors=[];
 page.on('pageerror',e=>errors.push(e.message));
 await page.setContent('<html lang="ru"><body><button id="btn-files-mkdir">Папка</button><div id="files-upload-panel"></div><ul id="files-list"></ul><div id="files-settings-menu"></div></body></html>');
 await page.addStyleTag({path:resolve('internal/frontend/dist/assets/app.css')});
 await page.evaluate(()=>{
  window.$=id=>document.getElementById(id);window.state={email:'owner@example.com'};window.filesState={ownerID:'',path:''};
  window.mailImageText=(ru,en)=>ru;window.t=x=>x;window.designIcon=()=>'';window.fmtBytes=x=>x+' B';window.refreshFiles=async()=>{};window.openPendingFileUploads=()=>{};window.escapeHtml=x=>x;
  window.requests=[];window.askConfirm=async()=>true;window.policy={automatic:false,version_days:0,trash_days:0,keep_versions:0};window.cleaned=false;window.protectedVersion=false;
  window.api=async(path,options={})=>{
   requests.push([path,options.method||'GET']);
   if(path.endsWith('/retention')){if(options.method==='PUT')policy=JSON.parse(options.body);return policy}
   if(path.endsWith('/cleanup/preview'))return {entries:[{}],quota_bytes:42};
   if(path.endsWith('/cleanup/run')){cleaned=true;return {entries:[{}],quota_bytes:42}}
   if(path.endsWith('/cleanup/journal'))return {entries:cleaned?[{at:new Date().toISOString(),kind:'versions',revision:{path:'report.txt'},bytes:42,completed:true}]:[]};
   if(path.endsWith('/protection')){protectedVersion=JSON.parse(options.body).protected;return {}}
   if(path.includes('/versions?'))return {entries:[{id:'test-id',path:'report.txt',at:new Date().toISOString(),size:42,protected:protectedVersion}]};
   throw Error('Unexpected '+path);
  };
 });
 await page.addScriptTag({path:resolve('frontend/src/file-library.js')});
 await page.getByRole('button',{name:'Хранение и очистка',exact:true}).click();
 const run=page.getByRole('button',{name:'Очистить сейчас',exact:true});
 await page.waitForFunction(()=>document.querySelector('dialog input[type=number]')?.value==='0');
 assert.equal(await run.isDisabled(),true);
 await page.locator('dialog input[type=number]').nth(0).fill('30');
 await page.locator('dialog input[type=number]').nth(1).fill('14');
 await page.locator('dialog input[type=number]').nth(2).fill('2');
 await page.getByRole('button',{name:'Сохранить политику',exact:true}).click();
 await page.waitForFunction(()=>policy.version_days===30);
 await page.getByRole('button',{name:'Рассчитать очистку',exact:true}).click();
 await page.waitForFunction(()=>!Array.from(document.querySelectorAll('dialog button')).find(b=>b.textContent==='Очистить сейчас').disabled);
 assert.match(await page.locator('dialog').innerText(),/42 B/);
 await run.click();await page.waitForFunction(()=>cleaned&&document.querySelector('dialog').textContent.includes('Удалено'));
 assert.equal(await run.isDisabled(),true);
 await page.setViewportSize({width:390,height:844});
 assert.equal(await page.locator('dialog').evaluate(node=>node.scrollWidth<=node.clientWidth),true);
 for(const button of await page.locator('dialog .file-library-body button').all()){const box=await button.boundingBox();assert.ok(box.x>=0&&box.x+box.width<=390,'button fits mobile viewport')}
 await page.screenshot({path:'/tmp/tayga-file-retention-mobile.png'});
 await page.keyboard.press('Escape');
 await page.evaluate(()=>openFileHistory('versions','report.txt'));
 await page.getByRole('button',{name:'Защитить версию',exact:true}).click();
 await page.waitForFunction(()=>protectedVersion);
 await page.getByRole('button',{name:'Снять защиту',exact:true}).waitFor();
 assert.deepEqual(errors,[]);
 console.log('Retention UI: save, preview, cleanup, journal, protection and mobile layout — OK');
}finally{await browser.close()}
