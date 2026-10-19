import {chromium} from 'playwright-core';
import fs from 'node:fs';
import assert from 'node:assert/strict';
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage({timezoneId:'America/New_York'}),errors=[];page.on('pageerror',error=>errors.push(error.message));
 await page.setContent('<!doctype html><html lang="ru"><body><section class="card" id="travel-profile"></section></body></html>');
 await page.addStyleTag({path:'internal/frontend/dist/assets/app.css'});
 await page.addScriptTag({content:`
 const $=id=>document.getElementById(id),mailImageText=ru=>ru,t=key=>({save:'Сохранить',refresh:'Обновить',close:'Закрыть'}[key]||key);
 const state={tokens:{access_token:'owner'}},setMsg=(el,text)=>{el.textContent=text};async function askAlert(text){throw Error(text)};
 const showApp=()=>{},refreshCalendar=async()=>{},openCalEvent=async id=>{opened=id};let opened='',requests=[],fail=false,mode='off';
 const ticket={kind:'flight',number:'SU1234',origin:'Москва <img src=x onerror=alert(1)>',destination:'Дели',departure:'2026-10-25T23:30',arrival:'2026-10-26T07:00',departure_offset:'',arrival_offset:'',passenger:'Test Person',booking:'ABC123',seat:'12A',source:'ticket.pdf'};
 const items=Array.from({length:12},(_,i)=>({id:'trip'+i,ticket:{...ticket,number:'SU'+(1234+i)},state:'review',event_id:'',event_etag:''}));
 async function api(path,options){
  if(options){requests.push({path,method:options.method,body:options.body?JSON.parse(options.body):null});await new Promise(resolve=>setTimeout(resolve,40));}
  if(path.endsWith('/preferences')){if(options)mode=JSON.parse(options.body).mode;return {mode};}
  if(path.includes('?offset=')){const offset=Number(path.split('=')[1]);return {candidates:structuredClone(items.slice(offset,offset+10)),total:12};}
  if(path.endsWith('/travel'))return {candidates:[structuredClone(items[0])],warnings:[]};
  const id=path.split('/')[4],item=items.find(x=>x.id===id)||items[0];
  if(path.endsWith('/event')){if(fail)throw Error('trip or calendar event changed; reopen the review');item.state='saved';item.event_id='event1';return {event_id:'event1',calendar_id:'personal',etag:'new'};}
  if(path.endsWith('/ignore')){item.state='ignored';return {};}
  return structuredClone(item);
 }
 `+fs.readFileSync('frontend/src/travel.js','utf8')});
 await page.evaluate(()=>refreshTravelProfile());
 assert.equal(await page.locator('.travel-trip-row').count(),10);
 await page.getByRole('button',{name:'Далее',exact:true}).click();await page.waitForFunction(()=>document.querySelectorAll('.travel-trip-row').length===2);
 await page.selectOption('#travel-mode','auto');await page.getByRole('button',{name:'Сохранить',exact:true}).click();await page.waitForFunction(()=>mode==='auto');
 assert.equal(await page.evaluate(()=>requests.at(-1).body.mode),'auto');
 await page.evaluate(()=>openTravelDrafts('message'));await page.waitForSelector('dialog[open]');
 assert.equal(await page.locator('dialog img').count(),0);
 const save=page.getByRole('button',{name:'Добавить в личный календарь',exact:true});
 await save.click();assert.match(await page.locator('dialog .msg').textContent(),/оба часовых пояса/);
 assert.equal(await page.evaluate(()=>requests.filter(x=>x.path.endsWith('/event')).length),0);
 await page.locator('dialog select').nth(0).selectOption('180');await page.locator('dialog select').nth(1).selectOption('330');
 for(const width of [1440,390]){await page.setViewportSize({width,height:900});const box=await page.locator('dialog').boundingBox();assert(box.x>=0&&box.x+box.width<=width+1&&box.y>=0&&box.y+box.height<=901);assert.equal(await page.locator('dialog').evaluate(e=>e.scrollWidth>e.clientWidth+1),false);await page.screenshot({path:'/tmp/tayga-travel-'+width+'.png'});}
 await page.evaluate(()=>{fail=true;const b=[...document.querySelectorAll('dialog button')].find(b=>b.textContent==='Добавить в личный календарь');b.click();b.click();});
 await page.waitForFunction(()=>document.querySelector('dialog .msg').textContent.includes('changed'));
 assert.equal(await page.evaluate(()=>requests.filter(x=>x.path.endsWith('/event')).length),1);
 let request=await page.evaluate(()=>requests.at(-1));assert.equal(request.body.start,'2026-10-25T20:30:00.000Z');assert.equal(request.body.end,'2026-10-26T01:30:00.000Z');
 await page.evaluate(()=>fail=false);await save.click();await page.getByRole('button',{name:'Открыть событие',exact:true}).waitFor();
 assert.match(await page.locator('dialog').textContent(),/Добавлено в личный календарь/);assert.equal(await page.locator('dialog input').count(),0);
 await page.getByRole('button',{name:'Открыть событие',exact:true}).click();assert.equal(await page.evaluate(()=>opened),'event1');
 await page.evaluate(()=>openTravelCandidate('trip1'));await page.waitForSelector('dialog');await page.locator('dialog select').nth(0).selectOption('180');await page.locator('dialog select').nth(1).selectOption('330');
 const count=await page.evaluate(()=>requests.length);await page.evaluate(()=>state.tokens.access_token='other');await save.click();await page.waitForSelector('dialog',{state:'detached'});assert.equal(await page.locator('dialog').count(),0);assert.equal(await page.evaluate(()=>requests.length),count);
 assert.deepEqual(errors,[]);console.log('PASS: travel modes/pagination, review/save/retry/dedup, UTC offsets across browser zones, account isolation, escaped content, 1440/390px');
} finally {await browser.close();}
