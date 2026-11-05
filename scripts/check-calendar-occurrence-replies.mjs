import {chromium} from 'playwright-core';
import {readFile} from 'node:fs/promises';
import assert from 'node:assert/strict';

const source=await readFile('frontend/src/calendar-occurrence-replies.js','utf8');
const app=await readFile('frontend/src/app.js','utf8');
const timeHelpers=app.slice(app.indexOf('function localInputToISO('),app.indexOf('function calEventDayKey('));
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
  const page=await browser.newPage({timezoneId:'Europe/Moscow'}),errors=[];
  page.on('pageerror',error=>errors.push(error.message));
  await page.setContent('<!doctype html><html lang="ru" data-color-mode="light"><body><main style="padding:16px"><article id="cal-reader"></article></main></body></html>');
  await page.addStyleTag({path:'internal/frontend/dist/assets/app.css'});
  await page.addScriptTag({content:`
    const $=id=>document.getElementById(id),mailImageText=ru=>ru;
    const t=key=>({cal_accept:'Принять',cal_tentative:'Под вопросом',cal_decline:'Отклонить',cal_counter:'Предложить время'}[key] || key);
    const escapeHtml=value=>String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const partstatLabel=value=>({ACCEPTED:'Принято',TENTATIVE:'Под вопросом',DECLINED:'Отклонено','NEEDS-ACTION':'Нет ответа'}[value] || value);
    const fmtCalEventWhen=(ev,value)=>new Date(value).toLocaleString('ru-RU');
    const setMsg=(el,text)=>el.textContent=text;
    const state={me:{id:'reader'}},calState={currentEvent:null};
    const initial={id:'copy@2026-10-26T09:00:00Z',object_id:'copy',recurrence_id:'2026-10-26T09:00:00Z',etag:'v1',start:'2026-10-27T09:00:00Z',end:'2026-10-27T10:00:00Z',occurrence_invite_id:'invite',occurrence_partstat:'NEEDS-ACTION'};
    let serverEvent=structuredClone(initial),requests=[],reads=0,loads=0,dropResponse=false,gate=null;
    let answers=[],prompts=[];
    const askPrompt=async(title,options)=>{prompts.push(options.value);return answers.shift();};
    const loadCalEvents=async()=>{loads++};
    function showCalReader(ev){calState.currentEvent=ev;renderCalOccurrenceReply(ev);}
    async function api(path,options){
      if(!options){reads++;if(gate)await gate;return structuredClone(serverEvent);}
      const body=JSON.parse(options.body);requests.push({path,body});
      if(gate)await gate;else await new Promise(resolve=>setTimeout(resolve,30));
      if(body.action==='counter'){
        serverEvent.occurrence_proposed_start=body.start;serverEvent.occurrence_proposed_end=body.end;
        if(dropResponse)throw Error('network interrupted');
        return {id:'invite',recurrence_id:initial.recurrence_id,action:'counter',proposed_start:body.start,proposed_end:body.end};
      }
      const status={accept:'ACCEPTED',tentative:'TENTATIVE',decline:'DECLINED'}[body.action];
      delete serverEvent.occurrence_proposed_start;delete serverEvent.occurrence_proposed_end;
      serverEvent.occurrence_partstat=status;serverEvent.etag='v'+(requests.length+1);
      if(dropResponse)throw Error('network interrupted');
      return {id:'invite',recurrence_id:initial.recurrence_id,partstat:status,series_partstat:'ACCEPTED'};
    }
  `+timeHelpers+source});
  await page.evaluate(()=>showCalReader(structuredClone(initial)));
  assert.match(await page.locator('#cal-occurrence-reply').textContent(),/Ответ только на это повторение/);
  // The displayed date is the moved start; the request retains the original identity.
  assert.match(await page.locator('#cal-occurrence-reply').textContent(),/27.10.2026/);
  for(const width of [1440,390]){
    await page.setViewportSize({width,height:900});
    await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
    assert.equal(await page.locator('#cal-occurrence-reply').count(),1);
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth),false);
    await page.screenshot({path:'/tmp/tayga-calendar-occurrence-reply-'+width+'.png'});
  }
  await page.evaluate(async()=>{const ev=calState.currentEvent,host=$('cal-occurrence-reply');await Promise.all([replyCalOccurrence(ev,'decline','DECLINED',host),replyCalOccurrence(ev,'accept','ACCEPTED',host)]);});
  let result=await page.evaluate(()=>({requests,reads,loads}));
  assert.equal(result.requests.length,1);assert.equal(result.reads,1);assert.equal(result.loads,1);
  assert.deepEqual(result.requests[0],{path:'/api/v1/calendar/invites/invite/reply',body:{action:'decline',etag:'v1',recurrence_id:'2026-10-26T09:00:00Z'}});
  assert.equal(await page.getByRole('button',{name:'Отклонить',exact:true}).isDisabled(),true);
  await page.getByRole('button',{name:'Под вопросом',exact:true}).click();
  await page.waitForFunction(()=>!calOccurrenceReplyBusy);
  assert.equal(await page.getByRole('button',{name:'Под вопросом',exact:true}).isDisabled(),true);
  // An ambiguous failure freezes actions until a fresh read, without replaying POST.
  await page.evaluate(()=>dropResponse=true);
  await page.getByRole('button',{name:'Принять',exact:true}).click();
  await page.getByRole('button',{name:'Обновить встречу',exact:true}).waitFor();
  for(const name of ['Принять','Под вопросом','Отклонить'])assert.equal(await page.getByRole('button',{name,exact:true}).isDisabled(),true);
  const posts=await page.evaluate(()=>requests.length);
  await page.getByRole('button',{name:'Обновить встречу',exact:true}).click();
  await page.waitForSelector('#cal-occurrence-reply .msg:empty');
  assert.equal(await page.evaluate(()=>requests.length),posts);
  assert.equal(await page.getByRole('button',{name:'Принять',exact:true}).isDisabled(),true);
  // Counter is a separate request scoped to the original date, using local input.
  const beforeCounter=await page.evaluate(()=>requests.length);
  await page.evaluate(()=>{dropResponse=false;answers=[null]});
  await page.getByRole('button',{name:'Предложить время',exact:true}).click();await page.waitForFunction(()=>!calOccurrenceReplyBusy);
  assert.equal(await page.evaluate(()=>requests.length),beforeCounter);
  assert.equal(await page.getByRole('button',{name:'Предложить время',exact:true}).isDisabled(),false);
  await page.evaluate(async()=>{answers=['2026-10-28T16:00','2026-10-28T15:00'];const ev=calState.currentEvent;await replyCalOccurrence(ev,'counter',null,$('cal-occurrence-reply'))});
  assert.equal(await page.evaluate(()=>requests.length),beforeCounter);assert.match(await page.locator('#cal-occurrence-reply [role=status]').textContent(),/Конец должен быть позже/);
  await page.evaluate(async()=>{answers=['2026-10-28T15:00','2026-10-28T16:00'];const ev=calState.currentEvent,host=$('cal-occurrence-reply');await Promise.all([replyCalOccurrence(ev,'counter',null,host),replyCalOccurrence(ev,'counter',null,host)]);});
  const counter=await page.evaluate(()=>requests.at(-1));
  assert.equal(await page.evaluate(()=>requests.length),beforeCounter+1);
  assert.deepEqual(counter.body,{action:'counter',etag:await page.evaluate(()=>serverEvent.etag),recurrence_id:'2026-10-26T09:00:00Z',start:'2026-10-28T12:00:00.000Z',end:'2026-10-28T13:00:00.000Z'});
  assert.match(await page.locator('#cal-occurrence-reply').textContent(),/Предложено:.*28.10.2026/);
  await page.screenshot({path:'/tmp/tayga-calendar-occurrence-counter-390.png'});
  assert.equal(await page.evaluate(()=>calState.currentEvent.occurrence_partstat),'ACCEPTED');
  assert.equal(await page.getByRole('button',{name:'Принять',exact:true}).isDisabled(),false,'an explicit RSVP can withdraw a pending proposal');
  await page.evaluate(async()=>{answers=[null];const ev=calState.currentEvent;await replyCalOccurrence(ev,'counter',null,$('cal-occurrence-reply'));});
  assert.equal(await page.evaluate(()=>prompts.at(-1)),'2026-10-28T15:00');
  await page.evaluate(()=>{dropResponse=true;answers=['2026-10-29T15:00','2026-10-29T16:00']});
  await page.getByRole('button',{name:'Предложить время',exact:true}).click();await page.getByRole('button',{name:'Обновить встречу',exact:true}).waitFor();
  const afterLostCounter=await page.evaluate(()=>requests.length);
  assert.equal(await page.getByRole('button',{name:'Предложить время',exact:true}).isDisabled(),true);
  await page.getByRole('button',{name:'Обновить встречу',exact:true}).click();await page.waitForSelector('#cal-occurrence-reply .msg:empty');
  assert.equal(await page.evaluate(()=>requests.length),afterLostCounter);
  assert.match(await page.locator('#cal-occurrence-reply').textContent(),/Предложено:.*29.10.2026/);
  // Closing/selecting another event during the response cannot repaint it.
  await page.evaluate(()=>{dropResponse=false;gate=new Promise(resolve=>window.release=resolve);const ev=calState.currentEvent;window.pending=replyCalOccurrence(ev,'decline','DECLINED',$('cal-occurrence-reply'));showCalReader({...initial,id:'other',object_id:'other'});release();gate=null;});
  await page.evaluate(()=>window.pending);
  assert.equal(await page.evaluate(()=>calState.currentEvent.id),'other');
  assert.equal(await page.getByRole('button',{name:'Принять',exact:true}).isDisabled(),false);
  // A different account is not shown the previous account's late result.
  await page.evaluate(()=>{gate=new Promise(resolve=>window.release=resolve);const ev=calState.currentEvent;window.pending=replyCalOccurrence(ev,'accept','ACCEPTED',$('cal-occurrence-reply'));state.me.id='other-user';showCalReader(null);release();gate=null;});
  await page.evaluate(()=>window.pending);
  assert.equal(await page.locator('#cal-occurrence-reply').count(),0);
  await page.evaluate(()=>showCalReader({...initial,busy_only:true}));assert.equal(await page.locator('#cal-occurrence-reply').count(),0);
  await page.evaluate(()=>showCalReader({...initial,status:'CANCELLED'}));assert.equal(await page.locator('#cal-occurrence-reply').count(),0);
  await page.evaluate(()=>showCalReader({...initial,occurrence_invite_id:undefined}));assert.equal(await page.locator('#cal-occurrence-reply').count(),0);
  assert.deepEqual(errors,[]);
  console.log('PASS: instance RSVP/counter, local time, original identity/moved date, double click, invalid time/cancel, lost response/refresh, selection/account isolation, cancelled/private event, 1440/390 px');
}finally{await browser.close();}
