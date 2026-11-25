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
 w.state={tokens:{access_token:'first'}};w.APPS=['chat'];w.showApp=()=>{};
 let list=[{id:'boards',user_id:'alice',title:'Boards',url:'/mm/'+'a'.repeat(26)+'/boards'}];
 let resolve;
 w.fetch=async(url,options={})=>{requests.push({url,options});if(options.method==='DELETE')return {ok:true};if(resolve)return new Promise(r=>{resolve.callback=()=>r({ok:true,json:async()=>list});});return {ok:true,json:async()=>list};};
 w.eval(source);t.after(()=>w.close());
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
