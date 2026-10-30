import {chromium} from 'playwright-core';
import assert from 'node:assert/strict';

// Mutating integration checks are restricted to a disposable seeded loopback server.
const base=process.env.TMS_URL||'http://127.0.0.1:18090';
assert(['127.0.0.1','localhost','[::1]'].includes(new URL(base).hostname));
const browser=await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const context=await browser.newContext({viewport:{width:1280,height:1000}}),page=await context.newPage(),errors=[];
 page.on('pageerror',e=>errors.push(e.message));
 await page.goto(base);await page.fill('#email','admin@example.com');await page.fill('#password','changeme');await page.click('#form-login button[type=submit]');await page.waitForSelector('#view-account:not(.hidden)');
 const fixture=await page.evaluate(async()=>{
  const email=`archive-${Date.now()}@example.com`;
  await api('/api/v1/admin/users',{method:'POST',body:JSON.stringify({email,password:'fixture-password'})});
  const room=await api('/api/v1/chat/rooms',{method:'POST',body:JSON.stringify({name:'Команда · Архив',members:[email]})});
  const other=await api('/api/v1/chat/rooms',{method:'POST',body:JSON.stringify({name:'Другой чат',members:[email]})});
  const ids=[];for(let i=0;i<257;i++){const ev=await api('/api/v1/chat/send',{method:'POST',body:JSON.stringify({to:room.jid,body:`Сообщение ${String(i).padStart(3,'0')} · история команды`})});ids.push(ev.id)}
  await api('/api/v1/chat/send',{method:'POST',body:JSON.stringify({to:other.jid,body:'Другой разговор'})});
  await refreshChat();showApp('chat');return {room:room.jid,other:other.jid,ids};
 });
 await page.evaluate(room=>openChatPeer(room),fixture.room);
 const bubbles=page.locator('#chat-messages .chat-bubble'),older=page.locator('.chat-history-older');
 assert.equal(await bubbles.count(),80);
 for(const expected of [160,240,257]){await older.click();await page.waitForFunction(n=>document.querySelectorAll('#chat-messages .chat-bubble').length===n,expected)}
 assert.equal(await older.isVisible(),false);
 const ids=await bubbles.evaluateAll(nodes=>nodes.map(n=>JSON.parse(n.dataset.messageKey)[1]));assert.deepEqual(ids,fixture.ids);
 await page.screenshot({path:'/private/tmp/tayga-room-archive-desktop.png'});
 await page.setViewportSize({width:390,height:844});await page.evaluate(room=>openChatPeer(room),fixture.room);
 await older.scrollIntoViewIfNeeded();assert(await older.isVisible());
 assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'Mobile page overflow');
 await page.screenshot({path:'/private/tmp/tayga-room-archive-mobile.png'});
 // Delay a real response, switch peers, then deliver the old room page.
 let release,arrived;let responseReady=new Promise(resolve=>arrived=resolve);const responseGate=new Promise(resolve=>release=resolve);
 const matcher=url=>url.pathname==='/api/v1/chat/rooms/history'&&url.searchParams.get('room')===fixture.room;
 await page.route(matcher,async route=>{const response=await route.fetch();arrived();await responseGate;await route.fulfill({response})});
 const stale=page.evaluate(room=>openChatPeer(room),fixture.room);await responseReady;
 await page.evaluate(room=>openChatPeer(room),fixture.other);release();await stale;await page.unroute(matcher);
 assert.equal(await bubbles.count(),1);assert((await bubbles.innerText()).includes('Другой разговор'));
 // A live message during initial load remains after the older page arrives.
 let releaseLive,arrivedLive;responseReady=new Promise(resolve=>arrivedLive=resolve);const liveGate=new Promise(resolve=>releaseLive=resolve);
 await page.route(matcher,async route=>{const response=await route.fetch();arrivedLive();await liveGate;await route.fulfill({response})});
 const opening=page.evaluate(room=>openChatPeer(room),fixture.room);await responseReady;
 const live=await page.evaluate(room=>api('/api/v1/chat/send',{method:'POST',body:JSON.stringify({to:room,body:'Живое сообщение во время загрузки'})}),fixture.room);
 await page.waitForFunction(id=>[...document.querySelectorAll('#chat-messages .chat-bubble')].some(n=>n.dataset.messageKey?.includes(id)),live.id);
 releaseLive();await opening;await page.unroute(matcher);
 assert.equal(await bubbles.count(),81);assert((await bubbles.last().innerText()).includes('Живое сообщение'));
 // An in-flight response after logout must not reinsert any private history.
 let releaseLogout,arrivedLogout;responseReady=new Promise(resolve=>arrivedLogout=resolve);const logoutGate=new Promise(resolve=>releaseLogout=resolve);
 await page.route(matcher,async route=>{const response=await route.fetch();arrivedLogout();await logoutGate;await route.fulfill({response})});
 const outgoing=page.evaluate(room=>openChatPeer(room),fixture.room);await responseReady;
 await page.evaluate(()=>{forceLogin('Fixture logout')});releaseLogout();await outgoing;await page.unroute(matcher);
 assert.equal(await bubbles.count(),0);assert.deepEqual(errors,[]);
 console.log('Room archive UI: 257-message pagination, mobile, stale peer response, live/page overlap and logout guard — OK');
}finally{await browser.close()}
