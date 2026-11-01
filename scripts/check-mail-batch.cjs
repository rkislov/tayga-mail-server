// Isolated browser fixture; no server credentials or user mail are used.
const {chromium}=require('playwright-core');
const fs=require('fs');const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
 try {
 const page=await browser.newPage();
 const html=fs.readFileSync('internal/frontend/dist/index.html','utf8').replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi,'').replace(/<link\b[^>]*>/gi,'');
 await page.setContent(html);await page.addStyleTag({content:fs.readFileSync('internal/frontend/dist/assets/app.css','utf8')});
 const app=fs.readFileSync('frontend/src/app.js','utf8');const design=fs.readFileSync('frontend/src/design-pack.js','utf8');
 await page.addScriptTag({content:app.slice(0,app.indexOf('function askConfirm'))+`
 function t(k){return ({ok:'OK',confirm_title:'Подтвердите',delete:'Удалить'})[k]||k} function mailImageText(ru,en){return ru}
 async function askConfirm(){return true}
 const mailState={mailboxID:'inbox'};let messages=Array.from({length:49},(_,i)=>({id:'id-'+i,mailbox_id:'inbox',subject:'Письмо '+i}));
 const requests=[];let releaseBatch;let hold=true;
 async function api(path,options){const body=JSON.parse(options.body);requests.push({path,...body});if(hold){hold=false;await new Promise(resolve=>releaseBatch=resolve);}return {results:body.items.map(x=>({id:x.id,ok:x.id!=='id-1',error:x.id==='id-1'?'message_content_unavailable':''}))}}
 async function refreshMail(){messages=messages.filter(x=>x.id==='id-1');renderRows()}
 function renderRows(){$('mail-msg-list').innerHTML=messages.map(m=>'<div><button data-msg="'+m.id+'">'+m.subject+'</button></div>').join('');renderMailSelection(messages);}
 `+design.slice(design.indexOf('const selectedMailIDs='),design.indexOf('function calendarDisplayLabel'))+`
 $('view-account').classList.remove('hidden');document.querySelector('.auth-shell')?.classList.add('hidden');$('app-mail').classList.remove('hidden');renderRows();$('dialog-ok').onclick=()=>closeDialog(true);
 `});
 await page.locator('#mail-selection input').check();await page.getByRole('button',{name:'В архив',exact:true}).click();
 await page.waitForSelector('#mail-selection progress');assert.equal(await page.locator('#mail-selection').getAttribute('aria-busy'),'true');
 assert.equal(await page.getByRole('button',{name:'В архив',exact:true}).isDisabled(),true);
 await page.evaluate(()=>releaseBatch());await page.waitForSelector('#dialog-backdrop:not(.hidden)');
 assert.match(await page.locator('#dialog-body').innerText(),/48\/49/);assert.doesNotMatch(await page.locator('#dialog-body').innerText(),/\/var\//);
 const calls=await page.evaluate(()=>requests);assert.equal(calls.length,5);assert.deepEqual(calls.map(x=>x.items.length),[10,10,10,10,9]);assert(calls.every(x=>x.path==='/api/v1/mail/messages/batch'));
 assert.equal(await page.locator('.mail-select').isChecked(),true);await page.click('#dialog-ok');
 for(const width of [1280,390]){
  await page.setViewportSize({width,height:844});await page.evaluate(()=>{void askAlert('Обработано 2/49. '+('/very-long-path'.repeat(35)));});
  const box=await page.locator('#dialog-panel').evaluate(e=>({width:e.getBoundingClientRect().width,scroll:e.scrollWidth,client:e.clientWidth,bottom:e.getBoundingClientRect().bottom}));assert(box.width<=width&&box.scroll<=box.client+1&&box.bottom<=844,JSON.stringify(box));
  await page.screenshot({path:'/tmp/tayga-batch-dialog-'+width+'.png'});await page.click('#dialog-ok');
 }
 console.log('PASS: 49 messages use 5 requests; progress, partial failure, retained selection, desktop/mobile dialog bounds');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
