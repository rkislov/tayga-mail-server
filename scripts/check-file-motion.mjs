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
const before=pdf(['','1 0 0 rg 50 50 150 100 re f','0 0 1 rg 50 50 150 100 re f','0 1 0 rg 50 50 150 100 re f']);
const after=before;
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:1440,height:900}}),errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.route('https://tayga.test/**',async route=>{
  const url=new URL(route.request().url());
  if(url.pathname.startsWith('/assets/pdfjs/')){const file=resolve('internal/frontend/dist',url.pathname.slice(1));await route.fulfill({body:await readFile(file),contentType:file.endsWith('.mjs')?'application/javascript':'application/octet-stream'});return}
  if(url.pathname==='/office/compare/pdf'){await route.fulfill({body:before,contentType:'application/pdf'});return}
  if(url.pathname==='/office/compare/timeline'){await route.fulfill({contentType:'application/json',body:JSON.stringify({frames:4,slides:[{states:[1,2],effects:[{kind:'fade',target:'2',duration:url.searchParams.get('side')==='left'?1000:2000,trigger:'click',delay:0}],transition:{kind:'cut',duration:0,direction:''}},{states:[3],effects:[],transition:{kind:'push',direction:'l',duration:1000}},{states:[4],effects:[],transition:{kind:'cut',duration:0},warning:'Example unsupported <img src=x onerror="window.injected=1">'}]})});return}
  await route.fulfill({contentType:'text/html',body:'<html lang="ru"><body><button id="btn-files-mkdir"></button><div id="files-upload-panel"></div><ul id="files-list"></ul><div id="files-settings-menu"></div><p id="files-msg"></p></body></html>'});
 });
 await page.goto('https://tayga.test');await page.addStyleTag({path:resolve('internal/frontend/dist/assets/app.css')});
 await page.evaluate(()=>{window.$=id=>document.getElementById(id);window.state={email:'owner@test'};window.filesState={};window.mailImageText=(ru,en)=>ru;window.t=key=>key;window.designIcon=()=> '×';window.fmtBytes=String;window.openPendingFileUploads=()=>{};window.refreshFiles=async()=>{};window.api=async(path,options)=>options?{mode:'motion',left_timeline:'/office/compare/timeline?side=left',right_timeline:'/office/compare/timeline?side=right',left_url:'/office/compare/pdf?side=left&storyboard=1',right_url:'/office/compare/pdf?side=right&storyboard=1'}:{entries:[{id:'old',at:'2026-11-05',size:20}],visual_compare_enabled:true}});
 await page.addScriptTag({path:resolve('frontend/src/file-library.js')});await page.addScriptTag({path:resolve('frontend/src/file-motion.js')});await page.evaluate(()=>openFileComparison('slides.pptx','old'));await page.getByLabel('Режим',{exact:true}).selectOption('motion');await page.locator('button[type=submit]').click();
 await page.waitForFunction(()=>document.querySelectorAll('.file-motion-dialog .file-render-content canvas').length===2);
 const seek=async value=>{await page.getByLabel('Время кадра',{exact:true}).evaluate((node,value)=>{node.value=String(value);node.dispatchEvent(new Event('input'))},value)};
 await seek(1000);
 const pixels=await page.locator('.file-motion-dialog .file-render-content canvas').evaluateAll(nodes=>nodes.map(c=>Array.from(c.getContext('2d').getImageData(100,400,1,1).data)));
 assert.ok(pixels[0][1]<pixels[1][1],JSON.stringify(pixels));assert.equal(pixels[0][0],255);
 await page.getByLabel('Вид кадров',{exact:true}).selectOption('diff');assert.match(await page.locator('.file-motion-dialog [role=status]').textContent(),/Отличается пикселей кадра: (?!0.00)/);
 await page.screenshot({path:'/tmp/tayga-motion-difference.png'});
 await page.getByLabel('Следующий кадр',{exact:true}).click();assert.equal(await page.getByLabel('Время кадра',{exact:true}).inputValue(),'1040');
 await page.getByLabel('Вид кадров',{exact:true}).selectOption('overlay');assert.equal(await page.locator('.file-motion-dialog .file-render-content canvas').count(),1);
 await seek(0);await page.getByLabel('Воспроизвести',{exact:true}).click();await page.waitForFunction(()=>Number(document.querySelector('.file-motion-scrub').value)>200);await page.getByLabel('Пауза',{exact:true}).click();const paused=await page.getByLabel('Время кадра',{exact:true}).inputValue();await page.waitForTimeout(180);assert.equal(await page.getByLabel('Время кадра',{exact:true}).inputValue(),paused);
 await page.getByLabel('Следующий слайд',{exact:true}).click();await page.waitForFunction(()=>document.querySelector('.file-motion-dialog input[type=number]').value==='2'&&!document.querySelector('.file-motion-scrub').disabled);
 await seek(500);await page.getByLabel('Вид кадров',{exact:true}).selectOption('sides');
 const transitionPixel=await page.locator('.file-motion-dialog .file-render-content canvas').first().evaluate(c=>Array.from(c.getContext('2d').getImageData(500,400,1,1).data));assert.deepEqual(transitionPixel,[0,0,255,255]);
 await page.getByLabel('Следующий слайд',{exact:true}).click();await page.waitForFunction(()=>document.querySelector('.file-motion-dialog input[type=number]').value==='3'&&!document.querySelector('.file-motion-scrub').disabled);
 assert.match(await page.locator('.file-motion-warning').textContent(),/неподдерживаемая/);assert.equal(await page.locator('.file-motion-warning img').count(),0);assert.equal(await page.evaluate(()=>window.injected),undefined);
 await seek(500);await page.getByLabel('Вид кадров',{exact:true}).selectOption('sides');
 await page.screenshot({path:'/tmp/tayga-motion-desktop.png'});await page.setViewportSize({width:390,height:844});await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));assert.equal(await page.locator('.file-motion-dialog').count(),1);await page.screenshot({path:'/tmp/tayga-motion-mobile.png'});assert.equal(await page.locator('.file-motion-dialog').evaluate(d=>d.scrollWidth<=d.clientWidth),true);
 await page.evaluate(()=>state.email='other@test');await page.waitForSelector('.file-motion-dialog',{state:'detached'});assert.deepEqual(errors,[]);
 console.log('Motion comparison: real PDF.js key states, fade timing differences, stepping, overlay/diff, playback/pause, transition navigation, warning escaping, mobile, account cleanup — OK');
}finally{await browser.close()}
