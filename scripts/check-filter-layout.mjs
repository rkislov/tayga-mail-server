import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import {chromium} from 'playwright-core';
const base=process.env.TMS_URL||'http://127.0.0.1:18080';
if(!['127.0.0.1','localhost'].includes(new URL(base).hostname))throw Error('Disposable loopback server required');
const out='output/ui-fixes/filter-layout';await fs.mkdir(out,{recursive:true});
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage({viewport:{width:1440,height:960}}),errors=[];page.on('pageerror',error=>errors.push(error.message));
 for(const name of ['app.js','app.css'])await page.route('**/assets/'+name+'*',route=>route.fulfill({path:'internal/frontend/dist/assets/'+name}));
 await page.goto(base);await page.evaluate(()=>localStorage.setItem('tayga.lang','ru'));await page.reload();
 await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForFunction(()=>state.me?.id);
 await page.evaluate(async()=>{
  showApp('filters');await refreshSieve();
  visualRules.rules=[
   {name:'Счета и документы',enabled:true,match:'all',conditions:[{field:'From',operator:'contains',value:'billing@example.com'}],action:'fileinto',destination:'Документы',stop:true},
   {name:'Письма команды',enabled:true,match:'all',conditions:[{field:'From',operator:'contains',value:'@team.example.com'}],action:'fileinto',destination:'Команда',stop:true},
   {name:'Рассылки',enabled:false,match:'all',conditions:[{field:'Subject',operator:'contains',value:'Новости'}],action:'fileinto',destination:'Рассылки',stop:true}
  ];visualRules.dirty=true;await saveVisualRules();
 });
 await page.waitForFunction(()=>!visualRules.dirty);assert.equal(await page.locator('.visual-rule-row').count(),3);
 assert.equal(await page.locator('.filter-rule-actions button').count(),12);
 assert.equal(await page.locator('.visual-rule-row').first().getByRole('button',{name:'Выше',exact:true}).isDisabled(),true);
 await page.locator('.visual-rule-row').first().getByRole('button',{name:'Ниже',exact:true}).click();
 assert.equal(await page.locator('.rule-summary strong').first().innerText(),'Письма команды');
 await page.locator('.visual-rule-row').nth(1).getByRole('button',{name:'Выше',exact:true}).click();
 await page.locator('.visual-rule-row').last().getByRole('switch').check();assert.match(await page.locator('#filter-rule-count').innerText(),/3 \/ 3/);
 await page.locator('.visual-rule-row').last().getByRole('switch').uncheck();assert.match(await page.locator('#filter-save-state').innerText(),/несохранённые/);
 await page.click('#btn-rules-save');await page.waitForFunction(()=>!visualRules.dirty);
 await page.click('#btn-rule-new');await page.fill('.visual-rule-dialog input[name=name]','Проверка сохранения');await page.fill('.visual-rule-dialog [data-value]','qa@example.com');await page.selectOption('.visual-rule-dialog select[name=action]','keep');await page.click('.visual-rule-dialog [type=submit]');await page.waitForSelector('.visual-rule-dialog',{state:'detached'});
 await page.reload();await page.waitForFunction(()=>state.me?.id);await page.evaluate(async()=>{showApp('filters');await refreshSieve()});await page.waitForFunction(()=>visualRules.rules.length===4);
 await page.locator('.visual-rule-row').last().getByRole('button',{name:'Изменить правило',exact:true}).click();assert.equal(await page.inputValue('.visual-rule-dialog input[name=name]'),'Проверка сохранения');await page.screenshot({animations:'disabled',path:out+'/visual-editor.png'});await page.keyboard.press('Escape');
 await page.locator('.visual-rule-row').last().getByRole('button',{name:'Удалить правило',exact:true}).click();await page.click('#btn-rules-save');await page.waitForFunction(()=>!visualRules.dirty);
 await page.locator('#filter-tab-rules').focus();await page.keyboard.press('ArrowRight');assert.equal(await page.locator('#filter-tab-vacation').getAttribute('aria-selected'),'true');assert.equal(await page.locator('.visual-rule-builder').isVisible(),false);
 await page.fill('#vacation-subject','Спасибо за ваше письмо');await page.fill('#vacation-body','Я вернусь в понедельник и отвечу на ваше письмо.\nПо срочным вопросам напишите команде: team@example.com.');await page.locator('#vacation-enabled').check();await page.click('#btn-vacation-save');
 // The current Sieve engine does not implement vacation: keep the failure visible,
 // and ensure it cannot replace the active visual rules with a non-working script.
 await page.waitForFunction(()=>document.getElementById('vacation-msg').textContent.includes('не поддерживает автоответ'));
 assert.equal(await page.evaluate(async()=>{const data=await api('/api/v1/sieve/scripts');return data.scripts.find(script=>script.active)?.name}),'visual-rules');
 await page.screenshot({animations:'disabled',path:out+'/vacation.png'});await page.locator('#vacation-enabled').uncheck();await page.click('#btn-vacation-save');await page.waitForFunction(()=>document.getElementById('vacation-msg').textContent.includes('сохранён'));
 await page.locator('#filter-tab-vacation').focus();await page.keyboard.press('End');assert.equal(await page.locator('#sieve-body').isVisible(),true);await page.keyboard.press('Home');assert.equal(await page.locator('#filter-tab-rules').getAttribute('aria-selected'),'true');
 await page.evaluate(()=>renderOutlookRules({rules_json:JSON.stringify([{name:'Письма от партнёров',enabled:true}]),script:'keep;'}));
 assert.equal(await page.locator('#outlook-rules-summary').evaluate(e=>e.parentElement.id),'filter-panel-rules');
 await page.locator('#outlook-rules-summary>summary').click();assert.equal(await page.locator('.filter-outlook-row').isVisible(),true);await page.locator('#outlook-rules-summary>summary').click();
 await page.screenshot({animations:'disabled',path:out+'/rules.png'});
 for(const width of [320,390,768,1440]){
  await page.setViewportSize({width,height:960});
  for(const tab of ['rules','vacation','advanced']){
   await page.click('#filter-tab-'+tab);
   assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'document overflow '+width+' '+tab);
   assert.ok(await page.locator('#app-filters .workspace-settings-content').evaluate(e=>e.scrollWidth<=e.clientWidth+1),'content overflow '+width+' '+tab);
  }
  await page.click('#filter-tab-rules');
  if(width===390)await page.screenshot({animations:'disabled',path:out+'/mobile.png'});
 }
 await page.evaluate(()=>applyColorMode('dark'));await page.screenshot({animations:'disabled',path:out+'/dark.png'});
 assert.deepEqual(errors,[]);console.log('Passed: saved rules, edit/reorder/toggle/delete, explicit unsupported-vacation error without replacing rules, keyboard tabs, Outlook placement, 320–1440px layouts and dark mode.');
} finally {await browser.close()}
