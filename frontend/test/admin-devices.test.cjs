const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const {JSDOM}=require('jsdom');
const full=fs.readFileSync(require('node:path').join(__dirname,'../src/design-pack.js'),'utf8');
const start=full.indexOf('async function openAdminDevices(panel){'),end=full.indexOf('\n}\n',start)+2,source=full.slice(start,end);
const settle=async()=>{for(let i=0;i<10;i++)await new Promise(r=>setImmediate(r));};
test('forgetting a reset phone uses retirement, hides it and restores as blocked without issuing a wipe',async t=>{
 const dom=new JSDOM('<body><section id="app-devices"><button id="admin-device-archive"></button><button id="admin-device-refresh"></button><div id="admin-device-content"></div><p id="admin-device-status"></p></section></body>',{url:'https://tms.test',runScripts:'outside-only'}),w=dom.window;t.after(()=>w.close());
 w.HTMLDialogElement.prototype.showModal=function(){this.open=true;};w.HTMLDialogElement.prototype.close=function(){this.dispatchEvent(new w.Event('close'));};
 w.state={email:'alice@test',me:{is_admin:true}};w.mailImageText=(ru)=>ru;w.t=key=>key;w.enhanceConnectionTable=()=>{};w.askPrompt=async()=>'android-fixture';let retired=false;const calls=[];
 const device={id:'record',email:'alice@test',device_id:'android-fixture',type:'Android',agent:'<script>unsafe</script>',last_seen:new Date().toISOString(),blocked:true,wipe_status:'cancelled',account_wipe_supported:false};
 w.api=async(url,options={})=>{calls.push({url,options});if(options.method==='POST'){assert.equal(url,'/api/v1/admin/devices/record');const action=JSON.parse(options.body);assert.deepEqual(action,{action:retired?'restore':'retire',confirm_device_id:'android-fixture'});retired=action.action==='retire';return {retired,blocked:true,remote_wipe_sent:false};}return {devices:retired?(url.includes('include_retired=true')?[{...device,retired_at:new Date().toISOString()}]:[]):[device]};};
 w.designIcon=()=>'<svg aria-hidden="true"></svg>';w.eval(source);await w.openAdminDevices(w.document.getElementById("app-devices"));assert.equal(w.document.querySelectorAll('script').length,0);assert.ok(w.document.querySelector('[data-action=wipe_account]').disabled);assert.ok(w.document.querySelector('[data-action=cancel_wipe]').disabled);assert.equal(w.document.querySelectorAll('.device-action-buttons button[aria-label] svg').length,4);
 [...w.document.querySelectorAll('button')].find(b=>b.getAttribute('aria-label')==='Забыть на сервере').click();await settle();assert.match(w.document.body.textContent,/Старый ID заблокирован/);assert.ok(!w.document.querySelector('tbody').textContent.includes('android-fixture'));
 [...w.document.querySelectorAll('button')].find(b=>b.textContent==='Показать забытые').click();await settle();assert.match(w.document.querySelector('tbody').textContent,/Забыто на сервере/);assert.equal(w.document.querySelectorAll('tbody button').length,1);const restore=w.document.querySelector('tbody button');assert.equal(restore.getAttribute('aria-label'),'Восстановить запись');restore.click();await settle();assert.match(w.document.querySelector('tbody').textContent,/Заблокировано/);assert.equal(calls.filter(c=>c.options.method==='POST').length,2);assert.ok(calls.filter(c=>c.options.method==='POST').every(c=>JSON.parse(c.options.body).action!=='wipe_account'));
});
