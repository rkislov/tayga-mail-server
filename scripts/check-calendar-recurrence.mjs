import {chromium} from 'playwright-core';
import fs from 'node:fs';
import assert from 'node:assert/strict';
const source=fs.readFileSync('frontend/src/calendar-recurrence.js','utf8');
const app=fs.readFileSync('frontend/src/app.js','utf8');
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage({timezoneId:'Europe/Moscow'}),errors=[];page.on('pageerror',error=>errors.push(error.message));
 await page.setContent('<!doctype html><html lang="ru" data-color-mode="light"><body><button id="btn-cal-recurrence">Повторение</button><p id="cal-load-msg"></p></body></html>');
 await page.addStyleTag({path:'internal/frontend/dist/assets/app.css'});
 await page.addScriptTag({content:`
 const $=id=>document.getElementById(id),t=key=>({save:'Сохранить',cancel:'Отмена'}[key]||key),mailImageText=ru=>ru;
 const state={me:{id:'owner'}},calState={currentEvent:{id:'object@instance',object_id:'object',recurrence_organizer:true},calendarID:'calendar',attendees:[]};
 const setMsg=(el,text)=>{if(el)el.textContent=text};const fmtCalEventWhen=(ev,date)=>new Date(date).toLocaleString('ru-RU');
 const master={id:'object',etag:'version1',recurrence_organizer:true,summary:'Планирование проекта',start:'2026-10-19T08:00:00Z',end:'2026-10-19T09:00:00Z',recurrence:{editable:true,timezone:'Europe/Berlin',exceptions:1,rule:{frequency:'WEEKLY',interval:1,weekdays:['MO'],count:4}}};
 let requests=[],fail=false,hold=null,loads=0;
 async function api(path,options){if(hold)await hold;if(!options)return structuredClone(master);requests.push({path,method:options.method,body:JSON.parse(options.body)});await new Promise(resolve=>setTimeout(resolve,25));if(fail)throw Error('event changed; refresh before saving');return {id:'object'};}
 const loadCalEvents=async()=>{loads++};
 `+source});
 await page.click('#btn-cal-recurrence');await page.waitForSelector('#form-cal-recurrence');
 assert.equal(await page.inputValue('#cal-rule-frequency'),'WEEKLY');assert.equal(await page.inputValue('#cal-rule-count'),'4');
 assert.match(await page.locator('.cal-recurrence-hint').textContent(),/Europe\/Berlin/);
 for(const width of [1440,390]) {
  await page.setViewportSize({width,height:900});
  const box=await page.locator('#cal-recurrence-dialog').boundingBox();assert(box.x>=0&&box.x+box.width<=width+1&&box.y>=0&&box.y+box.height<=901);
  assert.equal(await page.locator('#cal-recurrence-dialog').evaluate(el=>el.scrollWidth>el.clientWidth+1),false);
  await page.locator('#form-cal-recurrence button[type=submit]').scrollIntoViewIfNeeded();
  await page.screenshot({path:'/tmp/tayga-calendar-rule-'+width+'.png'});
 }
 await page.fill('#cal-rule-count','6');
 await page.evaluate(()=>{fail=true;$('form-cal-recurrence').requestSubmit();$('form-cal-recurrence').requestSubmit();});
 await page.waitForFunction(()=>$('cal-recurrence-msg').textContent.includes('event changed'));
 assert.equal(await page.evaluate(()=>requests.length),1);assert.equal(await page.locator('#form-cal-recurrence button[type=submit]').isDisabled(),false);
 let request=await page.evaluate(()=>requests[0]);assert.equal(request.path,'/api/v1/calendar/events/object/recurrence');assert.equal(request.body.etag,'version1');assert.deepEqual(request.body.rule,{frequency:'WEEKLY',interval:1,weekdays:['MO'],count:6});
 await page.evaluate(()=>{fail=false;$('form-cal-recurrence').requestSubmit()});await page.waitForSelector('#cal-recurrence-dialog',{state:'detached'});
 assert.equal(await page.evaluate(()=>loads),1);
 // Ending by date emits only Until, including in a browser in another zone.
 await page.click('#btn-cal-recurrence');await page.waitForSelector('#form-cal-recurrence');
 await page.selectOption('#cal-rule-end','until');await page.fill('#cal-rule-until','2026-11-30');
 await page.locator('#form-cal-recurrence button[type=submit]').click();await page.waitForSelector('#cal-recurrence-dialog',{state:'detached'});
 request=await page.evaluate(()=>requests.at(-1));assert.equal(request.body.rule.until,'2026-11-30');assert.equal(request.body.rule.count,undefined);
 // Cancellation never sends a mutation; an account switch cannot submit.
 let count=await page.evaluate(()=>requests.length);
 await page.click('#btn-cal-recurrence');await page.waitForSelector('#form-cal-recurrence');await page.keyboard.press('Escape');assert.equal(await page.evaluate(()=>requests.length),count);
 await page.click('#btn-cal-recurrence');await page.waitForSelector('#form-cal-recurrence');await page.evaluate(()=>{state.me.id='other';$('form-cal-recurrence').requestSubmit()});await page.waitForSelector('#cal-recurrence-dialog',{state:'detached'});assert.equal(await page.evaluate(()=>requests.length),count);
 // A late master fetch after closing must not insert a new form.
 await page.evaluate(()=>{state.me.id='owner';hold=new Promise(resolve=>window.release=resolve);openCalRecurrenceEditor()});await page.keyboard.press('Escape');await page.evaluate(()=>{release();hold=null});await page.waitForTimeout(40);assert.equal(await page.locator('#form-cal-recurrence').count(),0);
 // Complex imported rules are visibly read-only.
 await page.evaluate(()=>{master.recurrence={editable:false,raw:'FREQ=MONTHLY;BYDAY=1MO',rule:{frequency:'MONTHLY',interval:1},timezone:'UTC'}});
 await page.click('#btn-cal-recurrence');await page.waitForSelector('#form-cal-recurrence');assert.equal(await page.locator('#form-cal-recurrence button[type=submit]').isDisabled(),true);assert.match(await page.locator('#cal-rule').textContent(),/исходном клиенте/);await page.keyboard.press('Escape');
 // Wire up the actual creation handler with the same fields.
 await page.evaluate(()=>{
  document.body.insertAdjacentHTML('beforeend','<form id="form-cal-event"><input id="cal-summary" value="Новая серия"><input id="cal-location"><textarea id="cal-description"></textarea><input type="datetime-local" id="cal-start" value="2026-10-19T11:00"><input type="datetime-local" id="cal-end" value="2026-10-19T12:00"><p id="cal-msg"></p><button type="submit">Сохранить</button></form>');mountNewEventRecurrence();
 });
 await page.addScriptTag({content:`function addCalAttendeesFromInput(){};const localInputToISO=v=>new Date(v).toISOString();function renderCalAttendeeChips(){};function showCalCompose(){};function openCalEvent(){};`+app.slice(app.indexOf('$('+'"form-cal-event")?.addEventListener("submit"'),app.indexOf('/* —— Contacts —— */'))});
 await page.selectOption('#cal-new-recurrence-frequency','DAILY');await page.selectOption('#cal-new-recurrence-end','count');await page.fill('#cal-new-recurrence-count','3');
 await page.locator('#form-cal-event button').click();await page.waitForFunction(()=>requests.at(-1)?.method==='POST');
 request=await page.evaluate(()=>requests.at(-1));assert.equal(request.path,'/api/v1/calendar/calendars/calendar/events');assert.deepEqual(request.body.recurrence,{frequency:'DAILY',interval:1,count:3});assert.equal(request.body.timezone,'Europe/Moscow');assert.equal(request.body.start,'2026-10-19T08:00:00.000Z');
 const bookingMessages=await page.evaluate(()=>[
  calendarRecurrenceError('resource booking: resource booking requires a recurrence end date or count'),
  calendarRecurrenceError('resource is busy: Room 201 (2026-10-26T10:00:00Z)'),
  calendarRecurrenceError('resource booking: resource booking is limited to 1000 occurrences within 5 years')
 ]);
 assert.match(bookingMessages[0],/дату окончания серии/);assert.match(bookingMessages[1],/Помещение занято: Room 201/);assert.match(bookingMessages[1],/Изменения не сохранены/);assert.match(bookingMessages[2],/1000 встреч/);
 assert.deepEqual(errors,[]);
 console.log('PASS: recurrence editor/create, timezone, count/date, readonly import, cancel/retry/double-submit, account change, 1440/390 px');
} finally {await browser.close();}
