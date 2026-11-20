// Only a disposable loopback instance. Real uploads and posts use the demo account.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import {chromium} from 'playwright-core';
const base=process.env.TMS_URL||'http://127.0.0.1:18080';
if(!['127.0.0.1','localhost'].includes(new URL(base).hostname))throw Error('Disposable loopback server required');
const out=process.env.TMS_SHOTS||'output/ui-fixes/screenshots';await fs.mkdir(out,{recursive:true});
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:1440,height:960}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(base);await page.evaluate(()=>{localStorage.clear();localStorage.setItem('tayga.lang','ru')});await page.reload();
 await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForFunction(()=>state.me?.id);
 await page.evaluate(()=>showApp('chat'));await page.waitForSelector('.mm-chat-channel');await page.locator('.mm-chat-channel').first().click();await page.waitForSelector('#mm-chat-thread:not(.hidden)');
 for(const [id,label] of [['new','Новый диалог'],['create','Создать канал'],['browse','Открытые каналы'],['members','Участники'],['call','Звонок Jitsi']]){assert.equal(await page.locator('#mm-chat-'+id).getAttribute('aria-label'),label);assert.equal(await page.locator('#mm-chat-'+id+' svg').count(),1)}
 assert.equal(await page.locator('.mm-chat-identity>.mm-avatar').count(),1);
 await page.selectOption('#mm-chat-status','away');await page.waitForFunction(()=>document.getElementById('mm-chat-status').dataset.status==='away');
 assert.equal(await page.evaluate(async()=> (await api('/api/v4/users/me/status')).status),'away');await page.selectOption('#mm-chat-status','online');
 await page.evaluate(async()=>{await mmChat.openPeer('anna@example.com')});await page.waitForFunction(()=>document.getElementById('mm-chat-title').textContent.includes('Анна'));assert.ok(await page.locator('.mm-chat-channel .mm-avatar').count()>0);
 // An actual image and document attached to a textless post, through the browser picker.
 const png=await fs.readFile('frontend/assets/tayga-forest.jpg');
 await page.locator('#mm-chat-files').setInputFiles([{name:'Тайга.jpg',mimeType:'image/jpeg',buffer:png},{name:'Материалы.txt',mimeType:'text/plain',buffer:Buffer.from('Материалы к обсуждению интерфейса.')}]);
 assert.equal(await page.locator('.mm-pending-file').count(),2);await page.click('#mm-chat-send button[type=submit]');
 await page.waitForFunction(()=>[...document.querySelectorAll('.mm-file-image')].some(img=>img.complete&&img.naturalWidth>0));
 await page.waitForFunction(()=>document.getElementById('mm-chat-attachments').hidden);assert.ok(await page.locator('.mm-file').count()>=2);
 await page.waitForFunction(()=>!document.querySelector('#mm-chat-send button[type=submit]').disabled);assert.ok(await page.locator('.mm-file-format[data-format=TXT]').count()>0);await page.screenshot({path:out+'/chat.png'});
 // Reader grows by the full rail delta, and HTML content is one continuous scroll.
 await page.evaluate(async()=>{showApp('mail');await refreshMail();const id=document.querySelector('#mail-msg-list [data-msg]')?.dataset.msg;if(id)await openMailMessage(id)});
 await page.waitForSelector('#mail-body iframe');
 const widths=()=>page.evaluate(()=>['#app-nav','#app-mail .mail-list-pane','#app-mail .mail-read'].map(s=>document.querySelector(s).getBoundingClientRect().width));const before=await widths();await page.click('#sidebar-collapse');await page.waitForTimeout(250);const after=await widths();assert.ok(Math.abs(before[1]-after[1])<2);assert.ok(Math.abs((after[2]-before[2])-(before[0]-after[0]))<2);
 await page.evaluate(()=>{const frame=document.querySelector('#mail-body iframe');frame.srcdoc='<html><body><div style="height:2400px">Длинное письмо</div><script>parent.__mailScriptRan=true;<\/script></body></html>'});
 await page.waitForFunction(()=>document.querySelector('#mail-body iframe').getBoundingClientRect().height>=2400);assert.equal(await page.evaluate(()=>window.__mailScriptRan),undefined);
 const rect=await page.evaluate(()=>{const a=document.querySelector('#mail-body iframe').getBoundingClientRect(),b=document.querySelector('#mail-reader .workspace-reader-footer').getBoundingClientRect();return{end:a.bottom,actions:b.top}});assert.ok(rect.actions<rect.end);await page.click('#sidebar-collapse');
 await page.evaluate(async()=>{await refreshMail();await openMailMessage(document.querySelector('#mail-msg-list [data-msg]').dataset.msg)});await page.screenshot({path:out+'/mail.png'});
 await page.evaluate(async()=>{showApp('calendar');await refreshCalendar();calState.view='week';renderCalMonth();if(calState.events[0])await openCalEvent(calState.events[0].id)});
 await page.screenshot({path:out+'/calendar-week.png'});
 await page.locator('.workspace-view-tabs [data-view=day]').click();assert.equal(await page.locator('#workspace-week:visible').count(),1);assert.equal(await page.locator('#cal-day-view:visible').count(),0);assert.equal(await page.locator('[data-schedule-day]').count(),1);
 const day=page.locator('[data-schedule-day]');await day.focus();await page.keyboard.press('Enter');await page.waitForSelector('#cal-backdrop[open]');await page.keyboard.press('Escape');await page.screenshot({path:out+'/calendar-day.png'});
 await page.evaluate(async()=>{showApp('files');await refreshFiles();selectFileEntry('План релиза.txt')});assert.equal(await page.locator('.workspace-file-hero').count(),1);assert.equal(await page.locator('#files-reader .workspace-reader-more').count(),1);await page.screenshot({path:out+'/files.png'});
 await page.evaluate(()=>showApp('antispam'));await page.waitForSelector('#protection-diagnostics');
 assert.equal(await page.locator('[role=tab][aria-selected=true]').filter({hasText:'Обзор'}).count(),1);await page.screenshot({path:out+'/antispam.png'});
 // Actual mail log records were generated by the demo seed deliveries.
 await page.evaluate(async()=>{showApp('maillog');await refreshMailLog()});await page.locator('.workspace-log-open').first().click();await page.waitForSelector('.workspace-delivery-dialog[open]');await page.waitForTimeout(200);assert.ok(await page.locator('.delivery-timeline li').count()>0);assert.equal(await page.evaluate(()=>document.querySelector('.workspace-delivery-dialog').contains(document.activeElement)),true);await page.screenshot({path:out+'/delivery-status.png'});await page.keyboard.press('Escape');
 // A real saved avatar is retrieved with Bearer authorization and decoded.
 await page.evaluate(async bytes=>{const response=await fetch('/api/v1/me/avatar',{method:'PUT',headers:{Authorization:'Bearer '+state.tokens.access_token,'Content-Type':'image/jpeg'},body:new Uint8Array(bytes)});if(!response.ok)throw Error('Avatar upload failed');mmChat.stop();showApp('chat');await mmChat.refresh()},[...png]);
 await page.waitForFunction(()=>{const img=document.querySelector('.mm-chat-identity .mm-avatar img');return img?.complete&&img.naturalWidth>0});
 await page.evaluate(async()=>{await api('/api/v1/me/avatar',{method:'DELETE'});mmChat.stop()});
 assert.deepEqual(errors,[]);console.log('UI fixes: real chat attachment upload/preview, status, icon labels, avatars, mail height/security/rail expansion, day/week, files and delivery detail passed.');
}finally{await browser.close()}
