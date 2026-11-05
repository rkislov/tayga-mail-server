#!/usr/bin/env node
// Isolated comparison UI test; mocked API, no user accounts or external editor.
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
import {resolve} from 'node:path';
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:1440,height:900}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.route('https://tayga.test/**',route=>route.fulfill({contentType:'text/html',body:'<html lang="ru" data-color-mode="light"><body><button id="btn-files-mkdir">Папка</button><div id="files-upload-panel"></div><ul id="files-list"></ul><div id="files-settings-menu"></div><p id="files-msg"></p></body></html>'}));
 await page.goto('https://tayga.test/');await page.addStyleTag({path:resolve('internal/frontend/dist/assets/app.css')});
 await page.evaluate(()=>{
  window.$=id=>document.getElementById(id);window.state={email:'owner@example.com'};window.filesState={ownerID:'',path:'',sharedRights:'write'};
  window.mailImageText=(ru,en)=>ru;window.t=key=>key;window.designIcon=()=>'×';window.fmtBytes=value=>value+' B';window.openPendingFileUploads=()=>{};window.refreshFiles=async()=>{};
  window.requests=[];window.api=async(path,options)=>{
   if(!options)return {entries:[{id:'revision-one',at:'2026-11-04T12:00:00Z',size:100,path:'file.docx'},{id:'revision-two',at:'2026-11-03T12:00:00Z',size:90,path:'file.docx'}],office_compare_enabled:true};
   const body=JSON.parse(options.body);requests.push(body);await new Promise(resolve=>setTimeout(resolve,20));
   if(window.failCompare){const error=Error('comparison version changed');error.status=409;throw error;}
   if(body.mode==='office')return {mode:'office',url:'https://tayga.test/office/compare/editor?access_token=fake-test-token'};
	if(body.mode==='structured'||body.mode==='details'){
	 if(body.mode==='details')return {mode:'structured',details:true,scope:'presentation',added:0,removed:0,modified:3,changes:[{kind:'change',section:'slide-style',group:'1',before:'font-size = 24 pt',after:'font-size = 28 pt'},{kind:'change',section:'slide-chart',group:'1',address:'1',before:'series/value = 3',after:'series/value = 9'},{kind:'change',section:'slide-animation',group:'1',before:'duration = 200 ms',after:'duration = 400 ms'}]};
	 const changes=window.emptyStructure?[]:window.manyStructure?Array.from({length:150},(_,i)=>({kind:'change',section:'cell',group:'План',address:'A'+(i+1),before:'1',after:'2'})):[body.path.endsWith('.xlsx')?{kind:'change',section:'cell',group:'План',address:'B2',before:'=SUM(A1:A2)',after:'<img src=x onerror="window.injected=true">=SUM(A1:A3)'}:{kind:'change',section:'slide-images',group:'2',index:2,before:'a'.repeat(64),after:'b'.repeat(64)}];
	 return {mode:'structured',scope:body.path.endsWith('.xlsx')?'spreadsheet':'presentation',added:0,removed:0,modified:changes.length,changes};
	}
   return {mode:'text',added:1,removed:1,lines:[{kind:'equal',text:'Без изменений',left:1,right:1},{kind:'delete',text:'Старый абзац',left:2},{kind:'insert',text:'<img src=x onerror="window.injected=true">Новый абзац',right:2}]};
  };
 });
 await page.addScriptTag({path:resolve('frontend/src/file-library.js')});
 await page.evaluate(()=>openFileComparison('file.docx','revision-one'));
 const submit=page.locator('dialog[open] button[type=submit]');
 await page.locator('dialog form').evaluate(form=>{form.requestSubmit();form.requestSubmit()});
 await page.waitForSelector('.file-comparison-insert');assert.equal(await page.evaluate(()=>requests.length),1);
 assert.equal(await page.locator('.file-comparison-insert img').count(),0);assert.equal(await page.evaluate(()=>window.injected),undefined);
 assert.match(await page.locator('dialog [role=status]').textContent(),/Добавлено строк: 1/);
 await page.screenshot({path:'/tmp/tayga-file-comparison-desktop.png'});
 await page.setViewportSize({width:390,height:844});await page.screenshot({path:'/tmp/tayga-file-comparison-mobile.png'});
 assert.equal(await page.locator('dialog[open]').evaluate(node=>node.scrollWidth<=node.clientWidth),true);
 await page.getByLabel('Новая версия',{exact:true}).selectOption('revision-one');await submit.click();assert.match(await page.locator('dialog [role=status]').textContent(),/две разные/);assert.equal(await page.evaluate(()=>requests.length),1);
 await page.getByLabel('Новая версия',{exact:true}).selectOption('current');await page.getByLabel('Режим',{exact:true}).selectOption('office');await submit.click();await page.waitForSelector('.file-office-dialog iframe');assert.match(await page.locator('.file-office-dialog iframe').getAttribute('src'),/\/office\/compare\/editor/);
 await page.keyboard.press('Escape');await page.waitForSelector('.file-office-dialog',{state:'detached'});
 await page.getByLabel('Режим',{exact:true}).selectOption('text');await page.evaluate(()=>failCompare=true);await submit.click();await page.waitForFunction(()=>document.querySelector('dialog [role=status]').textContent.includes('путь уже занят'));
 assert.equal(await page.locator('.file-comparison-insert').count(),0);
 await page.evaluate(()=>state.email='other@example.com');await submit.click();await page.waitForSelector('dialog',{state:'detached'});assert.equal(await page.evaluate(()=>requests.length),3);
	await page.evaluate(()=>{failCompare=false;return openFileComparison('plan.xlsx','revision-one')});
	assert.deepEqual(await page.getByLabel('Режим',{exact:true}).locator('option').evaluateAll(nodes=>nodes.map(node=>node.value)),['structured','details','visual']);
	await page.locator('dialog form').evaluate(form=>{form.requestSubmit();form.requestSubmit()});await page.waitForSelector('.file-structured-change');
	assert.equal(await page.evaluate(()=>requests.length),4);assert.match(await page.locator('.file-structured-change>strong').textContent(),/План.*B2/);assert.equal(await page.locator('.file-structured-change img').count(),0);
	await page.screenshot({path:'/tmp/tayga-spreadsheet-comparison-mobile.png'});assert.equal(await page.locator('dialog[open]').evaluate(node=>node.scrollWidth<=node.clientWidth),true);
	await page.setViewportSize({width:1440,height:900});await page.screenshot({path:'/tmp/tayga-spreadsheet-comparison-desktop.png'});
	await page.evaluate(()=>manyStructure=true);await submit.click();await page.waitForFunction(()=>document.querySelectorAll('.file-structured-change').length===100);
	const count=await page.evaluate(()=>requests.length);await page.getByLabel('Следующая страница',{exact:true}).click();assert.equal(await page.locator('.file-structured-change').count(),50);assert.equal(await page.evaluate(()=>requests.length),count);await page.evaluate(()=>manyStructure=false);
	await page.keyboard.press('Escape');await page.waitForSelector('dialog',{state:'detached'});
	await page.evaluate(()=>openFileComparison('slides.pptx','revision-one'));await submit.click();await page.waitForSelector('.file-structured-change');assert.match(await page.locator('.file-structured-change>strong').textContent(),/Изображения слайда/);
	await page.evaluate(()=>emptyStructure=true);await submit.click();await page.waitForFunction(()=>document.querySelector('.file-comparison-output').textContent.includes('В сравниваемом содержимом изменений нет'));
	await page.getByLabel('Режим',{exact:true}).selectOption('details');await submit.click();await page.waitForFunction(()=>document.querySelectorAll('.file-structured-change').length===3);
	assert.match(await page.locator('.file-comparison-output').textContent(),/Оформление слайда/);assert.match(await page.locator('.file-comparison-output').textContent(),/Диаграмма слайда/);assert.match(await page.locator('.file-comparison-output').textContent(),/Анимация слайда/);
	await page.screenshot({path:'/tmp/tayga-detailed-comparison-desktop.png'});await page.setViewportSize({width:390,height:844});await page.screenshot({path:'/tmp/tayga-detailed-comparison-mobile.png'});assert.equal(await page.locator('dialog[open]').evaluate(node=>node.scrollWidth<=node.clientWidth),true);
 assert.deepEqual(errors,[]);console.log('Version comparison UI: diff, XSS escaping, one request, mobile layout, same-version guard, office launch, conflict and account switch — OK');
}finally{await browser.close()}
