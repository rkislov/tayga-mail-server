const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {JSDOM} = require('jsdom');
const root = path.join(__dirname,'../..');
const tick = () => new Promise(resolve => setImmediate(resolve));
async function settle() {for(let i=0;i<12;i++) await tick();}
function fixture(t) {
 const dom = new JSDOM(fs.readFileSync(path.join(root,'internal/frontend/dist/index.html'),'utf8'),{url:'https://chat.example.test/',runScripts:'outside-only',pretendToBeVisual:true});
 const w=dom.window, requests=[], sockets=[], timers=[];
 w.HTMLDialogElement.prototype.showModal=function(){this.open=true;};
 w.HTMLDialogElement.prototype.close=function(){if(this.open){this.open=false;this.dispatchEvent(new w.Event('close'));}};
 w.document.hasFocus=()=>true;
 w.document.getElementById('app-chat').classList.remove('hidden');
 const me={id:'alice-id',username:'alice',nickname:'Alice'}, bob={id:'bob-id',username:'bob',nickname:'Боб'};
 const channel={id:'town-id',type:'O',name:'town-square',display_name:'Town Square',total_msg_count:1,last_post_at:100};
 const direct={id:'direct-id',type:'D',name:'alice-id__bob-id',display_name:'',total_msg_count:0,last_post_at:0};
 const history=[{id:'first-post',user_id:bob.id,channel_id:channel.id,message:'<img src=x onerror=alert(1)>',root_id:'',create_at:100}];
 const model={me,bob,channels:[channel,direct],history,failSend:false,deferHistory:null};
 class Socket {
  static OPEN=1;
  constructor(url){this.url=String(url);this.readyState=1;this.sent=[];sockets.push(this);}
  send(data){this.sent.push(JSON.parse(data));}
  close(){this.readyState=3;this.onclose?.();}
  async emit(message){await this.onmessage?.({data:JSON.stringify(message)});}
 }
 Object.assign(w,{$:id=>w.document.getElementById(id),state:{email:'alice@example.test',tokens:{access_token:'secret-session-token'}},lang:'ru',WebSocket:Socket,isMobilePane:()=>false,setMobilePane:()=>{},setMsg:(node,text)=>{if(node)node.textContent=text;}});
 w.setInterval=()=>1;w.clearInterval=()=>{};w.setTimeout=(callback,delay)=>{timers.push({callback,delay});return timers.length;};w.clearTimeout=()=>{};
 w.api=async function(route,options={}) {
  options._guard?.();requests.push({route,options});
  let result;
  if(route==='/api/v4/users/me')result=me;
  else if(route==='/api/v4/users/me/teams')result=[{id:'team-id'}];
  else if(route.endsWith('/channels/members'))result=model.channels.map(c=>({channel_id:c.id,msg_count:0}));
  else if(route==='/api/v4/users/me/teams/team-id/channels')result=model.channels;
  else if(route==='/api/v4/users/ids')result=[me,bob].filter(u=>JSON.parse(options.body).includes(u.id));
  else if(route==='/api/v4/users/search')result=[me,bob];
  else if(route.startsWith('/api/v4/users/email/'))result=bob;
  else if(route==='/api/v4/channels/direct')result=direct;
  else if(route.startsWith('/api/v4/channels/')&&route.includes('/posts?')){
   if(model.deferHistory)result=await model.deferHistory;
   else {const url=new URL(route,w.location.href);const p=Number(url.searchParams.get('page'));const since=Number(url.searchParams.get('since')||0);const list=model.history.filter(post=>post.channel_id===url.pathname.split('/')[4]&&post.create_at>since).sort((a,b)=>b.create_at-a.create_at).slice(p*80,(p+1)*80);result={order:list.map(p=>p.id),posts:Object.fromEntries(list.map(p=>[p.id,p]))};}
  }
  else if(route==='/api/v4/channels/members/me/view')result={status:'OK'};
  else if(route==='/api/v4/posts'){
   if(model.failSend){model.failSend=false;throw Error('network failed');}
   const payload=JSON.parse(options.body);result={...payload,id:'sent-post',user_id:me.id,create_at:200};model.history.push(result);
  } else throw Error('Unexpected request '+route);
  options._guard?.();return structuredClone(result);
 };
 w.eval(fs.readFileSync(path.join(root,'frontend/src/mattermost-chat.js'),'utf8')+'\nwindow.chat=mmChat;');
 t.after(()=>{w.chat.stop();w.close();});
 return {w,model,requests,sockets,timers,el:id=>w.document.getElementById('mm-chat-'+id)};
}
async function selectTown(f){await f.w.chat.refresh();[...f.el('channels').querySelectorAll('button')].find(b=>b.textContent.includes('Town Square')).click();await settle();}

