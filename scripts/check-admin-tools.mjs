import { chromium } from 'playwright-core';
import assert from 'node:assert/strict';
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage({viewport:{width:1440,height:1000}}),errors=[];
 page.on('pageerror',e=>errors.push(e.message));
 await page.goto(process.env.TMS_URL||'http://127.0.0.1:18081');
 await page.evaluate(()=>{localStorage.clear();localStorage.setItem('tayga.lang','ru');localStorage.setItem('tayga.colorMode','light')});await page.reload();
 await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForSelector('#view-account:not(.hidden)');
 await page.evaluate(()=>showApp('monitor'));await page.waitForSelector('#chart-bars svg');await page.waitForTimeout(5200);
 assert.equal(await page.locator('#protection-stats polyline').count(),6);
 assert.equal(await page.locator('#chart-bars rect').count(),0);
 await page.screenshot({path:'docs/screenshots/ui-admin-monitor-099.png',fullPage:true});
 await page.evaluate(()=>openDomainDKIM());await page.waitForSelector('.dkim-generator table tr:nth-child(2)');
 await page.locator('.dkim-generator button[title="Ключ и DNS"]').first().click();await page.waitForSelector('.dkim-domain-form');
 // Generate only for this isolated example.com fixture, with signing disabled.
 await page.locator('.dkim-domain-form input[type=checkbox]').uncheck();
 await page.getByRole('button',{name:'Создать новый ключ и привязать',exact:true}).click();
 // On repeat runs, confirm replacement in the app's custom prompt.
 const confirm=page.getByRole('button',{name:'Заменить ключ',exact:true});
 if(await confirm.isVisible())await confirm.click();
 await page.waitForFunction(()=>document.querySelector('.dkim-generator [role=status]').textContent.startsWith('Ключ закреплён'));
 await page.waitForFunction(()=>[...document.querySelectorAll('.dkim-domain-form textarea')].some(e=>e.value.startsWith('v=DKIM1;')));
 await page.screenshot({path:'docs/screenshots/ui-admin-dkim-099.png'});
 await page.locator('.dkim-generator header button').click();
 await page.evaluate(async()=>{const tenants=await api('/api/v1/admin/tenants');const tenant=tenants.tenants[0];const domains=await api('/api/v1/admin/domains?tenant_id='+tenant.id);const domain=domains.domains[0];tenantNav.tenantID=tenant.id;tenantNav.tenantName=tenant.name;tenantNav.domainID=domain.id;tenantNav.domainName=domain.name;showApp('tenants');await refreshAdminUsers()});
 await page.fill('#admin-user-search','not-present@example.com');await page.waitForFunction(()=>document.querySelector('#admin-user-search-status').textContent==='Найдено: 0');
 await page.fill('#admin-user-search','ADMIN@EXAMPLE.COM');await page.waitForFunction(()=>document.querySelector('#admin-user-search-status').textContent==='Найдено: 1');
 await page.screenshot({path:'docs/screenshots/ui-admin-users-search-099.png',fullPage:true});
 await page.evaluate(()=>openRolesDirectory());await page.waitForSelector('dialog[open] table');assert.ok(await page.locator('dialog[open]').innerText().then(t=>t.includes('admin@example.com')));await page.locator('dialog[open] header button').click();
 await page.evaluate(()=>showApp('outbound'));await page.waitForFunction(()=>document.querySelector('#outbound-list').textContent.length>0);
 assert.equal(await page.locator('#app-monitor #outbound-list').count(),0);
 assert.deepEqual(errors,[]);console.log('Admin charts, DKIM generation, scoped user search, roles menu and separate queue — OK');
} finally {await browser.close()}
