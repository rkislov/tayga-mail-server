const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {JSDOM}=require('jsdom');
const source=fs.readFileSync(path.join(__dirname,'../src/mattermost-products.js'),'utf8');
const settle=async()=>{for(let i=0;i<8;i++)await new Promise(r=>setImmediate(r));};
function fixture(t){
 const dom=new JSDOM('<nav><button data-app="chat">Chat</button></nav><main><section id="app-chat"></section></main>',{url:'https://tms.test',runScripts:'outside-only'});
 const w=dom.window, requests=[];
 w.HTMLDialogElement.prototype.showModal=function(){this.open=true;};
 w.HTMLDialogElement.prototype.close=function(){this.open=false;this.dispatchEvent(new w.Event('close'));};
 w.state={tokens:{access_token:'first'}};w.APPS=['chat'];w.showApp=name=>{for(const app of w.APPS)w.document.getElementById('app-'+app)?.classList.toggle('hidden',app!==name);};
 let list=[{id:'boards',user_id:'alice',title:'Boards',url:'/mm/'+'a'.repeat(26)+'/boards'}];
 let resolve;
 w.fetch=async(url,options={})=>{requests.push({url,options});if(options.method==='DELETE')return {ok:true};if(resolve)return new Promise(r=>{resolve.callback=()=>r({ok:true,json:async()=>list});});return {ok:true,json:async()=>list};};
 w.eval(fs.readFileSync(require('node:path').join(__dirname,'../src/chat-workspace.js'),'utf8'));w.eval(source);t.after(()=>w.close());
 return {w,requests,setList:value=>{list=value;},defer:()=>{resolve={};return()=>{resolve.callback();resolve=null;}},refresh:token=>{w.state.tokens={access_token:token};w.showApp('chat');}};
}
test('products retain frame on token rotation and remove it on identity change',async t=>{
 const f=fixture(t);await settle();f.w.showApp('boards');
 const old=f.w.document.querySelector('iframe');assert.match(old.src,/\/boards$/);
 f.refresh('renewed');await settle();assert.equal(f.w.document.querySelector('iframe'),old);
 f.setList([{id:'boards',user_id:'bob',title:'Boards',url:'/mm/'+'a'.repeat(26)+'/boards'}]);f.refresh('second-user');await settle();
 assert.notEqual(f.w.document.querySelector('iframe'),old);assert.equal(old.isConnected,false);
 assert.ok(f.requests.some(r=>r.options.method==='DELETE'&&r.url.endsWith('/products/session')));
 f.w.dispatchEvent(new f.w.Event('tms-products-session-ended'));
 assert.equal(f.w.document.querySelector('iframe'),null);
});
test('catalog cannot inject cross-origin frames and late response cannot resurrect logged-out session',async t=>{
 const f=fixture(t);await settle();
 f.setList([{id:'playbooks',title:'unsafe',url:'https://other.test/playbooks',user_id:'alice'}]);f.refresh('new');await settle();
 assert.equal(f.w.document.querySelectorAll('iframe').length,0);
 const finish=f.defer();f.refresh('pending');f.w.state.tokens=null;f.w.dispatchEvent(new f.w.Event('tms-products-session-ended'));finish();await settle();
 assert.equal(f.w.document.querySelectorAll('iframe').length,0);
});

test('products are chat tabs, preserve frames and drafts, support keyboard navigation and leave mail alone',async t=>{
 const f=fixture(t),w=f.w;f.setList([{id:'boards',user_id:'alice',title:'Boards',url:'/mm/'+'a'.repeat(26)+'/boards'},{id:'playbooks',user_id:'alice',title:'Playbooks',url:'/mm/'+'a'.repeat(26)+'/playbooks'}]);f.refresh('loaded');await settle();
 const mainNav=w.document.querySelector('body>nav');assert.equal(mainNav.querySelectorAll('button').length,1);
 const shell=w.document.getElementById('app-chat-workspace');assert.ok(shell.querySelector('#app-boards'));assert.ok(shell.querySelector('#app-playbooks'));
 assert.deepEqual([...shell.querySelectorAll('[role="tab"]')].map(b=>b.textContent),['Диалоги','Доски','Сценарии']);
 const draft=w.document.createElement('textarea');draft.value='unsent private draft';w.document.getElementById('app-chat').append(draft);
 w.document.querySelector('[data-chat-app="boards"]').click();const frame=w.document.querySelector('#app-boards iframe');assert.equal(w.document.querySelector('[data-chat-app="boards"]').getAttribute('aria-selected'),'true');
 assert.equal(w.document.getElementById('app-chat').classList.contains('hidden'),true);assert.equal(shell.classList.contains('hidden'),false);
 const boardTab=w.document.querySelector('[data-chat-app="boards"]');boardTab.dispatchEvent(new w.KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true}));
 assert.equal(w.document.querySelector('[data-chat-app="playbooks"]').getAttribute('aria-selected'),'true');assert.equal(w.document.activeElement.dataset.chatApp,'playbooks');
 w.showApp('chat');w.showApp('boards');assert.equal(w.document.querySelector('#app-boards iframe'),frame);assert.equal(draft.value,'unsent private draft');
 w.showApp('mail');assert.equal(shell.classList.contains('hidden'),true);
});

