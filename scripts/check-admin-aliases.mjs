import {chromium} from 'playwright-core';
import fs from 'node:fs';
import assert from 'node:assert/strict';
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:1100,height:800}}),errors=[];
 page.on('pageerror',e=>errors.push(e.message));
 await page.route('http://localhost/**',route=>route.fulfill({body:'<!doctype html><html><body></body></html>',contentType:'text/html'}));
 await page.goto('http://localhost');
 await page.setContent('<!doctype html><html><body><button id="launch">Open</button></body></html>');
 await page.addStyleTag({path:'internal/frontend/dist/assets/app.css'});
 const helpers=fs.readFileSync('frontend/src/directory.js','utf8').split('function directoryErrorText')[0];
 await page.addScriptTag({content:`
 const state={me:{is_global_admin:true}};
 const t=k=>({close:'Закрыть',cancel:'Отмена'}[k]||k),mailImageText=ru=>ru;
 const setMsg=(el,text)=>el.textContent=text;
 const askConfirm=async()=>true;
 let aliases=[],requests=[],fail=false;
 async function api(path,options){
  if(!options){if(path.includes('?tenant_id='))return{domains:[{id:'main',name:'example.com'},{id:'other',name:'other.example'}]};return{aliases:structuredClone(aliases)}}
  const body=options.body?JSON.parse(options.body):null;requests.push({path,method:options.method,body});
  if(fail)throw Error('address already exists');
  if(options.method==='DELETE')aliases=aliases.filter(a=>!path.endsWith('/'+a.id));
  else if(options.method==='PATCH')aliases=aliases.map(a=>path.endsWith('/'+a.id)?{...a,...body,email:body.local_part+'@example.com'}:a);
  else aliases.push({id:'a'+requests.length,domain_id:body.domain_id||'main',name:body.name,local_part:body.local_part,email:body.local_part+'@'+(body.domain_id==='other'?'other.example':'example.com')});
  return{};
 }
 ${helpers}
 ${fs.readFileSync('frontend/src/admin-aliases.js','utf8')}
 const target={id:'main',name:'example.com',email:'owner@example.com',domain_id:'main',tenant_id:'tenant'};
 document.getElementById('launch').onclick=()=>openAdminAliases('domain',target);
 `});
 await page.click('#launch');
 const dialog=page.locator('dialog[open]');
 await dialog.getByLabel('Домен-алиас').fill('alias.example');
 await dialog.getByRole('button',{name:'Добавить алиас',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('.alias-row')?.textContent.includes('alias.example'));
 await dialog.getByRole('button',{name:'Изменить',exact:true}).click();
 await dialog.getByLabel('Домен-алиас').fill('renamed.example');
 await dialog.getByRole('button',{name:'Сохранить',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('.alias-row')?.textContent.includes('renamed.example'));
 await page.evaluate(()=>fail=true);
 await dialog.getByLabel('Домен-алиас').fill('example.com');
 await dialog.getByRole('button',{name:'Добавить алиас',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('dialog [role=status]').textContent.includes('уже занят'));
 assert.equal(await dialog.locator('.alias-row').count(),1);
 await page.evaluate(()=>fail=false);
 await dialog.getByRole('button',{name:'Удалить',exact:true}).click();
 await page.waitForFunction(()=>!document.querySelector('.alias-row'));
 await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();
 await page.evaluate(()=>{aliases=[];return openAdminAliases('mailbox',target)});
 await dialog.getByLabel('Имя до @').fill('sales');
 await dialog.getByLabel('Домен адреса').selectOption('other');
 await dialog.getByRole('button',{name:'Добавить алиас',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('.alias-row')?.textContent.includes('sales@other.example'));
 assert.equal(await page.evaluate(()=>requests.at(-1).body.domain_id),'other');
 await dialog.getByRole('button',{name:'Изменить',exact:true}).click();
 assert.equal(await dialog.getByLabel('Домен адреса').isDisabled(),true);
 await dialog.getByRole('button',{name:'Отмена',exact:true}).click();
 assert.equal(await dialog.getByLabel('Домен адреса').isDisabled(),false);
 await page.setViewportSize({width:390,height:844});
 assert.ok(await dialog.evaluate(el=>el.getBoundingClientRect().width<=window.innerWidth));
 assert.ok(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth));
 fs.mkdirSync('/tmp/tms-alias-qa',{recursive:true});
 await page.screenshot({path:'/tmp/tms-alias-qa/mailbox-mobile.png'});
 await dialog.getByRole('button',{name:'Закрыть',exact:true}).click();
 await page.evaluate(()=>{state.me.is_global_admin=false;return openAdminAliases('domain',target)});
 assert.equal(await dialog.locator('form').isVisible(),false);
 assert.equal(await dialog.getByRole('button',{name:'Удалить',exact:true}).count(),0);
 assert.deepEqual(errors,[]);
 console.log('Domain/mailbox alias CRUD, conflict recovery, domain selection, permissions and mobile layout — OK');
}finally{await browser.close()}
