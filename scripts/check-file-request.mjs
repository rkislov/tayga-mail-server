#!/usr/bin/env node
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const html=readFileSync('internal/httpapi/file_request.html','utf8').replaceAll('{{.Expires}}','2030-01-01T00:00:00Z'),script=readFileSync('internal/httpapi/file_request.js','utf8'),css=readFileSync('internal/httpapi/file_request.css','utf8');
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:390,height:844}}),errors=[],uploads=[];let slow=false;
 page.on('pageerror',error=>errors.push(error.message));
 await page.route('https://files.test/**',async route=>{
  const request=route.request(),url=new URL(request.url());
  if(url.pathname.endsWith('.js'))return route.fulfill({contentType:'text/javascript',body:script});
  if(url.pathname.endsWith('.css'))return route.fulfill({contentType:'text/css',body:css});
  if(request.method()==='PUT'){
   uploads.push({name:url.searchParams.get('name'),body:request.postDataBuffer()?.toString()});
   if(slow)await new Promise(resolve=>setTimeout(resolve,500));
   return route.fulfill({status:url.searchParams.get('name')==='second.txt'?507:201,contentType:'application/json',body:'{}'}).catch(()=>{});
  }
  return route.fulfill({contentType:'text/html',body:html,headers:{'Content-Security-Policy':"default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'",'Referrer-Policy':'no-referrer'}});
 });
 await page.goto('https://files.test/s/capability');
 await page.locator('#documents').setInputFiles([{name:'счёт №1.pdf',mimeType:'application/pdf',buffer:Buffer.from('first')},{name:'second.txt',mimeType:'text/plain',buffer:Buffer.from('second')}]);
 await page.locator('#upload-form').evaluate(form=>{form.requestSubmit();form.requestSubmit()});
 await page.waitForFunction(()=>document.querySelector('#status').textContent==='Принято документов: 1.');
 assert.equal(uploads.length,2);assert.deepEqual(uploads.map(x=>x.body),['first','second']);
 assert.match(await page.locator('#results').innerText(),/недостаточно свободного места/);
 assert.equal(await page.locator('#send').isEnabled(),true);
 assert.equal(await page.locator('body').evaluate(node=>node.scrollWidth<=innerWidth),true);
 assert.equal(await page.locator('main').count(),1);
 await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
 await page.screenshot({path:'/tmp/tayga-file-request-mobile.png',fullPage:true});
 await page.locator('#documents').setInputFiles({name:'<b>unsafe.txt',mimeType:'text/plain',buffer:Buffer.from('third')});
 await page.getByRole('button',{name:'Отправить документы',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('#results li')?.textContent.includes('получен'));assert.equal(await page.locator('#results b').count(),0);
 slow=true;await page.locator('#documents').setInputFiles({name:'cancel.txt',mimeType:'text/plain',buffer:Buffer.from('cancel')});
 await page.getByRole('button',{name:'Отправить документы',exact:true}).click();await page.getByRole('button',{name:'Отменить текущую отправку',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('#results').textContent.includes('отменена'));assert.equal(await page.locator('#send').isEnabled(),true);
 assert.deepEqual(errors,[]);console.log('Public file request: multiple files, one submit, quota error, progress/cancel, safe names and mobile layout — OK');
}finally{await browser.close()}
