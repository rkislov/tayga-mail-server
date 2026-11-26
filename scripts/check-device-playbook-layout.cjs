// Run with the opt-in TestProductBrowserStand fixture and playwright-core.
const {chromium}=require('playwright-core');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const root=path.resolve(__dirname,'..'),output=process.env.TMS_UI_OUTPUT||'/private/tmp/tms-ui-qa';
const base=process.env.TMS_PRODUCT_URL||'http://127.0.0.1:59864';
const asset=name=>fs.readFileSync(path.join(root,'internal/frontend/dist/assets',name),'utf8');
(async()=>{
 fs.mkdirSync(output,{recursive:true});
 const browser=await chromium.launch({headless:true,...(process.env.CHROME_PATH?{executablePath:process.env.CHROME_PATH}:{})});
 try{
  const page=await browser.newPage({viewport:{width:1440,height:900}});
  await page.route('**/assets/tayga-products.css*',route=>route.fulfill({contentType:'text/css',body:asset('tayga-products.css')}));
  await page.route('**/assets/tayga-product-frame.js*',route=>route.fulfill({contentType:'text/javascript',body:asset('tayga-product-frame.js')}));
  for(const theme of ['light','dark']){
   await page.goto(base+'/test-login?theme='+theme);await page.waitForSelector('[data-testid="search-filter"]');
   const search=page.locator('[data-testid="search-filter"] input');assert.ok((await search.boundingBox()).height<=38);
   const checkbox=page.locator('[data-testid="my-runs-only"] input');
   await checkbox.focus();await page.keyboard.press('Space');assert.ok(await checkbox.isChecked());
   await page.keyboard.press('Space');assert.ok(!await checkbox.isChecked());
   await search.fill('bob');await page.waitForTimeout(650);
   await search.fill('');await page.waitForTimeout(650);
   await page.waitForFunction(()=>[...document.querySelectorAll('.PlaybookRunProfile img')].every(i=>i.complete&&i.naturalWidth>0));
   await page.screenshot({path:path.join(output,'playbooks-'+theme+'.png')});
   await page.goto(base+'/test-login?run=legacy&theme='+theme);
   await page.waitForSelector('[data-testid="checkbox-item-container"] input[type="checkbox"]');
   const task=page.locator('[data-testid="checkbox-item-container"] input[type="checkbox"]').first();
   await task.check();assert.ok(await task.isChecked());
   assert.ok((await task.evaluate(e=>getComputedStyle(e).backgroundImage)).includes('data:image/svg+xml'));
   await page.screenshot({path:path.join(output,'playbook-task-'+theme+'.png')});
   await task.uncheck();assert.ok(!await task.isChecked());

   const update=page.getByRole('button',{name:/^(Опубликовать обновление|Post update)$/}).first();
   await update.click();
   const modal=page.locator('#playbooks_update_run_status_dialog');
   const editor=modal.locator('textarea#update_run_status_textbox');
   const previewToggle=modal.locator('.textbox-preview-link');
   const publish=modal.getByTestId('modal-confirm-button');
   const cancel=modal.getByTestId('modal-cancel-button');
   await editor.waitFor();
   assert.ok(await publish.isDisabled());
   assert.equal(await editor.evaluate(el=>getComputedStyle(el).borderTopStyle),'solid');
   await editor.fill('**Проверка Markdown**');
   const finish=modal.getByTestId('mark-run-as-finished');
   await finish.check();
   await page.waitForFunction(()=>!document.querySelector('#playbooks_update_run_status_dialog [data-testid="modal-confirm-button"]').disabled);
   await finish.uncheck();
   await previewToggle.click();
   const preview=modal.locator('.tayga-update-preview strong');
   await preview.waitFor();
   assert.equal(await preview.textContent(),'Проверка Markdown');
   await previewToggle.click();
   assert.equal(await editor.inputValue(),'**Проверка Markdown**');
   await page.screenshot({path:path.join(output,'playbook-update-'+theme+'.png')});
   await page.setViewportSize({width:390,height:700});
   const box=await modal.locator('.modal-content').boundingBox();
   assert.ok(box.x>=0 && box.x+box.width<=390);
   assert.ok(await cancel.isVisible() && await publish.isVisible());
   await page.screenshot({path:path.join(output,'playbook-update-mobile-'+theme+'.png')});
   await editor.fill('');
   await cancel.click();
   await modal.waitFor({state:'hidden'});
   await page.setViewportSize({width:1440,height:900});
  }
  const source=fs.readFileSync(path.join(root,'frontend/src/design-pack.js'),'utf8');
  const start=source.indexOf('async function openAdminDevices(panel){'),end=source.indexOf('\n}\n',start)+2;
  const iconStart=source.indexOf('function designIcon(name) {'),iconEnd=source.indexOf('\n}\n',iconStart)+2;
  const html=`<html lang="ru"><meta charset="utf-8"><link rel="stylesheet" href="/assets/app.css"><body style="padding:24px"><div class="admin-operation-content"><button id="admin-device-refresh"></button><button id="admin-device-archive"></button><p id="admin-device-status"></p><div id="admin-device-content"></div></div><script>const state={me:{is_admin:true}};const t=k=>k,mailImageText=ru=>ru,enhanceConnectionTable=()=>{};const api=async()=>({devices:[{id:'qa',email:'alice@example.com',type:'iPhone',device_id:'QJEOHR81PP5112T7GE8LQKJVJO',agent:'Apple-iPhone19C3/2402.119',address:'46.242.15.89:34568',last_seen:'2026-11-26T08:52:27+03:00',wipe_status:'none'}]});${source.slice(iconStart,iconEnd)};${source.slice(start,end)};openAdminDevices(document.body);</script>`;
  await page.route('**/assets/app.css',route=>route.fulfill({contentType:'text/css',body:asset('app.css')}));
  await page.route('**/device-fixture',route=>route.fulfill({contentType:'text/html',body:html}));
  await page.goto(base+'/device-fixture');await page.waitForSelector('.device-action-buttons');
  assert.equal(await page.locator('.device-action-buttons button[aria-label] svg').count(),4);
  assert.ok((await page.locator('.device-action-buttons').boundingBox()).width<180);
  assert.equal(await page.locator('.device-address').evaluate(el=>getComputedStyle(el).whiteSpace),'nowrap');
  await page.screenshot({path:path.join(output,'devices-desktop.png')});
  console.log('Passed: compact filters, keyboard toggles, task updates, status editor and preview, avatars, light/dark, device icon controls.');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
