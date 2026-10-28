import {chromium} from 'playwright-core';
import fs from 'node:fs';
import assert from 'node:assert/strict';
const app=fs.readFileSync('frontend/src/app.js','utf8'),reader=fs.readFileSync('frontend/src/reader-actions.js','utf8'),html=fs.readFileSync('internal/frontend/dist/index.html','utf8');
const section=(start,end)=>app.slice(app.indexOf(start),app.indexOf(end,app.indexOf(start)));
const editor=html.slice(html.indexOf('<div id="cal-backdrop"'),html.indexOf('<!-- CONTACTS -->'));
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage({timezoneId:'Europe/Moscow'}),errors=[];
 page.on('pageerror',error=>errors.push(error.message));
 await page.route('https://yandex.ru/**',route=>route.fulfill({body:'<html>Map fixture</html>',contentType:'text/html'}));
 await page.setContent('<!doctype html><html lang="ru" data-color-mode="light"><body><p id="cal-load-msg"></p><div id="cal-reader"><h3 id="cal-subject"></h3><dl id="cal-headers"></dl><div id="cal-read-attachments"></div></div>'+editor+'</body></html>');
 await page.addStyleTag({path:'internal/frontend/dist/assets/app.css'});
 const mapSource=fs.readFileSync('frontend/src/design-pack.js','utf8').split('function renderCalendarMap(geo){')[1].split('async function openFileAccess')[0];
 await page.addScriptTag({content:`
 const $=id=>document.getElementById(id),t=key=>({new_event:'Новое событие',save:'Сохранить',cancel:'Отмена'}[key]||key),mailImageText=ru=>ru;
 const escapeHtml=v=>String(v).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 const state={me:{id:'owner'}},calState={calendarID:'personal',calendars:[{id:'personal',display_name:'Личный календарь',writable:true}],events:[],attendees:[]};
 const setMsg=(el,text)=>{if(el)el.textContent=text};const calendarDisplayLabel=c=>c.display_name;
 let requests=[],fail=false,hold=null,loads=0;
 const master={id:'flight',calendar_id:'personal',etag:'v1',recurrence_organizer:true,summary:'Flight SU1418 · Москва → Екатеринбург',location:'Аэропорт Шереметьево, терминал B',geo:'55.9726;37.4146',start:'2026-10-23T16:10:15Z',end:'2026-10-23T18:40:15Z',description:'Маршрутная квитанция',attendees:[{email:'room@example.com',name:'Переговорная',kind:'resource'}]};
 async function api(path,options){if(!options)return structuredClone(master);requests.push({path,method:options.method,body:JSON.parse(options.body)});if(hold)await hold;if(fail)throw Error('event changed; refresh before saving');return {id:'flight'};}
 const loadCalEvents=async()=>{loads++},loadCalResourceOptions=()=>{},openCalEvent=()=>{},renderCalReadAttachments=()=>{};
 function showCalReader(record){$('cal-subject').textContent=record.summary;renderCalendarMap(record.geo)}
 function renderCalendarMap(geo){${mapSource}
 `+reader+fs.readFileSync('frontend/src/calendar-recurrence.js','utf8')+
 section('function localInputToISO(', 'async function uploadCalAttachment(')+
 section('function renderCalAttendeeChips(', 'async function loadCalFreeBusy(')+
 section('function startNewCalEvent(', 'let calInboxFocus')+
 section('$('+'"form-cal-event")?.addEventListener("submit"', '/* —— Contacts —— */')+
 `$('btn-cal-close').onclick=()=>showCalCompose(false);$('btn-cal-attendee-add').onclick=addCalAttendeesFromInput;`});
 await page.evaluate(()=>openEntityModal('calendar','flight'));
 assert.equal(await page.locator('#entity-reader-dialog button').filter({hasText:'Редактировать'}).count(),1);
 const map=await page.locator('#entity-reader-dialog iframe').getAttribute('src');
 assert.match(map,/ll=37.4146%2C55.9726/);assert.match(map,/pt=37.4146%2C55.9726%2Cpm2rdm/);
 await page.locator('#entity-reader-dialog button').filter({hasText:'Редактировать'}).click();
 await page.waitForSelector('#cal-backdrop[open]');
 assert.equal(await page.inputValue('#cal-summary'),'Flight SU1418 · Москва → Екатеринбург');
 assert.equal(await page.inputValue('#cal-start'),'2026-10-23T19:10:15');
 assert.equal(await page.locator('#event-calendar-choice').isDisabled(),true);
 assert.equal(await page.locator('#cal-location').getAttribute('data-geo'),'55.9726;37.4146');
 assert.match(await page.locator('#cal-attendee-chips').textContent(),/Переговорная/);
 for(const width of [1440,390]) {
  await page.setViewportSize({width,height:900});
  const box=await page.locator('#cal-backdrop').boundingBox();assert(box.x>=0&&box.x+box.width<=width+1&&box.y>=0&&box.y+box.height<=901);
  assert.equal(await page.locator('#cal-backdrop').evaluate(el=>el.scrollWidth>el.clientWidth+1),false);
  await page.screenshot({path:'/tmp/tayga-calendar-edit-'+width+'.png',animations:'disabled'});
 }
 await page.fill('#cal-attendee-input','guest@example.com');await page.click('#btn-cal-attendee-add');
 await page.evaluate(()=>{fail=true;hold=new Promise(resolve=>window.release=resolve);$('form-cal-event').requestSubmit();$('form-cal-event').requestSubmit()});
 await page.keyboard.press('Escape');assert.equal(await page.locator('#cal-backdrop').evaluate(el=>el.open),true);
 await page.evaluate(()=>{release();hold=null});await page.waitForFunction(()=>$('cal-msg').textContent.includes('event changed'));
 assert.equal(await page.evaluate(()=>requests.length),1);assert.equal(await page.locator('#form-cal-event [type=submit]').isDisabled(),false);
 let req=await page.evaluate(()=>requests[0]);assert.equal(req.method,'PUT');assert.equal(req.body.id,'flight');assert.equal(req.body.etag,'v1');assert.equal(req.body.geo,'55.9726;37.4146');assert.equal(req.body.start,'2026-10-23T16:10:15.000Z');assert.equal(req.body.recurrence,undefined);assert.equal(req.body.attendees.length,2);
 await page.evaluate(()=>{fail=false;$('form-cal-event').requestSubmit()});await page.waitForFunction(()=>!$('cal-backdrop').open);assert.equal(await page.evaluate(()=>loads),1);
 // All-day editing stays DATE based and retains the exclusive end date.
 await page.evaluate(()=>{master.all_day=true;master.start='2026-10-23T00:00:00Z';master.end='2026-10-24T00:00:00Z';editCalendarRecord(master)});
 await page.waitForSelector('#cal-backdrop[open]');assert.equal(await page.inputValue('#cal-start'),'2026-10-23');
 await page.evaluate(()=>$('form-cal-event').requestSubmit());await page.waitForFunction(()=>!$('cal-backdrop').open);
 req=await page.evaluate(()=>requests.at(-1));assert.equal(req.body.all_day,true);assert.equal(req.body.end,'2026-10-24T00:00:00.000Z');
 // A new event after an edit must create, with clean fields and editable calendar.
 await page.evaluate(()=>startNewCalEvent('2026-10-25',10));await page.fill('#cal-summary','Новое событие');
 assert.equal(await page.locator('#cal-start').getAttribute('type'),'datetime-local');assert.equal(await page.locator('#event-calendar-choice').isDisabled(),false);assert.equal(await page.locator('#cal-location').getAttribute('data-geo'),null);
 await page.evaluate(()=>$('form-cal-event').requestSubmit());await page.waitForFunction(()=>!$('cal-backdrop').open);
 req=await page.evaluate(()=>requests.at(-1));assert.equal(req.method,'POST');assert.equal(req.body.id,undefined);
 // A received meeting is read-only, and switching accounts cannot submit edits.
 await page.evaluate(()=>{master.recurrence_organizer=false;editCalendarRecord(master)});assert.equal(await page.locator('#cal-backdrop').evaluate(el=>el.open),false);
 await page.evaluate(()=>{master.recurrence_organizer=true;editCalendarRecord(master)});await page.waitForSelector('#cal-backdrop[open]');
 const count=await page.evaluate(()=>requests.length);await page.evaluate(()=>{state.me.id='other';$('form-cal-event').requestSubmit()});assert.equal(await page.evaluate(()=>requests.length),count);
 assert.deepEqual(errors,[]);
 console.log('PASS: calendar edit/modal/map, PUT/ETag, attendees, seconds/all-day, conflict/double-submit, new-event reset, account/permission checks, 1440/390 px');
} finally {await browser.close();}
