import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';

// Only a disposable seeded HTTPS loopback server, with xmpp.websocket enabled.
const base = new URL(process.env.TMS_URL || 'https://localhost:18443');
assert.equal(base.protocol, 'https:');
assert.equal(base.hostname, 'localhost');
const browser = await chromium.launch({executablePath:process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const context = await browser.newContext({ignoreHTTPSErrors:true});
 const page = await context.newPage();
 await page.goto(base.href);
 await page.fill('#email','admin@example.com');
 await page.fill('#password','changeme');
 await page.click('#form-login button[type=submit]');
 await page.waitForSelector('#view-account:not(.hidden)');
 const result = await page.evaluate(async()=>{
  const framing='urn:ietf:params:xml:ns:xmpp-framing',stream='http://etherx.jabber.org/streams';
  const check=(ok,message)=>{if(!ok)throw new Error(message)};
  async function connect(){
   const url=new URL('/xmpp-websocket',location.href);url.protocol='wss:';
   const ws=new WebSocket(url,'xmpp'),queue=[];let wake,failed;
   ws.onmessage=e=>{queue.push(e.data);wake?.();wake=null};
   ws.onclose=()=>{failed=new Error('WebSocket closed');wake?.();wake=null};
   await new Promise((resolve,reject)=>{ws.onopen=resolve;ws.onerror=()=>reject(new Error('WebSocket handshake failed'))});
   check(ws.protocol==='xmpp','subprotocol not negotiated');
   async function next(){
    while(!queue.length){
     if(failed)throw failed;
     await new Promise((resolve,reject)=>{
      const timer=setTimeout(()=>{wake=null;reject(new Error('WebSocket receive timeout'))},5000);
      wake=()=>{clearTimeout(timer);resolve()};
     });
    }
    const d=new DOMParser().parseFromString(queue.shift(),'application/xml');
    check(!d.querySelector('parsererror'),'invalid standalone XML');return d.documentElement;
   }
   const send=xml=>ws.send(xml);
   async function want(local){let e=await next();while(e.localName==='r')e=await next();check(e.localName===local,`expected ${local}, got ${e.localName}`);return e}
   async function open(){
    send(`<open xmlns='${framing}' to='example.com' version='1.0'/>`);
    check((await want('open')).namespaceURI===framing,'framing namespace');
    const features=await want('features');check(features.namespaceURI===stream,'stream namespace');check(!features.querySelector('starttls'),'STARTTLS on WSS');
   }
   async function login(resource){
    await open();send(`<auth xmlns='urn:ietf:params:xml:ns:xmpp-sasl' mechanism='PLAIN'>${btoa('\0admin@example.com\0changeme')}</auth>`);
    await want('success');await open();
    if(resource){send(`<iq xmlns='jabber:client' type='set' id='bind'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'><resource>${resource}</resource></bind></iq>`);check((await want('iq')).textContent.includes('/'+resource),'binding failed')}
   }
   return {ws,send,next,want,open,login};
  }
  const noAuth=await connect();await noAuth.open();
  noAuth.send(`<iq xmlns='jabber:client' type='set'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'/></iq>`);
  check((await noAuth.want('error')).querySelector('not-authorized'),'web cookie bypassed SASL');noAuth.ws.close();
  const first=await connect();await first.login('wss-browser-fixture');
  first.send(`<enable xmlns='urn:xmpp:sm:3' resume='true'/>`);
  const token=(await first.want('enabled')).getAttribute('id');check(token,'missing resume token');
  first.send(`<iq xmlns='jabber:client' type='get' id='ping'><ping xmlns='urn:xmpp:ping'/></iq>`);
  check((await first.want('iq')).getAttribute('id')==='ping','ping failed');
  // Do not acknowledge: the replacement transport must replay exactly this IQ.
  const second=await connect();await second.login('');
  second.send(`<resume xmlns='urn:xmpp:sm:3' previd='${token}' h='0'/>`);
  check((await second.want('resumed')).getAttribute('previd')===token,'resume failed');
  check((await second.want('iq')).getAttribute('id')==='ping','replay lost or changed IQ');
  second.send(`<a xmlns='urn:xmpp:sm:3' h='1'/>`);
  second.send(`<close xmlns='${framing}'/>`);await second.want('close');second.ws.close();first.ws.close();
  return 'Browser WSS: same-origin handshake, explicit SASL despite web login, bind, ping and resumable replay — OK';
 });
 console.log(result);
} finally {await browser.close()}
