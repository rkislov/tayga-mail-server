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
const rect=color=>`${color} rg 50 50 100 80 re f 0 0 0 rg 95 85 10 10 re f`;
const matte=(color,shape='')=>`${color} rg 0 0 600 400 re f ${shape}`;
const background=matte('0.9 0.9 0.9','0 0 1 rg 5 5 20 20 re f');
const fixture=pdf([background,background,background,background,background,matte('0 0 0'),matte('1 1 1'),matte('0 0 0',rect('1 0 0')),matte('1 1 1',rect('1 0 0')),matte('0 0 0','0 1 0 rg 250 50 100 80 re f'),matte('1 1 1','0 1 0 rg 250 50 100 80 re f'),matte('0 0 0',rect('1 0 0')),matte('1 1 1',rect('1 0 0')),matte('0 0 0',rect('0 0 1')),matte('1 1 1',rect('0 0 1')),matte('1 1 1')]);
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:1440,height:900}}),errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.route('https://tayga.test/**',async route=>{
  const url=new URL(route.request().url()),right=url.searchParams.get('side')==='right';
  if(url.pathname.startsWith('/assets/pdfjs/')){const file=resolve('internal/frontend/dist',url.pathname.slice(1));await route.fulfill({body:await readFile(file),contentType:file.endsWith('.mjs')?'application/javascript':'application/octet-stream'});return}
  if(url.pathname==='/office/compare/pdf'){await route.fulfill({body:fixture,contentType:'application/pdf'});return}
  if(url.pathname==='/office/compare/timeline'){await route.fulfill({contentType:'application/json',body:JSON.stringify({frames:16,slides:[{states:[1,2,3,4,5],background:5,probe:[6,7],layers:[{target:'2',black:8,white:9,hidden:false},{target:'3',black:10,white:11,hidden:true}],colors:[{effect:2,from_black:12,from_white:13,to_black:14,to_white:15}],hits:[{target:'2',name:'Transform object',box:[50/600,270/400,100/600,80/400]}],effects:[{kind:'rotate',target:'2',duration:1000,trigger:'click',delay:0,from:[0],to:[right?0:90]},{kind:'scale',target:'2',duration:1000,trigger:'with',delay:0,from:[1,1],to:[2,.5]},{kind:'color',target:'2',duration:1000,trigger:'with',delay:0,from:[255,0,0],to:[0,0,255],property:'fill'},{kind:'fade',target:'3',exit:false,duration:500,trigger:'click',event:'2',delay:0}],transition:{kind:'cut',duration:0}},{states:[16],effects:[],transition:{kind:'fade',duration:1000}}]})});return}
  await route.fulfill({contentType:'text/html',body:'<html lang="ru"><body><button id="btn-files-mkdir"></button><div id="files-upload-panel"></div><ul id="files-list"></ul><div id="files-settings-menu"></div><p id="files-msg"></p></body></html>'});
 });
 await page.goto('https://tayga.test');await page.addStyleTag({path:resolve('internal/frontend/dist/assets/app.css')});
 await page.evaluate(()=>{window.$=id=>document.getElementById(id);window.state={email:'owner@test'};window.filesState={};window.mailImageText=(ru,en)=>ru;window.t=key=>key;window.designIcon=()=> '×';window.fmtBytes=String;window.openPendingFileUploads=()=>{};window.refreshFiles=async()=>{};window.api=async(path,options)=>options?{mode:'motion',left_timeline:'/office/compare/timeline?side=left',right_timeline:'/office/compare/timeline?side=right',left_url:'/office/compare/pdf?side=left&storyboard=1',right_url:'/office/compare/pdf?side=right&storyboard=1'}:{entries:[{id:'old',at:'2026-11-05',size:20}],visual_compare_enabled:true}});
 await page.addScriptTag({path:resolve('frontend/src/file-library.js')});await page.addScriptTag({path:resolve('frontend/src/file-motion.js')});await page.evaluate(()=>openFileComparison('transform.pptx','old'));await page.getByLabel('Режим',{exact:true}).selectOption('motion');await page.locator('button[type=submit]').click();
 await page.waitForFunction(()=>document.querySelectorAll('.file-motion-dialog .file-render-content canvas').length===2);
 const seek=async value=>{await page.getByLabel('Время кадра',{exact:true}).evaluate((node,value)=>{node.value=String(value);node.dispatchEvent(new Event('input'))},value)};
 const pixel=async(x,y,side=0)=>page.locator('.file-motion-dialog .file-render-content canvas').nth(side).evaluate((c,{x,y})=>Array.from(c.getContext('2d').getImageData(Math.floor(c.width*x/600),Math.floor(c.height*y/400),1,1).data),{x,y});
 await seek(1000);const purple=await pixel(121,331);assert.ok(purple[0]>110&&purple[0]<145&&purple[1]<10&&purple[2]>110&&purple[2]<145,JSON.stringify(purple));assert.deepEqual(await pixel(100,310),[0,0,0,255]);
 await seek(1500);assert.deepEqual(await pixel(100,240),[0,0,255,255]);assert.deepEqual(await pixel(100,240,1),[230,230,230,255]);assert.deepEqual(await pixel(15,385),[0,0,255,255]);
 await page.getByLabel('Вид кадров',{exact:true}).selectOption('diff');assert.match(await page.locator('.file-motion-dialog [role=status]').textContent(),/Отличается пикселей кадра: (?!0.00)/);await page.screenshot({path:'/tmp/tayga-motion-transform-difference.png'});
 await page.getByLabel('Вид кадров',{exact:true}).selectOption('sides');const canvas=page.locator('.file-motion-dialog .file-render-content canvas').first();await canvas.click({position:await canvas.evaluate(c=>({x:c.clientWidth*100/600,y:c.clientHeight*240/400}))});await page.getByLabel('Пауза',{exact:true}).click();await seek(2100);assert.deepEqual(await pixel(300,310),[0,255,0,255]);
 await page.getByLabel('Следующий слайд',{exact:true}).click();await page.waitForFunction(()=>document.querySelector('.file-motion-dialog input[type=number]').value==='2'&&!document.querySelector('.file-motion-scrub').disabled);await seek(500);const previousEnd=await pixel(100,240);assert.ok(previousEnd[0]>115&&previousEnd[0]<140&&previousEnd[1]>115&&previousEnd[1]<140&&previousEnd[2]===255,JSON.stringify(previousEnd));
 await page.getByLabel('Предыдущий слайд',{exact:true}).click();await page.waitForFunction(()=>document.querySelector('.file-motion-dialog input[type=number]').value==='1'&&!document.querySelector('.file-motion-scrub').disabled);await seek(1000);await page.screenshot({path:'/tmp/tayga-motion-transform-desktop.png'});await page.setViewportSize({width:390,height:844});await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));await page.screenshot({path:'/tmp/tayga-motion-transform-mobile.png'});assert.equal(await page.locator('.file-motion-dialog').evaluate(d=>d.scrollWidth<=d.clientWidth),true);
 await page.evaluate(()=>state.email='other@test');await page.waitForSelector('.file-motion-dialog',{state:'detached'});assert.deepEqual(errors,[]);
 console.log('Transform playback: real PDF.js, RGB fill interpolation, preserved black detail, rotation/scale composition, transformed click hit, previous final pose in transition, frame diff, mobile and cleanup — OK');
}finally{await browser.close()}