test('product language follows Tayga and its source shortcut creates a private board', async t => {
 const f=fixture(t),w=f.w;
 await settle();
 w.showApp('boards');
 const frame=w.document.querySelector('iframe');
 w.document.documentElement.lang='en';
 await settle();
 assert.match(frame.src,/\/boards\?lang=en$/);
 assert.ok(w.document.querySelector('[data-chat-app="boards"]').textContent.includes('Boards'));

 const menu=w.document.createElement('div');w.document.body.append(menu);
 let discussed=false;
 w.tmsProductActions.appendActions(menu,{title:'  QA review of launch  ',discuss:()=>{discussed=true;}});
 const buttons=[...menu.querySelectorAll('button')];
 assert.equal(buttons[0].textContent,'Create board');
 assert.equal(buttons[1].hidden,true);
 buttons[2].click();assert.equal(discussed,true);
 const calls=[],team='t'.repeat(26),board='b'.repeat(27);
 w.api=async(url,opts)=>{
   calls.push([url,opts]);
   if(url.endsWith('/bootstrap'))return {team:{id:team}};
   const payload=JSON.parse(opts.body);
   assert.equal(payload.boards[0].type,'P');
   assert.equal(payload.boards[0].title,'QA review of launch');
   assert.equal(payload.boards[0].description,'');
   assert.equal(payload.blocks[0].boardId,payload.boards[0].id);
   return {boards:[{id:board}]};
 };
 buttons[0].click();
 const dialog=w.document.querySelector('.product-create-dialog');
 assert.equal(dialog.querySelector('input').value,'QA review of launch');
 dialog.querySelector('form').dispatchEvent(new w.Event('submit',{cancelable:true}));
 await settle();
 assert.equal(calls.length,2);
 assert.match(calls[1][0],/\/plugins\/focalboard\/api\/v2\/boards-and-blocks$/);
 assert.ok(frame.src.endsWith('/boards/team/'+team+'/'+board+'?lang=en'));
 w.document.documentElement.lang='ru';
 await settle();
 assert.ok(frame.src.endsWith('/boards/team/'+team+'/'+board));
});

test('Boards translates imported template IDs and saves a translated copy without changing a user template',async t=>{
 const scope='/mm/'+'s'.repeat(26);
 const dom=new JSDOM('<html lang="ru" data-tayga-product="boards"></html>',{url:'https://tms.test'+scope+'/boards',runScripts:'outside-only'});
 const w=dom.window;
 t.after(()=>w.close());
 w.Request=class extends Request {constructor(input,init){super(input instanceof Request?input:new URL(input,w.location.href),init);}};
 w.Response=Response;w.Headers=Headers;
 const root=scope+'/plugins/focalboard/api/v2', templateID='b'+'x'.repeat(26), copyID='b'+'y'.repeat(26);
 const board={id:templateID,teamId:'0',isTemplate:true,title:'Competitive Analysis',cardProperties:[{id:'property',name:'Website'}]};
 const custom={...board,teamId:'g'.repeat(26),id:'b'+'z'.repeat(26)};
 let saved;
 w.fetch=async(input,init)=>{
   const req=input instanceof Request?input:new w.Request(input,init);
   const pathname=new URL(req.url).pathname;
   if(pathname===root+'/teams/'+custom.teamId+'/templates')return Response.json([board,custom]);
   if(pathname===root+'/boards/'+templateID+'/blocks')return Response.json([{id:'card',type:'card',title:'Team Schedule'}]);
   if(pathname===root+'/boards/'+templateID+'/duplicate')return Response.json({boards:[{...board,id:copyID,teamId:custom.teamId,isTemplate:false}],blocks:[{id:'card',type:'card',title:'Team Schedule'}]});
   if(pathname===root+'/boards-and-blocks'&&req.method==='PATCH'){saved=await req.json();return Response.json({});}
   throw Error('Unexpected API '+req.method+' '+pathname);
 };
 const assets=path.join(__dirname,'../../internal/frontend/dist/assets/');
 for(const file of ['tayga-boards-templates-ru.js','tayga-product-frame.js'])w.eval(fs.readFileSync(path.join(assets,file),'utf8'));
 const templates=await (await w.fetch(root+'/teams/'+custom.teamId+'/templates')).json();
 assert.equal(templates[0].title,'Анализ конкурентов');
 assert.equal(templates[0].cardProperties[0].name,'Сайт');
 assert.equal(templates[1].title,'Competitive Analysis');
 const blocks=await (await w.fetch(root+'/boards/'+templateID+'/blocks')).json();
 assert.equal(blocks[0].title,'Расписание команды');
 const duplicate=await (await w.fetch(root+'/boards/'+templateID+'/duplicate',{method:'POST'})).json();
 assert.equal(duplicate.boards[0].title,'Анализ конкурентов');
 assert.equal(saved.boardIDs[0],copyID);
 assert.equal(saved.blockPatches[0].title,'Расписание команды');
});
