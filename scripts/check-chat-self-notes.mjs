// Isolated browser check: real chat rendering with a fixture history, no user messages sent.
import {chromium} from 'playwright-core';
import {readFile} from 'node:fs/promises';
import assert from 'node:assert/strict';
const app=await readFile('frontend/src/app.js','utf8');
const strings=await readFile('frontend/src/i18n.js','utf8');
const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
try {
 const page=await browser.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.setContent(`<ul id="chat-roster"></ul><div id="chat-empty"></div><section id="chat-thread" class="hidden"><h2 id="chat-peer"></h2><button id="chat-request-presence">Request</button><span id="chat-peer-presence"></span><div id="chat-messages"></div><p id="chat-msg"></p></section>`);
 await page.addScriptTag({content:`const state={lang:'ru',email:'alice@example.com'}; const chatState={me:state.email,peer:'',roster:[{jid:state.email,self:true},{jid:'bob@example.com',name:'Борис',presence:'offline'}],historyRevision:0,messages:[]};const $=id=>document.getElementById(id);const t=k=>I18N[state.lang][k]||k;const escapeHtml=s=>String(s).replaceAll('&','&amp;').replaceAll('<','&lt;');function renderChatSelfPresence(){}function renderOMEMOButton(){}function renderChatGroupButton(){}function omemoGuard(){return ()=>{}}function setMobilePane(){}function setMsg(){}function scrollChatBottom(){}function chatPresenceDot(){return '<span data-presence></span>'}function appendChatBubble(m){const d=document.createElement('p');d.className='chat-bubble';d.textContent=m.body;$('chat-messages').append(d);return d}async function api(path){window.historyPath=path;return {messages:[{id:'saved',from:state.email,to:state.email,body:'Сохранённая личная заметка'}]}}`});
 await page.addScriptTag({content:strings.slice(0,strings.indexOf("\nlet lang"))});
 for(const [start,end] of [['function chatBare(', 'function stopChatLive('],['function renderChatPeerPresence()', '$("btn-chat-new")?.']]){
  await page.addScriptTag({content:app.slice(app.indexOf(start),app.indexOf(end))});
 }
 await page.evaluate(()=>renderChatRoster());
 await page.getByRole('button',{name:/Заметки для себя/}).click();
 assert.equal(await page.textContent('#chat-peer'),'Заметки для себя');
 assert.match(await page.textContent('#chat-messages'),/Сохранённая личная заметка/);
 assert.equal(await page.locator('[data-jid="alice@example.com"]').count(),1);
 assert.equal(await page.locator('[data-jid="alice@example.com"] [data-presence]').count(),0);
 assert.equal(await page.isVisible('#chat-request-presence'),false);
 assert.match(await page.evaluate(()=>window.historyPath),/with=alice%40example.com/);
 await page.evaluate(()=>{state.lang='en';renderChatRoster()});
 assert.equal(await page.getByRole('button',{name:/Notes to self/}).count(),1);
 assert.deepEqual(errors,[]);console.log('Self notes: dedicated row, RU/EN labels, own history and no self-presence request — OK');
} finally {await browser.close()}
