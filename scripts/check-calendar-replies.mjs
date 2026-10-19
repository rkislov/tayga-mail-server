// Exercise the actual invite controller in a browser, including local time and retries.
import { chromium } from 'playwright-core';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const root = new URL('../', import.meta.url);
const app = await readFile(new URL('frontend/src/app.js', root), 'utf8');
const html = await readFile(new URL('internal/frontend/dist/index.html', root), 'utf8');
const markupStart = html.indexOf('<div id="cal-invites-backdrop"');
const markup = html.slice(markupStart, html.indexOf('</section>', markupStart) + '</section>'.length) + '</div>';
const helpers = app.slice(app.indexOf('function localInputToISO('), app.indexOf('function calEventDayKey('));
const controller = app.slice(app.indexOf('async function refreshCalInvites('), app.indexOf('function shiftCalMonth('));
const recurrenceSource=await readFile(new URL('frontend/src/calendar-recurrence.js',root),'utf8');
const bookingErrorHelper=recurrenceSource.slice(recurrenceSource.indexOf('function calendarRecurrenceError('),recurrenceSource.indexOf("$('btn-cal-recurrence')"));
const browser = await chromium.launch({executablePath:process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
  const page = await browser.newPage({timezoneId:'Europe/Moscow'});
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.setContent(markup);
  await page.addScriptTag({content:`
    const $ = id => document.getElementById(id);
    const t = key => ({cal_accept:'Принять',cal_tentative:'Под вопросом',cal_decline:'Отклонить',cal_counter:'Предложить время',cal_invites_empty:'Нет ожидающих приглашений и предложений'}[key] || key);
    const mailImageText = ru => ru;
    const escapeHtml = value => String(value).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const fmtCalWhen = value => new Date(value).toLocaleString('ru-RU');
    const setMsg = (el, text) => { el.textContent = text; };
    const updateCalInviteBadge = () => {};
    const loadCalEvents = async () => {};
    const invite = {id:'one',summary:'Планирование',etag:'"version7"',start:'2026-10-19T10:00:00Z',end:'2026-10-19T11:00:00Z',organizer:{email:'owner@example.com'}};
    const state = {me:{id:"reader"}};
    const calState = {invites:[invite],highlightInvite:''};
    window.requests = []; window.prompts = []; window.answers = []; window.rejectReply = false;
    window.proposals = []; window.confirmResult = true;
    const askPrompt = async (title, options) => { window.prompts.push(options.value); return window.answers.shift(); };
    const askConfirm = async () => { if(window.switchUser) state.me={id:'another'}; return window.confirmResult; };
    const api = async (path, options) => {
      if(!options?.method) return {invites:[invite],proposals:window.proposals};
      window.requests.push({path,body:JSON.parse(options.body)});
      await new Promise(resolve => setTimeout(resolve,20));
      if(window.rejectReply) throw new Error('event changed; refresh');
      if(path.endsWith('/decision')) window.proposals=[];
      return {};
    };
  ` + helpers + bookingErrorHelper + controller});
  await page.evaluate(() => refreshCalInvites());
  await page.evaluate(async () => {
    window.answers = ['2026-10-20T15:00', '2026-10-20T16:00'];
    await Promise.all([replyCalInvite('one','counter'), replyCalInvite('one','counter')]);
  });
  const result = await page.evaluate(() => ({requests:window.requests,prompts:window.prompts,message:$('cal-invite-msg').textContent}));
  assert.deepEqual(result.prompts,['2026-10-19T13:00','2026-10-19T14:00']);
  assert.equal(result.requests.length,1);
  assert.equal(result.requests[0].body.start,'2026-10-20T12:00:00.000Z');
  assert.equal(result.requests[0].body.end,'2026-10-20T13:00:00.000Z');
  assert.equal(result.requests[0].body.etag,'"version7"');
  assert.match(result.message,/Время встречи пока не изменилось/);
  await page.evaluate(async () => { window.answers=[null]; await replyCalInvite('one','counter'); });
  await page.evaluate(async () => { window.answers=['2026-10-20T16:00','2026-10-20T15:00']; await replyCalInvite('one','counter'); });
  assert.equal(await page.evaluate(() => window.requests.length),1);
  assert.match(await page.textContent('#cal-invite-msg'),/Конец должен быть позже/);
  await page.evaluate(async () => { window.rejectReply=true; await replyCalInvite('one','accept'); });
  assert.match(await page.textContent('#cal-invite-msg'),/event changed/);
  assert.equal(await page.locator('#cal-invite-list button:disabled').count(),0);
  await page.evaluate(async () => { window.rejectReply=false; await replyCalInvite('one','accept'); });
  assert.equal(await page.evaluate(() => window.requests.length),3);

  // Organizer decisions use both the proposal revision and event ETag.
  await page.evaluate(async () => {
    window.requests = [];
    window.proposals = [{id:'proposal',event_id:'event',revision:'2026-10-19T12:00:00.123456Z',etag:'"version8"',summary:'Планирование следующего этапа разработки',attendee:'colleague@example.com',start:'2026-10-19T10:00:00Z',end:'2026-10-19T11:00:00Z',proposed_start:'2026-10-20T12:00:00Z',proposed_end:'2026-10-20T13:00:00Z'}];
    await refreshCalInvites();
  });
  const css = await readFile(new URL('internal/frontend/dist/assets/app.css', root),'utf8');
  await page.addStyleTag({content:css});
  await page.evaluate(() => $('cal-invites-backdrop').classList.remove('hidden'));
  for (const width of [1440,390]) {
    await page.setViewportSize({width,height:900});
    const bounds = await page.locator('#cal-invites-dialog').boundingBox();
    assert.ok(bounds.x>=0 && bounds.x+bounds.width<=width+1,'dialog overflow');
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth>innerWidth),false);
    await page.screenshot({path:'/tmp/tayga-calendar-proposal-'+width+'.png'});
  }
  assert.match(await page.textContent('.cal-proposal-item'),/Предложено:.*15:00/);
  await page.evaluate(async () => { window.confirmResult=false; await decideCalProposal('proposal','accept'); });
  assert.equal(await page.evaluate(() => window.requests.length),0);
  await page.evaluate(async () => { window.confirmResult=true; window.rejectReply=true; await decideCalProposal('proposal','accept'); });
  assert.equal(await page.locator('#cal-invite-list button:disabled').count(),0);
  await page.evaluate(async () => { window.rejectReply=false; await Promise.all([decideCalProposal('proposal','accept'),decideCalProposal('proposal','accept')]); });
  const decisions = await page.evaluate(() => window.requests);
  assert.equal(decisions.length,2);
  assert.equal(decisions[1].body.event_id,'event');
  assert.equal(decisions[1].body.revision,'2026-10-19T12:00:00.123456Z');
  assert.equal(decisions[1].body.etag,'"version8"');
  assert.equal(await page.locator('[data-proposal-act]').count(),0);
  assert.match(await page.textContent('#cal-invite-msg'),/Время изменено/);
  await page.evaluate(async () => {
    calState.proposals = [{id:'switch',proposed_start:'2026-10-20T12:00:00Z',proposed_end:'2026-10-20T13:00:00Z'}];
    window.switchUser = true;
    await decideCalProposal('switch','accept');
  });
  assert.equal(await page.evaluate(() => window.requests.length),2,'must not decide after account switch');
  assert.deepEqual(errors,[]);
  console.log('Calendar replies and organizer decisions: timezone, stale version, double submit, cancellation, retry and 1440/390px layout passed.');
} finally { await browser.close(); }
