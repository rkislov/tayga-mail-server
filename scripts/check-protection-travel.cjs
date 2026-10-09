const {chromium}=require('playwright-core');const fs=require('fs');
(async()=>{
 const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});const page=await browser.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.stack));
 const long='Очень длинная тема '.repeat(100);const item={id:'q1',subject:long,from:'sender@example.org',email:'admin@example.org',folder:'Junk',date:'2026-10-09T10:00:00Z',size:2048,kind:'spam',spam_score:'18.22',spam_status:'test='.repeat(300),preview:'<img src=x onerror=alert(1)> decoded text'};
 await page.route('**/*',async route=>{const url=new URL(route.request().url());let body,type='application/json';
 if(url.pathname==='/') {body=fs.readFileSync('internal/frontend/dist/index.html','utf8');type='text/html'}
 else if(url.pathname==='/assets/app.js'){body=fs.readFileSync('internal/frontend/dist/assets/app.js','utf8');type='text/javascript'}
 else if(url.pathname==='/assets/app.css'){body=fs.readFileSync('internal/frontend/dist/assets/app.css','utf8');type='text/css'}
 else if(url.pathname==='/api/v1/admin/quarantine')body={items:[item]};
 else if(url.pathname==='/api/v1/admin/quarantine/q1')body=item;
 else if(url.pathname.endsWith('/travel'))body={trips:[{kind:'flight',number:'SU1234',origin:'Москва',destination:'Сочи',departure:'2026-10-09T23:55',arrival:'2026-10-10T03:10',seat:'12A',source:'ticket.pdf'}]};
 else if(url.pathname==='/api/v1/calendar/calendars')body={calendars:[{id:'cal',name:'default',display_name:'Личный',writable:true}]};
 else if(url.pathname.endsWith('/events'))body={events:[]};
 else if(url.pathname.endsWith('/invites'))body={invites:[]};
 else if(url.pathname.endsWith('/resources'))body={resources:[]};
 else body={};
 await route.fulfill({status:200,contentType:type,body:typeof body==='string'?body:JSON.stringify(body)});
 });
 await page.goto('http://tayga.test/');await page.evaluate(()=>{state.tokens={access_token:'test'};show('view-account');showApp('antispam')});await page.locator('[data-q-details="q1"]').first().waitFor();
 for(const width of [1280,390]){await page.setViewportSize({width,height:900});const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1);if(overflow)throw Error('Protection page horizontal overflow at '+width);}
 await page.setViewportSize({width:1280,height:900});await page.locator('[data-q-details="q1"]').first().click();if(!await page.locator('#protection-reader').evaluate(e=>e.open))throw Error('Preview missing');if(await page.locator('#protection-reader img').count())throw Error('Unsafe preview markup rendered');await page.screenshot({path:'/private/tmp/protection-details.png'});await page.keyboard.press('Escape');
 await page.evaluate(()=>openTravelDrafts('ticket'));await page.locator('dialog[open] select').nth(0).selectOption('180');await page.locator('dialog[open] select').nth(1).selectOption('180');await page.locator('dialog[open]').getByRole('button',{name:'Создать событие',exact:true}).click();await page.locator('#cal-backdrop[open]').waitFor();if(!await page.locator('#cal-description').inputValue().then(v=>v.includes('12A')))throw Error('Seat missing from event');
 await page.keyboard.press('Escape');
 for(const kind of ['cal','card','imap'])if(await page.locator('#mig-'+kind+'-skip-tls').isChecked())throw Error('Certificate bypass enabled by default');
 if(errors.length)throw Error(errors.join('\n'));console.log('PASS: full app bootstrap, quarantine bounded desktop/mobile, safe modal text, ticket to calendar, certificate verification defaults');await browser.close();
})().catch(err=>{console.error(err);process.exit(1)});
