const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const {JSDOM}=require('jsdom');
const source=fs.readFileSync(require('node:path').join(__dirname,'../src/mattermost-agents.js'),'utf8');
const settle=async()=>{for(let i=0;i<12;i++)await new Promise(r=>setImmediate(r));};
function fixture(t,admin=false){
 const dom=new JSDOM('<nav><button data-app="chat">Чат</button></nav><main><section id="app-chat"></section></main>',{url:'https://tms.test',runScripts:'outside-only'}),w=dom.window;
 w.state={email:'alice@test',tokens:{access_token:'alice'}};w.APPS=['chat'];w.showApp=name=>{for(const section of w.document.querySelectorAll('main>section'))section.classList.toggle('hidden',section.id!=='app-'+name);};
 const requests=[];let deferred=null,history=[],chat={id:'c1',agent_id:'helper',title:'Личный тест',updated_at:1,messages:[]};
 w.api=async(url,options={})=>{requests.push({url,options});options._guard?.();
  if(deferred&&url==='/api/v4/tms/agents')return new Promise(r=>{deferred.finish=()=>r({can_manage:false,catalog:{agents:[{id:'private',name:'STALE OWNER',description:''}],skills:[]}});});
  if(url.endsWith('/config'))return {version:0,providers:[],agents:[],skills:[]};
  if(url==='/api/v4/tms/agents')return {can_manage:admin,catalog:{agents:[{id:'helper',name:'<img src=x onerror=alert(1)>',description:'Помощник'}],skills:[]}};
  if(url.endsWith('/conversations')){if(options.method==='POST'){history=[chat];return chat;}return history;}
  if(url.endsWith('/messages')){chat={...chat,messages:[{role:'user',content:JSON.parse(options.body).content},{role:'assistant',content:'Ответ'}]};return chat;}
  return chat;
 };
 w.HTMLDialogElement.prototype.showModal=function(){this.open=true;};w.HTMLDialogElement.prototype.close=function(){this.open=false;this.dispatchEvent(new w.Event('close'));};
 w.eval(source);t.after(()=>w.close());return {w,requests,defer(){deferred={};return()=>{deferred.finish();deferred=null;};}};
}
test('Agents opens catalog, creates a private conversation and sends message',async t=>{
 const {w,requests}=fixture(t);w.showApp('agents');await settle();
 assert.ok(w.document.body.textContent.includes('Все агенты'));assert.equal(w.document.querySelectorAll('#app-agents img').length,0);
 const start=[...w.document.querySelectorAll('#app-agents button')].find(b=>b.textContent==='Начать диалог');start.click();await settle();
 const input=w.document.querySelector('.agents-composer textarea');input.value='Мой вопрос';w.document.querySelector('.agents-composer').dispatchEvent(new w.Event('submit',{cancelable:true}));await settle();
 assert.match(w.document.querySelector('.agents-messages').textContent,/Мой вопрос.*Ответ/);
 const req=requests.find(r=>r.url.endsWith('/messages'));assert.deepEqual(JSON.parse(req.options.body),{content:'Мой вопрос'});
 assert.equal(w.document.querySelector('[data-view="settings"]').hidden,false);
 w.document.querySelector('[data-view="settings"]').click();await settle();assert.match(w.document.querySelector('.agents-main').textContent,/администратору домена/);
 assert.ok(!requests.some(r=>r.url.endsWith('/config')));
});
test('logout removes private history and ignores a late catalog response',async t=>{
 const f=fixture(t),w=f.w;const finish=f.defer();w.showApp('agents');await settle();
 w.state.tokens=null;w.dispatchEvent(new w.Event('tms-products-session-ended'));finish();await settle();
 assert.ok(!w.document.querySelector('.agents-main').textContent.includes('STALE OWNER'));assert.equal(w.document.querySelector('.agents-main').textContent,'');assert.equal(w.document.querySelector('.agents-error').textContent,'');
});

test('DeepSeek preset supplies endpoint, model, thinking switch and private key field',async t=>{
 const {w,requests}=fixture(t,true);w.showApp('agents');await settle();w.document.querySelector('[data-view="settings"]').click();await settle();
 [...w.document.querySelectorAll('button')].find(b=>b.textContent==='+ Подключить модель').click();await settle();
 const dialog=w.document.querySelector('dialog');assert.ok(dialog.open);
 const labels=[...dialog.querySelectorAll('label')],field=text=>labels.find(el=>el.firstChild.textContent===text).querySelector('input,select');
 assert.equal(field('Провайдер').value,'deepseek');assert.equal(field('Базовый URL API').value,'https://api.deepseek.com');assert.equal(field('Идентификатор модели').value,'deepseek-flash');
 const key=field('API-ключ');assert.equal(key.type,'password');assert.equal(key.required,true);key.value='fixture-private-key';field('DeepSeek: режим рассуждений').checked=true;
 dialog.querySelector('form').dispatchEvent(new w.Event('submit',{cancelable:true}));await settle();
 const put=requests.find(r=>r.url.endsWith('/config')&&r.options.method==='PUT');assert.ok(put);const provider=JSON.parse(put.options.body).providers[0];assert.equal(provider.kind,'deepseek');assert.equal(provider.thinking,true);assert.equal(provider.api_key,'fixture-private-key');
 assert.ok(!w.document.body.textContent.includes('fixture-private-key'));
});

test('same email with a different login cannot accept a late response',async t=>{
 const f=fixture(t),w=f.w,finish=f.defer();w.showApp('agents');await settle();
 w.state.tokens={access_token:'different-login'};finish();await settle();
 assert.ok(!w.document.querySelector('.agents-main').textContent.includes('STALE OWNER'));
 assert.match(w.document.querySelector('.agents-main').textContent,/Все агенты/);
});