test('shared chat uses v4 history, readable directory and safe text rendering',async t=>{
 const f=fixture(t);await selectTown(f);
 assert.equal(f.el('posts').querySelectorAll('article').length,1);
 assert.equal(f.el('posts').querySelector('img'),null);
 assert.match(f.el('posts').textContent,/<img src=x/);
 assert.match(f.el('channels').textContent,/Боб/);
 assert.ok(f.requests.every(r=>r.route.startsWith('/api/v4/')));
 assert.equal(f.sockets.length,1);assert.ok(!f.sockets[0].url.includes('secret-session-token'));
 f.sockets[0].onopen();assert.equal(f.sockets[0].sent[0].action,'authentication_challenge');
 assert.equal(f.sockets[0].sent[0].data.token,'secret-session-token');
});

test('send retry keeps idempotency key and WebSocket duplicate is displayed once',async t=>{
 const f=fixture(t);await selectTown(f);f.model.failSend=true;
 f.el('body').value='Привет с Mac';
 const submit=()=>f.el('send').dispatchEvent(new f.w.Event('submit',{cancelable:true}));
 submit();await settle();assert.equal(f.el('body').value,'Привет с Mac');
 submit();await settle();assert.equal(f.el('body').value,'');
 const calls=f.requests.filter(r=>r.route==='/api/v4/posts').map(r=>JSON.parse(r.options.body));
 assert.equal(calls.length,2);assert.equal(calls[0].pending_post_id,calls[1].pending_post_id);
 await f.sockets[0].emit({event:'posted',data:{post:JSON.stringify(f.model.history.find(p=>p.id==='sent-post'))}});
 assert.equal(f.el('posts').querySelectorAll('[data-post-id="sent-post"]').length,1);
});

test('reconnect recovers more than one page of missed messages',async t=>{
 const f=fixture(t);await selectTown(f);
 for(let i=0;i<180;i++)f.model.history.push({id:'missed-'+i,channel_id:'town-id',user_id:'bob-id',message:'Recovered '+i,root_id:'',create_at:101+i});
 f.sockets[0].close();const reconnect=f.timers.find(timer=>timer.delay===2000);assert.ok(reconnect);await reconnect.callback();
 assert.equal(f.sockets.length,2);await f.sockets[1].emit({event:'hello',data:{}});
 assert.equal(f.el('posts').querySelectorAll('article').length,181);
 assert.ok(f.requests.filter(r=>r.route.includes('since=99')).length>=3);
});

test('logout discards an in-flight history response and all chat data',async t=>{
 const f=fixture(t);await f.w.chat.refresh();let resolve;
 f.model.deferHistory=new Promise(r=>resolve=r);
 [...f.el('channels').querySelectorAll('button')].find(b=>b.textContent.includes('Town Square')).click();await tick();
 f.w.chat.stop();f.w.state.email='another@example.test';
 resolve({order:['private'],posts:{private:{id:'private',channel_id:'town-id',user_id:'bob-id',message:'Must not appear',create_at:500}}});await settle();
 assert.equal(f.el('posts').textContent,'');assert.equal(f.el('channels').textContent,'');
 assert.equal(f.el('thread').classList.contains('hidden'),true);
});
