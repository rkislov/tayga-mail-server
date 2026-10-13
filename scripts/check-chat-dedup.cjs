const fs=require('fs'),vm=require('vm'),assert=require('assert');
const source=fs.readFileSync('frontend/src/app.js','utf8');
const box={children:[],querySelector:()=>null,appendChild(x){this.children.push(x)}};
let submit,requests=0,resolveRequest;
const input={value:'hello'},button={disabled:false};
const context={chatState:{roster:[],peer:'bob@example.com',me:'alice@example.com',sending:false},state:{},document:{createElement:()=>({dataset:{}})},Array,JSON,Date,escapeHtml:x=>x,chatBare:x=>String(x||'').split('/')[0].toLowerCase(),$:id=>id==='chat-messages'?box:id==='chat-body'?input:id==='form-chat-send'?{addEventListener:(_,fn)=>submit=fn}:null,api:()=>{requests++;return new Promise(resolve=>resolveRequest=resolve)},omemoEnabled:async()=>false,setMsg:()=>{},scrollChatBottom:()=>{}};
vm.createContext(context);
vm.runInContext(source.slice(source.indexOf('function appendChatBubble'),source.indexOf('function scrollChatBottom')),context);
vm.runInContext(source.slice(source.indexOf('$("form-chat-send")?.addEventListener'),source.indexOf('function filesGoUp')),context);
(async()=>{
const e={preventDefault(){},currentTarget:{querySelector:()=>button}};
const ev={id:'one',from:'alice@example.com/web',to:'bob@example.com',body:'hello'};
const pending=submit(e);await submit(e);assert.equal(requests,1);
context.appendChatBubble(ev);resolveRequest(ev);await pending;assert.equal(box.children.length,1);
context.appendChatBubble(ev);assert.equal(box.children.length,1);
context.appendChatBubble({...ev,id:'two'});assert.equal(box.children.length,2);
context.appendChatBubble({...ev,from:'bob@example.com'});assert.equal(box.children.length,3);
box.children=[];context.appendChatBubble(ev);assert.equal(box.children.length,1);
console.log('Chat: HTTP/SSE deduplication, repeated submit and identical text with distinct IDs — OK');
})().catch(e=>{console.error(e);process.exitCode=1});

// Incoming SSE events notify once; the sender's own echo stays silent.
{
 const toasts=[];let onMessage;
 const c={state:{tokens:{access_token:'test'},email:'alice@example.com'},chatState:{me:'alice@example.com',peer:'',roster:[{jid:'bob@example.com',name:'Bob'},{jid:'team@web-groups.example.com',name:'Team',group:true}]},EventSource:class{addEventListener(type,fn){if(type==='message')onMessage=fn}},stopChatLive(){},chatBare:x=>String(x||'').split('/')[0].toLowerCase(),showToast:e=>toasts.push(e),renderChatRoster(){},refreshChat(){},JSON,encodeURIComponent};
 vm.createContext(c);vm.runInContext(source.slice(source.indexOf('function startChatLive()'),source.indexOf('function appendChatBubble')),c);c.startChatLive();
 onMessage({data:JSON.stringify({type:'message',id:'1',from:'bob@example.com/mobile',to:'alice@example.com',body:'hello'})});
 onMessage({data:JSON.stringify({type:'message',id:'2',from:'alice@example.com/web',to:'bob@example.com',body:'reply'})});
 onMessage({data:JSON.stringify({type:'message',id:'3',from:'bob@example.com',to:'team@web-groups.example.com',room:'team@web-groups.example.com',body:'team'})});
 assert.equal(toasts.length,2);assert.equal(toasts[1].title,'Team');assert.match(toasts[1].href,/chat\?peer=/);console.log('Chat: incoming direct/group notifications and silent own echo — OK');
}

(async()=>{
 let onOpen;
 const c={state:{tokens:{access_token:'test'}},chatState:{roster:[{jid:'bob@example.com',subscription_request:true}]},EventSource:class {addEventListener(type,fn){if(type==='open')onOpen=fn}},stopChatLive(){},api:async()=>({roster:[{jid:'bob@example.com',subscription:'both',subscription_request:false}]}),renderChatRoster(){},encodeURIComponent};
 vm.createContext(c);vm.runInContext(source.slice(source.indexOf('function startChatLive()'),source.indexOf('function appendChatBubble')),c);c.startChatLive();await onOpen();assert.equal(c.chatState.roster[0].subscription_request,false);console.log('Chat: reconnect refreshes accepted presence requests — OK');
})().catch(e=>{console.error(e);process.exitCode=1});
