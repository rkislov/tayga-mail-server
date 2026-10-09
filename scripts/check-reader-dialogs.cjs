const {chromium}=require('playwright-core');
const fs=require('fs');
(async()=>{
 const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
 const page=await browser.newPage();
 const html=fs.readFileSync('internal/frontend/dist/index.html','utf8').replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi,'').replace(/<link\b[^>]*>/gi,'');
 await page.setContent(html);await page.addStyleTag({content:fs.readFileSync('internal/frontend/dist/assets/app.css','utf8')});
 const source=fs.readFileSync('frontend/src/app.js','utf8');let functions='';
 for(const [start,end] of [['function showCalCompose(','async function uploadCalAttachment'],['function showContactCompose(','function showContactReader']])functions+=source.slice(source.indexOf(start),source.indexOf(end,source.indexOf(start)));
 await page.addScriptTag({content:`const $=id=>document.getElementById(id);const calState={selectedDay:'2026-10-09',calendarID:'cal',calendars:[{id:'cal',display_name:'Личный',writable:true}],attendees:[]};const contactState={editing:null};function mailImageText(ru,en){return ru}function setMsg(el,text){if(el)el.textContent=text}function renderCalAttendeeChips(){}function loadCalResourceOptions(){};function addCalAttendeesFromInput(){};function localInputToISO(value){return new Date(value).toISOString()};let creates=0,uploads=0;function api(){creates++;return Promise.resolve({id:'event'})};async function uploadCalAttachment(){uploads++;if(uploads===1)throw Error('upload failed')};function loadCalEvents(){return Promise.resolve()};function openCalEvent(){};`+fs.readFileSync('frontend/src/reader-actions.js','utf8')+functions+source.slice(source.indexOf('$("form-cal-event")?.addEventListener("submit"'),source.indexOf('/* —— Contacts',source.indexOf('$("form-cal-event")?.addEventListener("submit"')))});
 for(const width of [1280,390]){
  await page.setViewportSize({width,height:900});await page.evaluate(()=>showCalCompose(true));
  if(!await page.locator('#cal-backdrop').evaluate(e=>e.open))throw Error('Calendar dialog not open');
  const bounds=await page.locator('#cal-backdrop').evaluate(e=>({width:e.getBoundingClientRect().width,scroll:e.scrollWidth,client:e.clientWidth}));if(bounds.width>width||bounds.scroll>bounds.client+1)throw Error('Calendar overflow '+JSON.stringify(bounds));
  if(await page.locator('#event-calendar-choice').inputValue()!=='cal')throw Error('Calendar selection lost');
  await page.screenshot({path:`/private/tmp/calendar-editor-${width}.png`});await page.keyboard.press('Escape');if(await page.locator('#cal-backdrop').evaluate(e=>e.open))throw Error('Escape failed');
 }
 await page.setViewportSize({width:1280,height:900});await page.evaluate(()=>showCalCompose(true));await page.locator('#cal-summary').fill('Draft');
 await page.locator('#cal-files').setInputFiles({name:'file.txt',mimeType:'text/plain',buffer:Buffer.from('attachment')});
 await page.locator('#form-cal-event button[type=submit]').click();await page.waitForTimeout(100);if(await page.evaluate(()=>creates)!==1||!await page.locator('#cal-backdrop').evaluate(e=>e.open))throw Error('Failed upload did not retain draft');
 await page.locator('#form-cal-event button[type=submit]').click();await page.waitForTimeout(100);if(await page.evaluate(()=>creates)!==1||await page.locator('#cal-backdrop').evaluate(e=>e.open))throw Error('Upload retry duplicated event');
 await page.evaluate(()=>showContactCompose(true));if(!await page.locator('#contact-backdrop').evaluate(e=>e.open))throw Error('Contact editor not open');await page.keyboard.press('Escape');
 console.log('PASS: calendar editor fits desktop/mobile, calendar selection, native focus dialog and Escape; contact editor opens');await browser.close();
})().catch(error=>{console.error(error);process.exit(1)});
