const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const {JSDOM}=require('jsdom');
const source=fs.readFileSync(require('node:path').join(__dirname,'../src/mattermost-agents.js'),'utf8');
const settle=async()=>{for(let i=0;i<12;i++)await new Promise(r=>setImmediate(r));};
function fixture(t,admin=false,empty=false,tools=false){
 const dom=new JSDOM('<nav><button data-app="chat">Чат</button></nav><main><section id="app-chat"></section></main>',{url:'https://tms.test',runScripts:'outside-only'}),w=dom.window;
 w.state={email:'alice@test',tokens:{access_token:'alice'}};w.APPS=['chat'];w.showApp=name=>{for(const app of w.APPS)w.document.getElementById('app-'+app)?.classList.toggle('hidden',app!==name);};
 const requests=[];let reply='Ответ',deferred=null,history=[],chat={id:'c1',channel_id:'dm1',agent_id:'helper',title:'Личный тест',updated_at:1,messages:[],web_enabled:false,actions:tools?[{id:'a1',tool:'mail_draft',arguments:{to:'recipient@example.test',subject:'<img src=x>',text:'Exact proposed body'},state:'pending',expires_at:Date.now()+60000}]:[]};
 w.api=async(url,options={})=>{requests.push({url,options});options._guard?.();
  if(deferred&&url==='/api/v4/tms/agents')return new Promise(r=>{deferred.finish=()=>r({can_manage:false,catalog:{agents:[{id:'private',name:'STALE OWNER',description:''}],skills:[]}});});
  if(url.endsWith('/config'))return {version:0,providers:[],agents:[],skills:[]};
  if(url.endsWith('/config/models'))return {models:['catalog-first','catalog-second']};
  if(url==='/api/v4/files')return {file_infos:options.body.getAll('files').map((file,i)=>({id:'file'+i,name:file.name,size:file.size}))};
  if(url==='/api/v4/tms/agents')return {can_manage:admin,search_available:true,catalog:{agents:empty?[]:[{id:'helper',name:'<img src=x onerror=alert(1)>',description:'Помощник',tools:tools?['web_search']:[]}],skills:[]}};
  if(url.endsWith('/conversations')){if(options.method==='POST'){history=[chat];return chat;}return history;}
  if(options.method==='PATCH'){chat={...chat,...JSON.parse(options.body)};return chat;}
  if(url.includes('/actions/'))return {status:'sent'};
  if(url.endsWith('/messages')){chat={...chat,messages:[{role:'user',content:JSON.parse(options.body).content},{role:'assistant',content:reply}]};return chat;}
  return chat;
 };
 w.HTMLDialogElement.prototype.showModal=function(){this.open=true;};w.HTMLDialogElement.prototype.close=function(){this.open=false;this.dispatchEvent(new w.Event('close'));};
 w.eval(fs.readFileSync(require.resolve('markdown-it/browser'),'utf8'));
 w.eval(fs.readFileSync(require('node:path').join(__dirname,'../src/chat-markdown.js'),'utf8'));
 w.eval(fs.readFileSync(require('node:path').join(__dirname,'../src/chat-workspace.js'),'utf8'));w.eval(source);t.after(()=>w.close());return {w,requests,replyWith(content){reply=content;},defer(){deferred={};return()=>{deferred.finish();deferred=null;};}};
}
test('Agents opens catalog, creates a private conversation and sends message',async t=>{
 const {w,requests}=fixture(t);w.showApp('agents');await settle();
 assert.ok(w.document.body.textContent.includes('Все агенты'));assert.equal(w.document.querySelectorAll('#app-agents img').length,0);
 const start=[...w.document.querySelectorAll('#app-agents button')].find(b=>b.textContent==='Начать диалог');start.click();await settle();
 const input=w.document.querySelector('.agents-composer textarea');input.value='Мой вопрос';w.document.querySelector('.agents-composer').dispatchEvent(new w.Event('submit',{cancelable:true}));await settle();
 assert.match(w.document.querySelector('.agents-messages').textContent,/Мой вопрос.*Ответ/);
 const req=requests.find(r=>r.url.endsWith('/messages'));const body=JSON.parse(req.options.body);assert.equal(body.content,'Мой вопрос');assert.ok(body.pending_post_id);
 assert.equal(w.document.querySelector('[data-view="settings"]').hidden,false);
 w.document.querySelector('[data-view="settings"]').click();await settle();assert.match(w.document.querySelector('.agents-main').textContent,/администратору домена/);
 assert.ok(!requests.some(r=>r.url.endsWith('/config')));
});
test('agent replies render Markdown on send and when reopening the history',async t=>{
 const f=fixture(t),{w}=f;
 f.replyWith([
  '# Дайджест',
  '',
  'Отправка на \\*\\*07:00 мск\\*\\*. Напишите **«Дайджест»**.',
  '',
  '- [Источник](https://example.org/news)',
  '- [Справка](/help)',
  '- [Недоступная ссылка](javascript:alert%281%29)',
  '',
  '<img src=x onerror=alert(1)>',
  '',
  '![Иллюстрация](https://example.org/cover.png)',
  '[![Обложка](https://example.org/preview.png)](https://example.org/article)',
  '',
  '| Время | Событие |',
  '| --- | --- |',
  '| 07:00 | Выпуск |',
  '',
  '```text',
  '\\*\\*символы\\*\\*',
  '```',
 ].join('\n'));
 w.showApp('agents');await settle();
 [...w.document.querySelectorAll('button')].find(button=>button.textContent==='Начать диалог').click();
 await settle();
 const composer=w.document.querySelector('.agents-composer');
 composer.querySelector('textarea').value='Запрос **буквально**';
 composer.dispatchEvent(new w.Event('submit',{cancelable:true}));await settle();
 const verify=()=>{
  const body=w.document.querySelector('.agents-message.assistant .chat-markdown');
  assert.equal(body.querySelector('h1').textContent,'Дайджест');
  assert.deepEqual(Array.from(body.querySelectorAll('strong'),tag=>tag.textContent),['07:00 мск','«Дайджест»']);
  assert.equal(body.querySelectorAll('li').length,3);
  assert.equal(body.querySelector('tbody td').textContent,'07:00');
  assert.equal(body.querySelector('pre code').textContent.trim(),'\\*\\*символы\\*\\*');
  assert.equal(body.querySelector('a[href="/help"]').hasAttribute('target'),false);
  assert.equal(body.querySelector('a[href="https://example.org/news"]').rel,'noopener noreferrer');
  assert.equal(body.querySelector('a[href="https://example.org/cover.png"]').textContent,'Иллюстрация');
  assert.equal(body.querySelector('a[href="https://example.org/article"]').textContent,'Обложка');
  assert.equal(body.querySelector('a[href="https://example.org/preview.png"]'),null);
  assert.equal(body.querySelector('img,script,[onerror],a[href^="javascript:"]'),null);
  assert.match(body.textContent,/<img src=x/);
  assert.equal(w.document.querySelector('.agents-message.user>div').textContent,'Запрос **буквально**');
 };
 verify();
 w.document.querySelector('[data-view="history"]').click();await settle();
 [...w.document.querySelectorAll('.agents-main button')].find(button=>button.textContent==='Открыть').click();
 await settle();verify();
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

test('agents live inside chat navigation with an icon and selected subviews',async t=>{
 const {w}=fixture(t);const button=w.document.querySelector('[data-chat-app="agents"]');
 assert.equal(w.document.querySelector('[data-app="agents"]'),null);assert.ok(button.querySelector('svg'));assert.equal(button.lastElementChild.textContent,'Агенты');assert.equal(button.getAttribute('role'),'tab');assert.ok(w.document.querySelector('#app-chat-workspace #app-agents'));
 w.showApp('agents');await settle();assert.equal(w.document.querySelector('[data-view="catalog"]').getAttribute('aria-current'),'page');
 w.document.querySelector('[data-view="skills"]').click();await settle();assert.equal(w.document.querySelector('[data-view="skills"]').getAttribute('aria-current'),'page');assert.equal(w.document.querySelector('[data-view="catalog"]').hasAttribute('aria-current'),false);
});
test('empty catalog gives the administrator a direct model setup action',async t=>{
 const {w}=fixture(t,true,true);w.showApp('agents');await settle();
 assert.ok(w.document.querySelector('.agents-empty'));assert.equal(w.document.querySelector('.agents-search'),null);
 const connect=[...w.document.querySelectorAll('.agents-empty button')].find(b=>b.textContent==='Подключить модель');assert.ok(connect);connect.click();await settle();assert.ok(w.document.querySelector('dialog').open);
 assert.equal(w.document.querySelector('dialog input[type="password"]').value,'');
});

test('Ollama and OpenAI presets select native connection fields without carrying another key',async t=>{
 for(const kind of ['ollama','openai']){
  const {w,requests}=fixture(t,true);w.showApp('agents');await settle();w.document.querySelector('[data-view="settings"]').click();await settle();
  [...w.document.querySelectorAll('button')].find(b=>b.textContent==='+ Подключить модель').click();await settle();
  const dialog=w.document.querySelector('dialog'),labels=[...dialog.querySelectorAll('label')],field=text=>labels.find(el=>el.firstChild.textContent===text).querySelector('input,select');
  const key=dialog.querySelector('input[type="password"]');key.value='must-not-follow-provider-switch';const select=field('Провайдер');select.value=kind;select.dispatchEvent(new w.Event('change'));
  assert.equal(key.value,'');assert.equal(key.required,kind==='openai');
  assert.equal(field('Базовый URL API').value,kind==='ollama'?'http://127.0.0.1:11434':'https://api.openai.com/v1');
  if(kind==='ollama'){assert.equal(field('Идентификатор модели').value,'');field('Идентификатор модели').value='fixture-local:latest';}
  else{assert.equal(field('Идентификатор модели').value,'gpt-6.2-sol');assert.equal(field('Базовый URL API').readOnly,true);key.value='openai-fixture-key';}
  dialog.querySelector('form').dispatchEvent(new w.Event('submit',{cancelable:true}));await settle();
  const put=requests.find(r=>r.url.endsWith('/config')&&r.options.method==='PUT');assert.ok(put);const provider=JSON.parse(put.options.body).providers[0];assert.equal(provider.kind,kind);assert.equal(provider.thinking,false);assert.equal(provider.api_key,kind==='ollama'?'':'openai-fixture-key');
 }
});

test('web switch persists on the dialog and delivery confirms the exact stored action',async t=>{
 const {w,requests}=fixture(t,false,false,true);w.showApp('agents');await settle();[...w.document.querySelectorAll('button')].find(b=>b.textContent==='Начать диалог').click();await settle();
 const web=w.document.querySelector('.agents-composer [aria-label="Веб-поиск"]');assert.ok(web);web.click();await settle();
 const patch=requests.find(r=>r.options.method==='PATCH');assert.deepEqual(JSON.parse(patch.options.body),{web_enabled:true});assert.equal(web.getAttribute('aria-pressed'),'true');
 const card=w.document.querySelector('.agents-approvals');assert.match(card.textContent,/recipient@example.test.*Exact proposed body/);assert.equal(card.querySelector('img'),null);
 card.querySelector('button').click();await settle();const confirmation=requests.find(r=>r.url.endsWith('/conversations/c1/actions/a1'));assert.ok(confirmation);assert.equal(confirmation.options.body,'{}');assert.equal(card.children.length,0);
});

test('provider catalog is discoverable before saving and selects the exact returned ID',async t=>{
 const {w,requests}=fixture(t,true,true);w.showApp('agents');await settle();
 w.document.querySelector('.agents-empty .agents-primary').click();await settle();
 const dialog=w.document.querySelector('dialog');
 const provider=dialog.querySelector('[aria-label="Провайдер"]');
 provider.value='ollama';provider.dispatchEvent(new w.Event('change'));
 [...dialog.querySelectorAll('button')].find(button=>button.textContent==='Проверить подключение').click();await settle();
 assert.ok(requests.some(req=>req.url.endsWith('/config/models')&&req.options.method==='POST'));
 assert.ok(!requests.some(req=>req.options.method==='PUT'));
 const picker=dialog.querySelector('[aria-label="Модели провайдера"]');
 assert.equal(picker.parentElement.classList.contains('hidden'),false);
 picker.value='catalog-second';picker.dispatchEvent(new w.Event('change'));
 dialog.querySelector('form').dispatchEvent(new w.Event('submit',{cancelable:true}));await settle();
 const saved=requests.find(req=>req.options.method==='PUT');
 assert.equal(JSON.parse(saved.options.body).providers[0].model,'catalog-second');
});

test('agent composer uploads a file and recorded audio before posting to the private dialog',async t=>{
 const {w,requests}=fixture(t);let stops=0;
 const stream={getTracks:()=>[{stop(){stops++;}}]};
 Object.defineProperty(w.navigator,'mediaDevices',{value:{getUserMedia:async()=>stream}});
 w.MediaRecorder=class{
  constructor(stream){this.stream=stream;this.mimeType='audio/webm';this.state='inactive';}
  start(){this.state='recording';}
  stop(){this.state='inactive';this.ondataavailable({data:new w.Blob(['test audio'],{type:this.mimeType})});this.onstop();}
 };
 w.showApp('agents');await settle();
 [...w.document.querySelectorAll('button')].find(button=>button.textContent==='Начать диалог').click();await settle();
 const composer=w.document.querySelector('.agents-composer'),input=composer.querySelector('input[type=file]');
 Object.defineProperty(input,'files',{value:[new w.File(['text'],'brief.txt')]});
 input.dispatchEvent(new w.Event('change'));
 assert.match(composer.querySelector('.agents-draft-files').textContent,/brief.txt/);
 composer.querySelector('[aria-label="Записать аудио"]').click();await settle();
 assert.equal(composer.querySelector('[aria-label="Отправить"]').disabled,true);
 composer.querySelector('[aria-label="Остановить запись"]').click();await settle();
 assert.equal(stops,1);
 assert.match(composer.querySelector('.agents-draft-files').textContent,/voice-message-.+\.webm/);
 composer.dispatchEvent(new w.Event('submit',{cancelable:true}));await settle();
 const upload=requests.find(req=>req.url==='/api/v4/files');
 assert.equal(upload.options.body.get('channel_id'),'dm1');
 assert.equal(upload.options.body.getAll('files').length,2);
 const turn=requests.find(req=>req.url.endsWith('/messages'));
 assert.deepEqual(JSON.parse(turn.options.body).file_ids,['file0','file1']);
 assert.ok(requests.indexOf(upload)<requests.indexOf(turn));
 assert.equal(composer.querySelector('.agents-draft-files').hidden,true);
});
