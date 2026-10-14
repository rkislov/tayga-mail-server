// No network or real notifications: exercise worker routing in an isolated context.
const {readFileSync} = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const handlers = {};
let windows = [], shown = [], opened = [];
const self = {location:{origin:'https://mail.example.com'},addEventListener:(name,fn)=>handlers[name]=fn,
 registration:{showNotification:async (...args)=>shown.push(args)},
 clients:{matchAll:async()=>windows,openWindow:async url=>opened.push(url),claim:async()=>{}},skipWaiting:()=>{}};
vm.runInNewContext(readFileSync('frontend/src/sw.js','utf8'),{self,URL});
async function emit(name, args) {let done;handlers[name]({...args,waitUntil:p=>done=p});await done;}
(async()=>{
 await emit('push',{data:{json:()=>({title:'Новое письмо',href:'https://evil.test',tag:'1'})}});
 assert.equal(shown.length,1);assert.equal(shown[0][1].data.href,'mail');
 let closed=false;await emit('notificationclick',{notification:{close:()=>closed=true,data:{href:'chat'}}});
 assert.equal(closed,true);assert.equal(opened[0],'/?notification=chat');
 let message,focused=false;windows=[{url:'https://mail.example.com/',visibilityState:'visible',focus:async()=>focused=true,postMessage:m=>message=m}];
 await emit('push',{data:{json:()=>({title:'Новое сообщение',href:'chat'})}});assert.equal(shown.length,1);
 await emit('notificationclick',{notification:{close:()=>{},data:{href:'chat'}}});assert.equal(focused,true);assert.equal(message.href,'chat');
 await emit('push',{data:{json:()=>{throw Error('invalid')}}});assert.equal(shown.length,1);
 console.log('Web Push worker routing and visibility checks passed');
})().catch(error=>{console.error(error);process.exitCode=1});
