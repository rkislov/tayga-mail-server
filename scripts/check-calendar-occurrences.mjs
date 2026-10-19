// Exercise the shipped occurrence editor, immutable identity, retries and account changes.
import { chromium } from 'playwright-core';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
const root=new URL('../',import.meta.url);
const app=await readFile(new URL('frontend/src/app.js',root),'utf8');
const css=await readFile(new URL('internal/frontend/dist/assets/app.css',root),'utf8');
const helpers=app.slice(app.indexOf('function localInputToISO('),app.indexOf('function showCalCompose('));
const controller=app.slice(app.indexOf('let calOccurrenceBusy ='),app.indexOf('async function openCalEvent('));
const recurrenceSource=await readFile(new URL('frontend/src/calendar-recurrence.js',root),'utf8');
const bookingErrorHelper=recurrenceSource.slice(recurrenceSource.indexOf('function calendarRecurrenceError('),recurrenceSource.indexOf("$('btn-cal-recurrence')"));
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage({timezoneId:'Europe/Moscow'});
 const errors=[];page.on('pageerror',err=>errors.push(err.message));
 await page.setContent('<p id="cal-load-msg"></p>');
 await page.addStyleTag({content:css});
 await page.addScriptTag({content:`
  const $=id=>document.getElementById(id);
  const t=key=>({cal_instance_edit:'Изменить повторение',col_summary:'Тема',col_start:'Начало',col_end:'Конец',col_location:'Место',col_description:'Описание',save:'Сохранить',cancel:'Отмена'}[key] || key);
  const escapeHtml=value=>String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const mailImageText=ru=>ru;
  const fmtCalWhen=value=>new Date(value).toLocaleString('ru-RU');
  const setMsg=(el,text)=>{if(el)el.textContent=text;};
  const state={me:{id:'owner'}};
  const original={id:'object@2026-10-26T10:00:00Z',object_id:'object',recurrence_id:'2026-10-26T10:00:00Z',etag:'"series-v1"',occurrence_organizer:true,summary:'Планирование следующего этапа',start:'2026-10-26T10:00:00Z',end:'2026-10-26T11:00:00Z',location:'Офис',description:'План и обсуждение',geo:'55.75;37.61'};
  const calState={currentEvent:{...original}};
  window.requests=[];window.fail=false;window.confirmed=true;window.switchUser=false;window.confirmations=[];
  const loadCalEvents=async()=>{};
  const askConfirm=async text=>{window.confirmations.push(text);if(window.switchUser)state.me={id:'another'};return window.confirmed;};
  const api=async(path,opts)=>{const body=opts.body ? JSON.parse(opts.body) : {};window.requests.push({path,method:opts.method,body});await new Promise(r=>setTimeout(r,20));if(window.fail)throw new Error('event changed; refresh before saving');return {...original,...body};};
 `+helpers+bookingErrorHelper+controller});
 await page.evaluate(()=>openCalOccurrenceEditor());
 assert.equal(await page.inputValue('#cal-occ-start'),'2026-10-26T13:00');
 for(const width of [1440,390]) {
  await page.setViewportSize({width,height:900});
  const bounds=await page.locator('#cal-occurrence-dialog').boundingBox();
  assert.ok(bounds.x>=0 && bounds.x+bounds.width<=width+1 && bounds.y>=0 && bounds.y+bounds.height<=901,'dialog overflow');
  assert.equal(await page.evaluate(()=>$('cal-occurrence-dialog').scrollWidth>$('cal-occurrence-dialog').clientWidth+1),false);
  await page.screenshot({path:'/tmp/tayga-calendar-occurrence-'+width+'.png'});
 }
 await page.fill('#cal-occ-start','2026-10-27T15:00');
 await page.fill('#cal-occ-end','2026-10-27T16:30');
 await page.fill('#cal-occ-location','Другой офис');
 await page.evaluate(()=>{window.fail=true;$('form-cal-occurrence').requestSubmit();$('form-cal-occurrence').requestSubmit();});
 await page.waitForFunction(()=>$('cal-occ-msg')?.textContent.includes('event changed'));
 assert.equal(await page.evaluate(()=>window.requests.length),1);
 let request=await page.evaluate(()=>window.requests[0]);
 assert.equal(request.path,'/api/v1/calendar/events/object/occurrences');
 assert.equal(request.body.recurrence_id,'2026-10-26T10:00:00Z');
 assert.equal(request.body.etag,'"series-v1"');
 assert.equal(request.body.start,'2026-10-27T12:00:00.000Z');
 assert.equal(request.body.geo,'');
 assert.equal(await page.locator('#cal-occurrence-dialog button:disabled').count(),0);
 await page.evaluate(()=>{window.fail=false;$('form-cal-occurrence').requestSubmit();});
 await page.waitForSelector('#cal-occurrence-dialog',{state:'detached'});
 assert.equal(await page.evaluate(()=>window.requests.length),2);
 // Cancelling an occurrence cannot fall through to deleting its entire series.
 await page.evaluate(async()=>{window.confirmed=false;await deleteCalEvent(false);});
 assert.equal(await page.evaluate(()=>window.requests.length),2);
 await page.evaluate(async()=>{window.confirmed=true;await deleteCalEvent(false);});
 request=await page.evaluate(()=>window.requests.at(-1));
 assert.equal(request.method,'POST');assert.equal(request.body.action,'cancel');assert.equal(request.body.recurrence_id,'2026-10-26T10:00:00Z');
 assert.match(await page.evaluate(()=>window.confirmations.at(-1)),/только это повторение/);
 await page.evaluate(async()=>{await deleteCalEvent(true);});
 assert.equal(await page.evaluate(()=>window.requests.at(-1).method),'DELETE');
 assert.match(await page.evaluate(()=>window.confirmations.at(-1)),/всю серию/);
 const count=await page.evaluate(()=>window.requests.length);
 await page.evaluate(async()=>{window.switchUser=true;await deleteCalEvent(false);});
 assert.equal(await page.evaluate(()=>window.requests.length),count);
 // An editor opened under another account cannot submit after switching.
 await page.evaluate(()=>{state.me={id:'owner'};openCalOccurrenceEditor();state.me={id:'other'};$('form-cal-occurrence').requestSubmit();});
 await page.waitForSelector('#cal-occurrence-dialog',{state:'detached'});
 assert.equal(await page.evaluate(()=>window.requests.length),count);
 // Date-only instances do not shift to the browser's local timezone.
 await page.evaluate(()=>{state.me={id:'owner'};calState.currentEvent={...original,all_day:true,recurrence_id:'2026-10-26',start:'2026-10-26T00:00:00Z',end:'2026-10-27T00:00:00Z'};openCalOccurrenceEditor();});
 assert.equal(await page.getAttribute('#cal-occ-start','type'),'date');
 await page.fill('#cal-occ-start','2026-10-28');await page.fill('#cal-occ-end','2026-10-30');
 await page.evaluate(()=>$('form-cal-occurrence').requestSubmit());
 await page.waitForSelector('#cal-occurrence-dialog',{state:'detached'});
 assert.equal(await page.evaluate(()=>window.requests.at(-1).body.start),'2026-10-28T00:00:00.000Z');
 assert.deepEqual(await page.evaluate(()=>{
  const ev={all_day:true,start:'2026-10-28T00:00:00Z',end:'2026-10-30T00:00:00Z'};
  return ['2026-10-27','2026-10-28','2026-10-29','2026-10-30'].map(day=>calEventOccursOnDay(ev,day));
 }),[false,true,true,false]);
 assert.deepEqual(errors,[]);
 console.log('Calendar occurrence editor: timezone, date-only, immutable ID, retries, cancel/series scope, account isolation, 1440/390 px passed');
} finally {await browser.close();}
