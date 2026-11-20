#!/usr/bin/env node
// Real PDF.js rendering of deterministic PDF fixtures; API/converter are isolated.
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
import {resolve} from 'node:path';
import {readFile} from 'node:fs/promises';
function pdf(pages){
 const objects=['<< /Type /Catalog /Pages 2 0 R >>',`<< /Type /Pages /Count ${pages.length} /Kids [${pages.map((_,i)=>`${4+i*2} 0 R`).join(' ')}] >>`,'<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>'];
 for(const [i,stream] of pages.entries()){objects.push(`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 600 400] /Resources << /Font << /F1 3 0 R >> >> /Contents ${5+i*2} 0 R >>`);objects.push(`<< /Length ${Buffer.byteLength(stream)} >>\nstream\n${stream}\nendstream`)}
 let body='%PDF-1.4\n',offsets=[0];for(const [i,object] of objects.entries()){offsets.push(Buffer.byteLength(body));body+=`${i+1} 0 obj\n${object}\nendobj\n`}const start=Buffer.byteLength(body);body+=`xref\n0 ${objects.length+1}\n0000000000 65535 f \n${offsets.slice(1).map(o=>String(o).padStart(10,'0')+' 00000 n \n').join('')}trailer\n<< /Size ${objects.length+1} /Root 1 0 R >>\nstartxref\n${start}\n%%EOF`;return Buffer.from(body);
}
const rect=(color,x)=>`${color} rg ${x} 50 100 80 re f`;
const matte=(color,shape='')=>`${color} rg 0 0 600 400 re f\n${shape}`;
const background=matte('0.9 0.9 0.9','0 0 1 rg 5 5 20 20 re f');
const fixture=pdf([background,background,background,background,background,matte('0 0 0'),matte('1 1 1'),matte('0 0 0',rect('1 0 0',50)),matte('1 1 1',rect('1 0 0',50)),matte('0 0 0',rect('0 1 0',250)),matte('1 1 1',rect('0 1 0',250))]);
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:1440,height:900}}),errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.route('https://tayga.test/**',async route=>{
  const url=new URL(route.request().url()),right=url.searchParams.get('side')==='right',trigger=right?'constructor':'3';
  if(url.pathname.startsWith('/assets/pdfjs/')){const file=resolve('internal/frontend/dist',url.pathname.slice(1));await route.fulfill({body:await readFile(file),contentType:file.endsWith('.mjs')?'application/javascript':'application/octet-stream'});return}
  if(url.pathname==='/office/compare/pdf'){await route.fulfill({body:fixture,contentType:'application/pdf'});return}
  if(url.pathname==='/office/compare/timeline'){await route.fulfill({contentType:'application/json',body:JSON.stringify({frames:11,slides:[{states:[1,2,3,4],background:5,probe:[6,7],layers:[{target:'2',black:8,white:9,hidden:true},{target:trigger,black:10,white:11,hidden:true}],hits:[{target:'2',name:'Red object',box:[50/600,270/400,100/600,80/400]},{target:trigger,name:'Trigger object',box:[250/600,270/400,100/600,80/400]}],effects:[{kind:'fade',target:'2',exit:false,duration:1000,trigger:'click',delay:0},{kind:'fade',target:trigger,exit:false,duration:1000,trigger:'with',delay:0},{kind:'path',target:'2',duration:right?2000:1000,trigger:'click',event:trigger,delay:0,path:[{x:0,y:0},{x:.25,y:0}]}],transition:{kind:'cut',duration:0,direction:''}}]})});return}
  await route.fulfill({contentType:'text/html',body:'<html lang="ru"><body><button id="btn-files-mkdir"></button><div id="files-upload-panel"></div><ul id="files-list"></ul><div id="files-settings-menu"></div><p id="files-msg"></p></body></html>'});
 });
 await page.goto('https://tayga.test');await page.addStyleTag({path:resolve('internal/frontend/dist/assets/app.css')});
 await page.evaluate(()=>{window.$=id=>document.getElementById(id);window.state={email:'owner@test'};window.filesState={};window.mailImageText=(ru,en)=>ru;window.t=key=>key;window.designIcon=()=> '×';window.fmtBytes=String;window.openPendingFileUploads=()=>{};window.refreshFiles=async()=>{};window.api=async(path,options)=>options?{mode:'motion',left_timeline:'/office/compare/timeline?side=left',right_timeline:'/office/compare/timeline?side=right',left_url:'/office/compare/pdf?side=left&storyboard=1',right_url:'/office/compare/pdf?side=right&storyboard=1'}:{entries:[{id:'old',at:'2026-11-05',size:20}],visual_compare_enabled:true}});
 await page.addScriptTag({path:resolve('frontend/src/file-library.js')});await page.addScriptTag({path:resolve('frontend/src/file-motion.js')});await page.evaluate(()=>openFileComparison('layers.pptx','old'));await page.getByLabel('Режим',{exact:true}).selectOption('motion');await page.locator('button[type=submit]').click();
 await page.waitForFunction(()=>document.querySelectorAll('.file-motion-dialog .file-render-content canvas').length===2);
 const seek=async value=>{await page.getByLabel('Время кадра',{exact:true}).evaluate((node,value)=>{node.value=String(value);node.dispatchEvent(new Event('input'))},value)};
 const pixel=async(x,y,side=0)=>page.locator('.file-motion-dialog .file-render-content canvas').nth(side).evaluate((c,{x,y})=>Array.from(c.getContext('2d').getImageData(Math.floor(c.width*x),Math.floor(c.height*y),1,1).data),{x,y});
 await seek(0);const hiddenCanvas=page.locator('.file-motion-dialog .file-render-content canvas').first();await hiddenCanvas.click({position:await hiddenCanvas.evaluate(c=>({x:c.clientWidth*300/600,y:c.clientHeight*300/400}))});assert.equal(await page.getByLabel('Пауза',{exact:true}).count(),0);
 await seek(1000);const red=await pixel(100/600,300/400),green=await pixel(300/600,300/400);assert.ok(red[0]>230&&red[1]>100&&red[1]<140,JSON.stringify(red));assert.ok(green[1]>230&&green[0]>100&&green[0]<140,JSON.stringify(green));
 await seek(1500);const canvas=page.locator('.file-motion-dialog .file-render-content canvas').first();await canvas.click({position:await canvas.evaluate(c=>({x:c.clientWidth*300/600,y:c.clientHeight*300/400}))});await page.getByLabel('Пауза',{exact:true}).click();
 await seek(2000);assert.ok((await pixel(200/600,300/400))[1]<10);assert.ok((await pixel(75/600,300/400))[1]>220);assert.ok((await pixel(75/600,300/400,1))[1]>220);assert.deepEqual(await pixel(15/600,385/400),[0,0,255,255]);
 const moved=await pixel(200/600,300/400);await seek(1000);await seek(2000);assert.deepEqual(await pixel(200/600,300/400),moved);
 await page.getByLabel('Вид кадров',{exact:true}).selectOption('diff');assert.match(await page.locator('.file-motion-dialog [role=status]').textContent(),/Отличается пикселей кадра: (?!0.00)/);await page.screenshot({path:'/tmp/tayga-motion-layers-difference.png'});
 await page.getByLabel('Сбросить триггеры',{exact:true}).click();await seek(2000);await page.getByLabel('Вид кадров',{exact:true}).selectOption('sides');assert.ok((await pixel(75/600,300/400))[1]<10);assert.ok((await pixel(200/600,300/400))[1]>220);
 await page.screenshot({path:'/tmp/tayga-motion-layers-desktop.png'});await page.setViewportSize({width:390,height:844});await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));await page.screenshot({path:'/tmp/tayga-motion-layers-mobile.png'});assert.equal(await page.locator('.file-motion-dialog').evaluate(d=>d.scrollWidth<=d.clientWidth),true);
 await page.evaluate(()=>state.email='other@test');await page.waitForSelector('.file-motion-dialog',{state:'detached'});assert.deepEqual(errors,[]);
 console.log('Advanced motion: real PDF.js mattes, parallel fades, object click, paced path, stable background, event rewind/reset, frame diff, different trigger IDs, mobile and cleanup — OK');
}finally{await browser.close()}
