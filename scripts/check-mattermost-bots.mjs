import assert from 'node:assert/strict';
import {chromium} from 'playwright-core';
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:1360,height:900}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 for(const name of ['app.js','app.css'])await page.route('**/assets/'+name+'*',r=>r.fulfill({path:'internal/frontend/dist/assets/'+name}));
 await page.goto('http://127.0.0.1:18080');await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForFunction(()=>state.me?.id);
 await page.evaluate(()=>showApp('mattermost'));await page.waitForFunction(()=>mmAdmin.scopes.length>0);
 const index=await page.evaluate(()=>mmAdmin.scopes.findIndex(s=>s.scope.domain_id));assert.ok(index>=0);await page.selectOption('#mm-scope',String(index));
 await page.locator('.workspace-mm-tabs button').filter({hasText:'Боты'}).click();await page.waitForSelector('#mm-bot-create:visible');
 const username='test-bot-'+Date.now();await page.fill('#mm-bot-create [name=username]',username);await page.fill('#mm-bot-create [name=display_name]','Уведомления проекта');await page.click('#mm-bot-create button');await page.waitForSelector('#mm-bot-secret[open]');
 const token=await page.inputValue('#mm-bot-secret input');assert.ok(token.startsWith('tmsbot_'));
 const id=await page.locator('[data-bot-id]').textContent();await page.click('#mm-bot-secret form button');await page.waitForFunction(()=>!document.querySelector('#mm-bot-secret input').value);
 // Connect the actual external credential and add the account to a private demo channel.
 await page.evaluate(async({token,id})=>{
  const request=async(path,body)=>{const r=await fetch('/api/v4'+path,{method:body?'POST':'GET',headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});if(!r.ok)throw Error('Bot request '+r.status);return r.json()};
  const me=await request('/users/me');if(!me.is_bot)throw Error('Not a bot');
  const teams=await api('/api/v4/users/me/teams');const channel=await api('/api/v4/channels',{method:'POST',body:JSON.stringify({team_id:teams[0].id,name:'bots-'+Date.now(),display_name:'Обновления проекта',type:'P'})});
  await api('/api/v4/channels/'+channel.id+'/members',{method:'POST',body:JSON.stringify({user_id:id})});
  await request('/posts',{channel_id:channel.id,message:'Сборка завершена. Мобильный чат готов к проверке.'});
 },{token,id});
 assert.equal(await page.locator('#mm-policy-form').isVisible(),false);
 await page.screenshot({path:'output/ui-fixes/mobile/bots.png'});
 const row=page.locator('.mm-bot-row').filter({hasText:username});await row.getByRole('button',{name:'Отключить',exact:true}).click();await page.waitForFunction(name=>[...document.querySelectorAll('.mm-bot-row')].some(r=>r.textContent.includes(name)&&r.textContent.includes('Выключен')),username);
 assert.equal(await page.evaluate(async token=>(await fetch('/api/v4/users/me',{headers:{Authorization:'Bearer '+token}})).status,token),401);
 assert.deepEqual(errors,[]);console.log('Bots UI: domain scope, creation, one-time credential, actual bot post, disable/revocation passed.');
}finally{await browser.close()}
