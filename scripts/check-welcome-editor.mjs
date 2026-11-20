import {chromium} from 'playwright-core';
import fs from 'node:fs';
import assert from 'node:assert/strict';
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage({viewport:{width:1200,height:900}}),errors=[];
 page.on('pageerror',e=>errors.push(e.message));
 await page.route('http://localhost/**',route=>{
  const url=new URL(route.request().url());
  if(url.pathname.startsWith('/assets/ckeditor5/'))return route.fulfill({body:fs.readFileSync('internal/frontend/dist'+url.pathname),contentType:url.pathname.endsWith('.css')?'text/css':'text/javascript'});
  return route.fulfill({body:'<!doctype html><html><body></body></html>',contentType:'text/html'});});
 await page.goto('http://localhost');
 await page.setContent('<!doctype html><html><body></body></html>');
 await page.addStyleTag({path:'internal/frontend/dist/assets/app.css'});
 await page.addStyleTag({path:'internal/frontend/dist/assets/ckeditor5/ckeditor5.css'});
 const helpers=fs.readFileSync('frontend/src/directory.js','utf8').split('function directoryErrorText')[0];
 await page.addScriptTag({content:`
 const $=id=>document.getElementById(id),mailImageText=ru=>ru,t=k=>({close:'Закрыть'}[k]||k),setMsg=(el,text)=>el.textContent=text,askConfirm=async()=>true;
 let server={kind:'tenants',scope_id:'tenant',generation:0,mode:'off',published:0,revisions:[],events:[]},requests=[],fail=false,loadFailure=false,inherited=null;
 async function api(path,options){
  if(!options){if(loadFailure)throw Error('Network unavailable');return {template:structuredClone(server),effective:server.mode==='own'?structuredClone(server.revisions.find(v=>v.number===server.published)):server.mode==='inherit'?structuredClone(inherited):null,source:'tenants'}}
  const c=JSON.parse(options.body);requests.push(c);
  if(c.action==='preview'){
   const parsed=new DOMParser().parseFromString(c.content.html,'text/html');parsed.querySelectorAll('script,iframe,style').forEach(el=>el.remove());
   return {...c.content,html:parsed.body.innerHTML,text:parsed.body.textContent};
  }
  if(fail||c.expected!==server.generation)throw Error('welcome template changed; reload before saving');
  if(c.action==='save'||c.action==='restore'){
   const v=c.action==='save'?c.content:{...server.revisions.find(v=>v.number===c.revision),restored_from:c.revision,comment:c.content.comment};
   server.revisions.push({...v,number:server.revisions.length+1,author:'admin@example.com',saved_at:new Date().toISOString()});
  }
  if(c.action==='publish'){server.published=c.revision;server.mode='own'}
  if(c.action==='mode')server.mode=c.mode;
  server.generation++;server.events.push({action:c.action,revision:c.revision||server.revisions.length,actor:'admin@example.com',mode:server.mode,at:new Date().toISOString()});
  return structuredClone(server);
 }
 ${helpers}
 ${fs.readFileSync('frontend/src/rich-mail-editor.js','utf8')}
 ${fs.readFileSync('frontend/src/admin-welcome.js','utf8')}
 openAdminWelcome('tenants',{id:'tenant',name:'Организация'});
 `});
 const dialog=page.locator('dialog[open]');
 await dialog.getByLabel('Тема письма',{exact:true}).fill('Welcome first');
 await page.waitForFunction(()=>document.querySelector('.welcome-rich-host .ck-editor__editable_root') || !document.querySelector('.welcome-source').hidden);
 assert.equal(await dialog.locator('[role=status]').textContent(),'');
 const body=page.locator('.welcome-rich-host .ck-editor__editable_root');
 await body.fill('First visual body');
 await body.press('ControlOrMeta+a');
 await dialog.getByRole('button',{name:'Ссылка',exact:true}).click();
 const linkURL=dialog.getByRole('textbox',{name:'Ссылка URL',exact:true});
 await linkURL.fill('https://example.com/welcome');
 await linkURL.press('Enter');
 await page.waitForFunction(()=>document.querySelector('.welcome-rich-host .ck-editor__editable_root a')?.getAttribute('href')==='https://example.com/welcome');
 await dialog.getByRole('button',{name:'Предпросмотр',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('.welcome-text').textContent.includes('First visual body'));
 await dialog.getByRole('button',{name:'HTML',exact:true}).click();
 assert.match(await dialog.getByLabel('HTML письма',{exact:true}).inputValue(),/First visual body/);
 await dialog.getByLabel('HTML письма',{exact:true}).fill('<h1>First {{name}}</h1><p>Version one</p>');
 await dialog.getByRole('button',{name:'Визуальный редактор',exact:true}).click();
 await body.waitFor();await page.waitForFunction(()=>document.querySelector('.welcome-rich-host .ck-editor__editable_root')?.textContent.includes('Version one'));
 const photo=fs.readFileSync('docs/screenshots/ui-notes-tayga.png');
 await dialog.locator('.ck-file-dialog-button input[type=file]').setInputFiles({name:'team.png',mimeType:'image/png',buffer:photo});
 await page.waitForFunction(()=>document.querySelector('.welcome-rich-host img')?.src.startsWith('data:image/'));
 await dialog.getByLabel('Комментарий к версии',{exact:true}).fill('Initial');
 await dialog.getByRole('button',{name:'Сохранить новую версию',exact:true}).click();
 await page.waitForFunction(()=>server.revisions.length===1);
 await dialog.screenshot({path:'/tmp/tms-welcome-visual.png'});
 assert.match(await page.evaluate(()=>server.revisions[0].html),/data:image\/(png|jpeg);base64,/);
 assert.equal(await page.evaluate(()=>server.published),0);
 await dialog.getByRole('button',{name:'Опубликовать выбранную',exact:true}).click();
 await page.waitForFunction(()=>server.published===1);
 await dialog.getByRole('button',{name:'HTML',exact:true}).click();
 await dialog.getByLabel('Тема письма',{exact:true}).fill('Welcome second');
 await dialog.getByLabel('HTML письма',{exact:true}).fill('<p>Second version</p>');
 await dialog.getByRole('button',{name:'Сохранить новую версию',exact:true}).click();
 await page.waitForFunction(()=>server.revisions.length===2);
 assert.equal(await page.evaluate(()=>server.published),1);
 await dialog.getByLabel('Версия письма',{exact:true}).selectOption('1');
 await dialog.getByRole('button',{name:'Восстановить как новую версию',exact:true}).click();
 await page.waitForFunction(()=>server.revisions.length===3);
 assert.equal(await page.evaluate(()=>server.revisions[2].restored_from),1);
 assert.match(await page.evaluate(()=>server.revisions[2].html),/data:image\/(png|jpeg);base64,/);
 assert.equal(await dialog.getByLabel('Тема письма',{exact:true}).inputValue(),'Welcome first');
 assert.equal(await page.evaluate(()=>server.published),1);
 await dialog.getByRole('button',{name:'Опубликовать выбранную',exact:true}).click();
 await page.waitForFunction(()=>server.published===3);
 await dialog.getByLabel('Тема письма',{exact:true}).fill('Local conflict text');
 await page.evaluate(()=>fail=true);
 await dialog.getByRole('button',{name:'Сохранить новую версию',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('dialog [role=status]').textContent.includes('другим администратором'));
 assert.equal(await dialog.getByLabel('Тема письма',{exact:true}).inputValue(),'Local conflict text');
 await page.evaluate(()=>{fail=false;server.generation++});
 await dialog.getByRole('button',{name:'Обновить историю',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('dialog [role=status]').textContent.includes('История обновлена'));
 assert.equal(await dialog.getByLabel('Тема письма',{exact:true}).inputValue(),'Local conflict text');
 await dialog.getByRole('button',{name:'Сохранить новую версию',exact:true}).click();
 await page.waitForFunction(()=>server.revisions.length===4);
 assert.equal(await page.evaluate(()=>requests.at(-1).expected),6);
 await dialog.getByLabel('Режим шаблона',{exact:true}).selectOption('off');
 await dialog.getByRole('button',{name:'Применить режим',exact:true}).click();
 await page.waitForFunction(()=>server.mode==='off');
 await page.setViewportSize({width:390,height:844});
 assert.ok(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1),'mobile overflow');
 await dialog.screenshot({path:'/tmp/tms-welcome-mobile.png'});
 await page.setViewportSize({width:1200,height:900});
 await dialog.screenshot({path:'/tmp/tms-welcome-desktop.png'});
 await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();
 await page.evaluate(()=>{
   inherited={number:7,subject:'Tenant published letter',html:'<p>Inherited body</p>',text:'Inherited body'};
   server={kind:'domains',scope_id:'domain',generation:0,mode:'inherit',published:0,revisions:[],events:[]};
   return openAdminWelcome('domains',{id:'domain',name:'example.com'});
 });
 assert.equal(await dialog.getByLabel('Тема письма',{exact:true}).inputValue(),'Tenant published letter');
 assert.equal(await dialog.getByLabel('Режим шаблона',{exact:true}).inputValue(),'inherit');
 assert.equal(await dialog.getByRole('button',{name:'Опубликовать выбранную',exact:true}).isDisabled(),true);
 await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();
 await page.evaluate(()=>{loadFailure=true;return openAdminWelcome('domains',{id:'domain',name:'example.com'})});
 assert.equal(await dialog.getByRole('button',{name:'Закрыть',exact:true}).isEnabled(),true);
 assert.equal(await dialog.getByRole('button',{name:'Обновить историю',exact:true}).isEnabled(),true);
 await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();
 assert.deepEqual(errors,[]);
 console.log('CKEditor welcome: local photos, visual/source, preview, versions, publish, restore, conflicts, recovery and mobile — OK');
} finally {await browser.close()}
