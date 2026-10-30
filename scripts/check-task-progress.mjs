// Browser component test using the shipped markup/CSS and task controller.
// API persistence, validation, ownership and conflicts are covered by Go tests.
import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
import {readFile, mkdtemp} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';

const root = new URL('../', import.meta.url);
const html = await readFile(new URL('internal/frontend/dist/index.html', root), 'utf8');
const css = await readFile(new URL('internal/frontend/dist/assets/app.css', root), 'utf8');
const app = await readFile(new URL('frontend/src/app.js', root), 'utf8');
const translations = await readFile(new URL('frontend/src/i18n.js', root), 'utf8');
const markup = html.slice(html.indexOf('<section id="app-tasks"'), html.indexOf('<div id="app-notes"'));
const controller = app.slice(app.indexOf('let taskRows = [];'), app.indexOf('\nasync function chatSubscription'));
const output = await mkdtemp(join(tmpdir(), 'tayga-task-progress-'));
const browser = await chromium.launch({executablePath: process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true});
try {
  const page = await browser.newPage({viewport: {width: 1440, height: 1000}});
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  let rows = [], sequence = 0, rejectSave = false, activeUser = 'author';
  const candidate = {id:'assignee',name:'Екатерина Александровна · Подготовка внедрения',email:'assignee@branch.example.com'};
  const view = task => ({...task,owner_id:'author',owner_name:'Автор задачи',owner_email:'author@example.com',can_manage:activeUser==='author',can_update:activeUser==='author'||task.assignment_state==='accepted'});
  await page.exposeFunction('taskAPI', async (path, opts = {}) => {
    if (path.includes('/assignees?')) return {users:[candidate],total:1};
    if (path.includes('/history?')) return {events:[{id:'event',action:'accept',actor_name:candidate.name,created_at:'2026-10-19T10:00:00Z',detail:{changes:['assignment'],status:'InProgress',percent_complete:40}}],total:1};
    if (path.includes('/notifications?')) return {notifications:[],total:0,unread:0};
    if (path.endsWith('/assignment')) {
      const req = JSON.parse(opts.body), task=rows[0];
      if (req.action==='assign') {task.assignee_id=req.assignee_id;task.assignee_name=candidate.name;task.assignment_state='pending';}
      if (req.action==='accept') task.assignment_state='accepted';
      task.revision++;return {task:view(task)};
    }
    if (!opts.method) return {tasks:rows.map(view)};
    if (rejectSave) throw new Error('Задача изменена на другом устройстве. Обновите список.');
    const task = JSON.parse(opts.body);
    if (opts.method === 'POST') task.id = 'task:' + ++sequence;
    task.revision = (task.revision || 0) + 1;
    rows = [...rows.filter(x => x.id !== task.id), task];
    return view(task);
  });
  await page.setContent(`<html lang="ru" data-color-mode="light"><head><meta name="viewport" content="width=device-width, initial-scale=1"><style>${css}</style></head><body>${markup}</body></html>`);
  await page.addScriptTag({content: translations.slice(0, translations.indexOf('\nlet lang')) + `
    const $ = id => document.getElementById(id);
    const t = key => I18N.ru[key] || key;
    const escapeHtml = value => String(value).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const setMsg = (el, text) => { el.textContent = text; };
    const api = (path, opts) => window.taskAPI(path, opts);
  ` + controller});
  assert.deepEqual(errors, []);
  await page.evaluate(() => {
    $('app-tasks').classList.remove('hidden');
    $('tasks-new').classList.remove('hidden');
    document.querySelectorAll('[data-i18n]').forEach(el => el.textContent = t(el.dataset.i18n));
  });
  await page.click('#tasks-new');
  const title = 'Подготовить подробный план внедрения для нескольких организаций и доменов';
  await page.fill('#task-title', title);
  await page.fill('#task-body', 'Согласовать состав пилота, сроки и ответственных.');
  await page.selectOption('#task-status', 'InProgress');
  await page.fill('#task-percent', '35.5');
  await page.click('#task-save');
  await page.waitForSelector('[data-task-edit]');
  assert.equal(rows[0].status, 'InProgress');
  assert.equal(rows[0].percent_complete, 35.5);
  assert.match(await page.textContent('.task-row-detail'), /В работе · 35.5%/);
  await page.click('[data-task-edit]');
  assert.equal(await page.inputValue('#task-percent'), '35.5');
  await page.selectOption('#task-status', 'WaitingOnOthers');
  await page.click('#task-save');
  await page.waitForFunction(() => document.querySelector('.task-row-detail').textContent.includes('Ожидает других'));
  await page.click('[data-task-edit]');
  await page.fill('#task-percent', '100');
  assert.equal(await page.inputValue('#task-status'), 'Completed');
  await page.click('#task-save');
  await page.waitForFunction(() => document.querySelectorAll('[data-task-edit]').length === 0);
  await page.check('#tasks-completed');
  await page.waitForSelector('[data-task-toggle]:checked');
  await page.uncheck('[data-task-toggle]');
  await page.waitForFunction(() => document.querySelector('.task-row-detail').textContent.includes('Не начата · 0%'));
  await page.click('[data-task-edit]');
  await page.selectOption('#task-status', 'Deferred');
  await page.fill('#task-percent', '40');
  // Failed saves retain the user's edits and leave the dialog usable.
  rejectSave = true;
  await page.click('#task-save');
  await page.waitForFunction(() => document.getElementById('task-edit-msg').textContent.includes('другом устройстве'));
  assert.equal(await page.inputValue('#task-percent'), '40');
  assert.equal(await page.inputValue('#task-status'), 'Deferred');
  assert.equal(await page.isVisible('#task-dialog'), true);
  rejectSave = false;
  for (const width of [1440, 390]) {
    await page.setViewportSize({width, height: 900});
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    const box = await page.locator('#task-dialog').boundingBox();
    assert.ok(box.x >= 0 && box.x + box.width <= width + 1, `dialog overflow at ${width}`);
    assert.equal(await page.locator('#task-dialog').evaluate(el => el.scrollWidth > el.clientWidth + 1), false);
    assert.equal(await page.isVisible('#task-save'), true);
    await page.screenshot({path: join(output, `editor-${width}.png`)});
  }
  await page.click('#task-save');
  await page.waitForSelector('#task-dialog', {state: 'hidden'});
  assert.equal(rows[0].status, 'Deferred');
  assert.equal(rows[0].percent_complete, 40);
  for (const width of [1440, 390]) {
    await page.setViewportSize({width, height: 900});
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    assert.equal(await page.locator('#app-tasks').count(), 1);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1), false);
    const box = await page.locator('.task-row-delete').boundingBox();
    assert.ok(box.x >= 0 && box.x + box.width <= width + 1, `delete outside viewport at ${width}`);
    await page.screenshot({path: join(output, `list-${width}.png`)});
  }
  // Author assigns; recipient must accept before progress becomes editable.
  await page.click('[data-task-edit]');
  await page.waitForSelector('#task-assignee option[value="assignee"]', {state:'attached'});
  await page.selectOption('#task-assignee','assignee');
  await page.click('#task-assign');
  await page.waitForFunction(() => document.getElementById('task-people').textContent.includes('Ожидает принятия'));
  await page.click('#task-cancel');
  activeUser='assignee';
  await page.evaluate(() => refreshTasks());
  assert.equal(await page.isDisabled('[data-task-toggle]'),true);
  assert.equal(await page.locator('[data-task-delete]').count(),0);
  await page.click('[data-task-edit]');
  assert.equal(await page.isDisabled('#task-title'),true);
  assert.equal(await page.isDisabled('#task-percent'),true);
  assert.equal(await page.isVisible('#task-assignment-controls'),false);
  await page.click('#task-accept');
  await page.waitForFunction(() => !document.getElementById('task-percent').disabled);
  assert.equal(await page.isDisabled('#task-title'),true);
  await page.fill('#task-percent','65');
  await page.click('#task-save');
  await page.waitForSelector('#task-dialog',{state:'hidden'});
  assert.equal(rows[0].percent_complete,65);
  await page.click('[data-task-edit]');
  await page.click('#task-history-section summary');
  for(const width of [1440,390]) {
    await page.setViewportSize({width,height:1000});
    await page.evaluate(() => new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
    assert.equal(await page.locator('#task-dialog').evaluate(el=>el.scrollWidth>el.clientWidth+1),false);
    await page.screenshot({path:join(output,`assignment-${width}.png`)});
  }
  await page.click('#task-cancel');
  activeUser='author'; rows[0].recurring=true;
  await page.evaluate(() => refreshTasks());
  assert.equal(await page.isDisabled('[data-task-toggle]'),true);
  await page.click('[data-task-edit]');
  for(const id of ['task-title','task-body','task-status','task-percent','task-save']) assert.equal(await page.isDisabled('#'+id),true);
  assert.equal(await page.isVisible('#task-assignment-controls'),false);
  assert.match(await page.textContent('#task-permissions'),/Thunderbird/);
  assert.deepEqual(errors, []);
  await page.evaluate(() => resetTasks());
  assert.equal(await page.locator('[data-task-edit]').count(),0);
  assert.equal(await page.isVisible('#task-dialog'),false);
  console.log('Assignment, acceptance and author/assignee editor permissions — OK');
  console.log('Task status/progress, completion/reopening, failed-save retention and desktop/mobile layouts — OK');
  console.log('Screenshots:', output);
} finally { await browser.close(); }
