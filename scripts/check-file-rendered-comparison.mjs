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
const before=pdf(['1 0 0 rg 50 50 150 100 re f\nBT /F1 24 Tf 50 300 Td (Before) Tj ET']);
const after=pdf(['0 0 1 rg 100 50 150 100 re f\nBT /F1 24 Tf 50 300 Td (After) Tj ET','0 1 0 rg 80 80 200 200 re f']);
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 let same=false;
 const page=await browser.newPage({viewport:{width:1440,height:900}}),errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.route('https://tayga.test/**',async route=>{
  const url=new URL(route.request().url());
  if(url.pathname.startsWith('/assets/pdfjs/')){const file=resolve('internal/frontend/dist',url.pathname.slice(1));await route.fulfill({body:await readFile(file),contentType:file.endsWith('.mjs')?'application/javascript':'application/octet-stream'});return}
  if(url.pathname==='/office/compare/pdf'){await route.fulfill({body:same||url.searchParams.get('side')==='left'?before:after,contentType:'application/pdf'});return}
  await route.fulfill({contentType:'text/html',body:'<html lang="ru"><body><button id="btn-files-mkdir"></button><div id="files-upload-panel"></div><ul id="files-list"></ul><div id="files-settings-menu"></div><p id="files-msg"></p></body></html>'});
 });
 await page.goto('https://tayga.test');await page.addStyleTag({path:resolve('internal/frontend/dist/assets/app.css')});
 await page.evaluate(()=>{window.$=id=>document.getElementById(id);window.state={email:'owner@test'};window.filesState={};window.mailImageText=(ru,en)=>ru;window.t=key=>key;window.designIcon=()=> '×';window.fmtBytes=String;window.openPendingFileUploads=()=>{};window.refreshFiles=async()=>{};window.api=async(path,options)=>options?{mode:'visual',left_url:'/office/compare/pdf?side=left',right_url:'/office/compare/pdf?side=right'}:{entries:[{id:'old',at:'2026-11-05',size:20}],office_compare_enabled:false,visual_compare_enabled:true}});
 await page.addScriptTag({path:resolve('frontend/src/file-library.js')});await page.evaluate(()=>openFileComparison('slides.pptx','old'));await page.getByLabel('Режим',{exact:true}).selectOption('visual');await page.locator('button[type=submit]').click();
 await page.waitForFunction(()=>document.querySelectorAll('.file-render-content canvas').length===2);
 const sizes=await page.locator('.file-render-content canvas').evaluateAll(nodes=>nodes.map(c=>[c.width,c.height]));assert.deepEqual(sizes[0],sizes[1]);
 assert.equal(await page.locator('.file-render-content canvas').first().evaluate(c=>c.getContext('2d').getImageData(90,450,1,1).data[0]),255);
 await page.getByLabel('Вид сравнения',{exact:true}).selectOption('diff');assert.equal(await page.locator('.file-render-content canvas').count(),1);assert.match(await page.locator('.file-render-dialog [role=status]').textContent(),/Отличается пикселей: (?!0.00)/);
 await page.screenshot({path:'/tmp/tayga-rendered-comparison-difference.png'});
 await page.getByLabel('Вид сравнения',{exact:true}).selectOption('overlay');assert.equal(await page.locator('.file-render-content canvas').count(),1);
 await page.getByLabel('Следующая страница / слайд',{exact:true}).click();await page.waitForFunction(()=>document.querySelector('.file-render-toolbar').textContent.includes('2 / 2'));
 await page.getByLabel('Вид сравнения',{exact:true}).selectOption('sides');assert.match(await page.locator('.file-render-content').textContent(),/страница отсутствует/);
 await page.screenshot({path:'/tmp/tayga-rendered-comparison-desktop.png'});await page.setViewportSize({width:390,height:844});await page.screenshot({path:'/tmp/tayga-rendered-comparison-mobile.png'});
 assert.equal(await page.locator('.file-render-dialog').evaluate(d=>d.scrollWidth<=d.clientWidth),true);
 await page.keyboard.press('Escape');await page.waitForSelector('.file-render-dialog',{state:'detached'});
 same=true;await page.evaluate(()=>openRenderedFileComparison('same.pdf',{left_url:'/office/compare/pdf?side=left',right_url:'/office/compare/pdf?side=right'},state.email));
 await page.getByLabel('Вид сравнения',{exact:true}).selectOption('diff');assert.match(await page.locator('.file-render-dialog [role=status]').textContent(),/Отличается пикселей: 0.00%/);
 await page.evaluate(()=>state.email='other@test');await page.waitForSelector('.file-render-dialog',{state:'detached'});assert.deepEqual(errors,[]);
 console.log('Rendered comparison: real PDF.js, matching scale, pixel diff, overlay, page navigation, absent page, mobile, cleanup — OK');
}finally{await browser.close()}
