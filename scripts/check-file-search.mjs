#!/usr/bin/env node
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
import {resolve} from 'node:path';
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:390,height:844}}),errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.setContent('<html lang="ru"><body><button id="btn-files-mkdir">Папка</button><div id="files-upload-panel"></div><ul id="files-list"></ul><div id="files-settings-menu"></div></body></html>');
 await page.addStyleTag({path:resolve('internal/frontend/dist/assets/app.css')});
 await page.evaluate(()=>{
  window.$=id=>document.getElementById(id);window.state={email:'reader@example.com'};window.filesState={ownerID:'',path:''};window.requests=[];window.opened=[];
  window.mailImageText=(ru,en)=>ru;window.t=x=>x;window.designIcon=()=>'';window.fmtBytes=x=>x+' B';window.openPendingFileUploads=()=>{};window.escapeHtml=x=>x;
  window.refreshFiles=async()=>opened.push({...filesState});window.selectFileEntry=name=>opened.push(name);
  window.api=async(path)=>{
   requests.push(path);const url=new URL(path,'https://files.example.com'),q=url.searchParams.get('q'),page=Number(url.searchParams.get('page'));if(q==='slow')await new Promise(resolve=>setTimeout(resolve,100));
   return {entries:[{owner_id:'shared-owner',owner_email:'owner@example.com',path:'team/'+(page===2?'second.docx':q+'.pdf'),name:(page===2?'second.docx':q+'.pdf'),shared:true,rights:'read',is_dir:false,match:'content',snippet:{before:'До <img src=x> ',match:'ПОСТАВКА',after:' после'}}],total:51,page_size:50,skipped_documents:1,empty_documents:1,pdf_search_available:false,truncated:true};
  };
 });
 await page.addScriptTag({path:resolve('frontend/src/file-library.js')});
 await page.getByRole('button',{name:'Искать везде',exact:true}).click();
 const query=page.getByRole('searchbox',{name:'Имя или фраза из документа',exact:true});
 await query.fill('поставка');await page.getByRole('button',{name:'Найти',exact:true}).click();
 await page.locator('dialog mark').waitFor();assert.equal(await page.locator('dialog mark').innerText(),'ПОСТАВКА');assert.equal(await page.locator('dialog img').count(),0);
 assert.match(await page.locator('dialog').innerText(),/owner@example.com/);assert.match(await page.locator('dialog').innerText(),/pdftotext/);assert.match(await page.locator('dialog').innerText(),/неполные/);
 assert.match(await page.evaluate(()=>requests.at(-1)),/mode=all&scope=accessible/);
 await page.getByRole('button',{name:'Следующая страница',exact:true}).click();await page.getByRole('button',{name:'team/second.docx',exact:true}).waitFor();
 await page.getByLabel('Где искать',{exact:true}).selectOption('content');await page.getByLabel('Область поиска',{exact:true}).selectOption('own');
 await page.waitForFunction(()=>requests.at(-1).includes('mode=content&scope=own&page=1'));
 await query.fill('slow');await page.getByRole('button',{name:'Найти',exact:true}).click();await query.fill('fast');await page.getByRole('button',{name:'Найти',exact:true}).click();
 await page.getByRole('button',{name:'team/fast.pdf',exact:true}).waitFor();await page.waitForTimeout(150);assert.equal(await page.getByRole('button',{name:'team/slow.pdf',exact:true}).count(),0);
 assert.equal(await page.locator('dialog').evaluate(node=>node.scrollWidth<=node.clientWidth),true);
 await page.getByLabel('Область поиска',{exact:true}).selectOption('accessible');
 await page.getByRole('button',{name:'team/fast.pdf',exact:true}).waitFor();
 await page.screenshot({path:'/tmp/tayga-file-content-search-mobile.png'});
 await page.getByRole('button',{name:'team/fast.pdf',exact:true}).click();await page.waitForSelector('dialog',{state:'detached'});
 assert.equal(await page.evaluate(()=>opened[0].ownerID),'shared-owner');assert.equal(await page.evaluate(()=>opened[0].listTarget),'team/fast.pdf');assert.equal(await page.evaluate(()=>opened[0].sharedRights),'read');
 await page.evaluate(()=>openFileSearch());await page.evaluate(()=>state.email='different@example.com');await page.waitForSelector('dialog',{state:'detached'});
 assert.deepEqual(errors,[]);console.log('Content search UI: modes/scopes, safe snippets, pagination, stale responses, shared navigation, mobile and account switch — OK');
}finally{await browser.close()}
