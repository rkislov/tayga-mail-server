import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try{
 const page=await browser.newPage({viewport:{width:1440,height:900}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(process.env.TMS_URL||'http://127.0.0.1:18081');await page.evaluate(()=>{localStorage.clear();localStorage.setItem('tayga.lang','ru');localStorage.setItem('tayga.colorMode','light')});await page.reload();await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForSelector('#view-account:not(.hidden)');
 await page.evaluate(()=>showApp('contacts'));await page.waitForTimeout(500);
 await page.evaluate(()=>{contactState.cards=[{id:'a',fn:'Anna Smith',first_name:'Anna',last_name:'Smith',email:'anna@example.com',tel:'+123'},{id:'b',fn:'Boris Adams',first_name:'Boris',last_name:'Adams',email:'boris@example.com'},{id:'c',fn:'Support',email:'support@example.com'}];contactState.searchQ='';renderContactCards()});
 await page.click('[data-app=contacts] .mail-settings-toggle');await page.selectOption('#contact-name-order','last');assert.equal(await page.locator('#contact-list .msg-from').first().textContent(),'Adams Boris');
 await page.fill('#contact-search','anna@');assert.equal(await page.locator('#contact-list [data-card]').count(),1);assert.equal(await page.locator('#contact-list .msg-from').textContent(),'Smith Anna');await page.fill('#contact-search','');await page.click('[data-app=contacts] .mail-settings-toggle');await page.screenshot({path:'docs/screenshots/ui-contacts-search-097.png'});
 await page.evaluate(()=>showApp('mail'));await page.waitForSelector('#mail-msg-list [data-msg]');
 await page.locator('.mail-flag-toggle').first().click();await page.waitForTimeout(500);assert.equal(await page.locator('.mail-flag-toggle').first().getAttribute('aria-pressed'),'true');
 await page.locator('.mail-color-toggle').first().click();await page.getByRole('button',{name:'Синий',exact:true}).click();await page.waitForTimeout(500);assert.equal(await page.locator('#mail-msg-list li').first().getAttribute('data-mail-color'),'blue');await page.screenshot({path:'docs/screenshots/ui-mail-labels-097.png'});
 await page.reload();await page.waitForSelector('#mail-msg-list [data-msg]');assert.equal(await page.locator('#mail-msg-list li').first().getAttribute('data-mail-color'),'blue');
 await page.setViewportSize({width:390,height:844});await page.evaluate(()=>showApp('contacts'));await page.waitForTimeout(200);assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);assert.deepEqual(errors,[]);console.log('Contact search/order, persistent mail flag/color, mobile overflow — OK');
}finally{await browser.close()}
