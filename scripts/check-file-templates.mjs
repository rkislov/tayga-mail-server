#!/usr/bin/env node
// Isolated browser test: no credentials, network services or user files.
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
import {resolve} from 'node:path';
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:1440,height:900}});
 const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.setContent('<html lang="ru" data-color-mode="light"><body><button id="btn-files-mkdir">Папка</button><div id="files-upload-panel"></div><ul id="files-list"></ul><div id="files-settings-menu"></div><p id="files-msg"></p></body></html>');
 await page.addStyleTag({path:resolve('internal/frontend/dist/assets/app.css')});
 await page.evaluate(()=>{
  window.$=id=>document.getElementById(id);window.state={email:'owner@example.com'};
  window.filesState={ownerID:'',path:'docs',sharedRights:'write'};window.requests=[];window.opened=[];
  window.mailImageText=(ru,en)=>ru;window.t=key=>key;window.designIcon=()=>'';
  window.refreshFiles=async()=>{};window.selectFileEntry=()=>{};window.setMsg=(node,message)=>{if(node)node.textContent=message};window.joinPath=(path,name)=>[path,name].filter(Boolean).join('/');
	window.openPendingFileUploads=()=>{};
  window.api=async(path,options)=>{
   if(!options)return {templates:[{id:'document',title:'Пустой документ',extension:'.docx'},{id:'spreadsheet',title:'Таблица',extension:'.xlsx'}],office_enabled:false};
   const body=JSON.parse(options.body);requests.push(body);if(window.failCreate){const error=Error('collision');error.status=409;throw error}
   await new Promise(resolve=>setTimeout(resolve,30));return {path:body.directory+'/'+body.name,name:body.name,office_enabled:false};
  };
 });
 await page.addScriptTag({path:resolve('frontend/src/file-library.js')});
 await page.click('#btn-files-create-document');
 await page.locator('dialog select').selectOption('spreadsheet');
 assert.equal(await page.locator('dialog input[aria-label="Имя документа"]').inputValue(),'Новый документ.xlsx');
 await page.locator('dialog input[aria-label="Имя документа"]').fill('План.xlsx');
 await page.locator('dialog form').evaluate(form=>{form.requestSubmit();form.requestSubmit()});
 await page.waitForSelector('dialog',{state:'detached'});
 assert.equal(await page.evaluate(()=>requests.length),1);
 assert.deepEqual(await page.evaluate(()=>requests[0]),{directory:'docs',name:'План.xlsx',owner_id:'',template_id:'spreadsheet'});
 await page.evaluate(()=>{filesState.ownerID='shared-owner';filesState.sharedRights='read';failCreate=true;return openDocumentTemplate({name:'base.docx',path:'team/base.docx',owner:'shared-owner'})});
 await page.locator('dialog button[type="submit"]').click();
 await page.waitForFunction(()=>document.querySelector('dialog [role="status"]').textContent.includes('путь уже занят'));
 assert.deepEqual(await page.evaluate(()=>requests[1]),{directory:'',name:'base (копия).docx',owner_id:'',source_path:'team/base.docx',source_owner_id:'shared-owner'});
 await page.setViewportSize({width:390,height:844});
 await page.screenshot({path:'/tmp/tayga-file-template-mobile.png'});
 assert.equal(await page.locator('dialog').evaluate(node=>node.scrollWidth<=node.clientWidth),true);
 await page.keyboard.press('Escape');
	await page.waitForSelector('dialog',{state:'detached'});
 await page.evaluate(()=>{filesState.ownerID='';filesState.path='docs';failCreate=false;return openDocumentTemplate()});
 await page.evaluate(()=>state.email='different@example.com');
 await page.locator('dialog button[type="submit"]').click();
 await page.waitForSelector('dialog',{state:'detached'});
 assert.equal(await page.evaluate(()=>requests.length),2);
 assert.deepEqual(errors,[]);
 console.log('Template dialog: formats, one submit, shared-source personal copy, conflict, mobile layout and account switch — OK');
}finally{await browser.close()}
