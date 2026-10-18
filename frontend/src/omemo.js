// SPDX-License-Identifier: GPL-3.0-only
// OMEMO legacy (XEP-0384 0.3): ratchet operations are supplied by libomemo.js.
const omemoNS='eu.siacs.conversations.axolotl';
let omemoLibrary,omemoDB;
const omemoB64=b=>{let s='';for(const v of new Uint8Array(b))s+=String.fromCharCode(v);return btoa(s)};
const omemoBytes=s=>Uint8Array.from(atob(s.trim()),c=>c.charCodeAt(0)).buffer;
const omemoHex=b=>Array.from(new Uint8Array(b),v=>v.toString(16).padStart(2,'0')).join('');
const omemoFingerprint=b=>{const bytes=new Uint8Array(b);return omemoHex(bytes.length===33&&bytes[0]===5?bytes.slice(1):bytes).match(/.{1,8}/g).join(' ')};
const omemoXML=s=>{const d=new DOMParser().parseFromString(s,'application/xml');if(d.querySelector('parsererror'))throw Error('Invalid OMEMO XML');return d};
const omemoElement=(d,n)=>d.getElementsByTagNameNS(omemoNS,n)[0];
const omemoElements=(d,n)=>Array.from(d.getElementsByTagNameNS(omemoNS,n));
function omemoAccount(){return chatBare(state.email);}
async function omemoDatabase(){
 if(!omemoDB)omemoDB=new Promise((resolve,reject)=>{const r=indexedDB.open('tayga-omemo-v1',1);r.onupgradeneeded=()=>{r.result.createObjectStore('accounts');r.result.createObjectStore('messages')};r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error)});return omemoDB;
}
async function omemoRead(store,key){const db=await omemoDatabase();return new Promise((resolve,reject)=>{const tx=db.transaction(store),r=tx.objectStore(store).get(key);r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error)})}
async function omemoWrite(account,data,message){const db=await omemoDatabase();return new Promise((resolve,reject)=>{const tx=db.transaction(['accounts','messages'],'readwrite');tx.objectStore('accounts').put(data,account);if(message)tx.objectStore('messages').put(message.body,[account,message.key]);tx.oncomplete=resolve;tx.onerror=()=>reject(tx.error);tx.onabort=()=>reject(tx.error||Error('OMEMO storage failed'))})}
async function omemoLocked(fn){
 if(!isSecureContext||!navigator.locks)throw Error('OMEMO требует HTTPS и поддержку Web Locks / OMEMO requires HTTPS and Web Locks');
 const account=omemoAccount();if(!account)throw Error('Sign in first');
 return navigator.locks.request('tayga-omemo:'+account,async()=>{const lib=await (omemoLibrary||=import('/assets/libomemo.js'));const data=await omemoRead('accounts',account)||{store:{},enabled:{},trust:{}};const store=new lib.InMemoryStore();store.store=data.store;
 store.isTrustedIdentity=(address,key)=>data.trust[address]===omemoB64(key);
 const result=await fn({account,lib,data,store,save:m=>omemoWrite(account,data,m)});return result});
}
async function omemoPEP(owner,node){const r=await api('/api/v1/chat/omemo?owner='+encodeURIComponent(owner)+'&node='+encodeURIComponent(node));return r.items?.find(i=>i.id==='current')?.xml||r.items?.[0]?.xml||''}
async function omemoPublish(node,xml){await api('/api/v1/chat/omemo',{method:'PUT',body:JSON.stringify({node,xml})})}
async function omemoDevices(jid){const raw=await omemoPEP(jid,omemoNS+'.devicelist');if(!raw)return [];const ids=omemoElements(omemoXML(raw),'device').map(e=>Number(e.getAttribute('id')));if(ids.length>100||ids.some(i=>!Number.isInteger(i)||i<1||i>2147483647))throw Error('Invalid device list');return [...new Set(ids)]}
async function omemoBundle(jid,id){const raw=await omemoPEP(jid,omemoNS+'.bundles:'+id);if(!raw)throw Error('Нет ключей устройства / Missing device bundle: '+id);const doc=omemoXML(raw);const key=omemoElement(doc,'identityKey'),signed=omemoElement(doc,'signedPreKeyPublic'),sig=omemoElement(doc,'signedPreKeySignature'),prekeys=omemoElements(doc,'preKeyPublic');if(!key||!signed||!sig||!prekeys.length)throw Error('Incomplete OMEMO bundle');const pre=prekeys[crypto.getRandomValues(new Uint32Array(1))[0]%prekeys.length];return {registrationId:id,identityKey:omemoBytes(key.textContent),signedPreKey:{keyId:Number(signed.getAttribute('signedPreKeyId')),publicKey:omemoBytes(signed.textContent),signature:omemoBytes(sig.textContent)},preKey:{keyId:Number(pre.getAttribute('preKeyId')),publicKey:omemoBytes(pre.textContent)}}}
async function omemoInitialize(ctx){
 const {lib,data,store,save}=ctx;
 if(!data.id){data.id=crypto.getRandomValues(new Uint32Array(1))[0]%2147483646+1;store.put('identityKey',await lib.KeyHelper.generateIdentityKeyPair());store.put('registrationId',data.id);data.prekeys=[];data.nextPrekey=1;const signed=await lib.KeyHelper.generateSignedPreKey(store.getIdentityKeyPair(),1,omemoNS);store.storeSignedPreKey(1,signed.keyPair);data.signature=signed.signature;await save()}
 data.prekeys=data.prekeys.filter(id=>!!store.get('25519KeypreKey'+id));
 while(data.prekeys.length<100){const pre=await lib.KeyHelper.generatePreKey(data.nextPrekey++);store.storePreKey(pre.keyId,pre.keyPair);data.prekeys.push(pre.keyId)}
 await save(); // Persist private keys before advertising public material.
 const signed=store.loadSignedPreKey(1).keyPair;
 await omemoPublish(omemoNS+'.bundles:'+data.id,`<bundle xmlns="${omemoNS}"><signedPreKeyPublic signedPreKeyId="1">${omemoB64(signed.pubKey)}</signedPreKeyPublic><signedPreKeySignature>${omemoB64(data.signature)}</signedPreKeySignature><identityKey>${omemoB64(store.getIdentityKeyPair().pubKey)}</identityKey><prekeys>${data.prekeys.map(id=>`<preKeyPublic preKeyId="${id}">${omemoB64(store.get('25519KeypreKey'+id).pubKey)}</preKeyPublic>`).join('')}</prekeys></bundle>`);
 const devices=await omemoDevices(ctx.account);if(!devices.includes(data.id)){if(devices.length>=100)throw Error('Too many own devices');devices.push(data.id);await omemoPublish(omemoNS+'.devicelist',`<list xmlns="${omemoNS}">${devices.map(id=>`<device id="${id}"/>`).join('')}</list>`)}
}
async function omemoEnabled(peer){const data=await omemoRead('accounts',omemoAccount());return !!data?.enabled?.[peer]}
async function omemoSend(peer,body){
 return omemoLocked(async ctx=>{
 const {lib,store,data,save,account}=ctx;if(new TextEncoder().encode(body).length>120000)throw Error('OMEMO: сообщение слишком большое');if(!data.enabled[peer])throw Error('OMEMO is not enabled');await omemoInitialize(ctx);
 const recipients=await omemoDevices(peer);if(!recipients.length)throw Error('У собеседника нет OMEMO-устройств / No recipient devices');const own=await omemoDevices(account);const targets=[...recipients.filter(id=>peer!==account||id!==data.id).map(id=>[peer,id]),...own.filter(id=>id!==data.id&&peer!==account).map(id=>[account,id])];
 const prepared=[];for(const [jid,id] of targets){const bundle=await omemoBundle(jid,id),address=new lib.OMEMOAddress(jid,id);if(!await store.isTrustedIdentity(address.toString(),bundle.identityKey))throw Error('Подтвердите ключ устройства в OMEMO / Verify device key: '+jid+' #'+id);prepared.push({bundle,address})}
 const key=crypto.getRandomValues(new Uint8Array(16)),iv=crypto.getRandomValues(new Uint8Array(12));const aes=await crypto.subtle.importKey('raw',key,'AES-GCM',false,['encrypt']);const encrypted=new Uint8Array(await crypto.subtle.encrypt({name:'AES-GCM',iv},aes,new TextEncoder().encode(body)));const material=new Uint8Array(32);material.set(key);material.set(encrypted.slice(-16),16);
 let keys='';for(const {bundle,address} of prepared){const cipher=new lib.SessionCipher(store,address,omemoNS);if(!await cipher.hasOpenSession())await new lib.SessionBuilder(store,address,omemoNS).processPreKey(bundle);const wrapped=await cipher.encrypt(material.buffer);const bytes=typeof wrapped.body==='string'?Uint8Array.from(wrapped.body,c=>c.charCodeAt(0)):wrapped.body;keys+=`<key rid="${address.getDeviceId()}"${wrapped.type===3?' prekey="true"':''}>${omemoB64(bytes)}</key>`}
 const xml=`<encrypted xmlns="${omemoNS}"><header sid="${data.id}">${keys}<iv>${omemoB64(iv)}</iv></header><payload>${omemoB64(encrypted.slice(0,-16))}</payload></encrypted>`;
 await save();if(omemoAccount()!==account)throw Error('Account changed');const ev=await api('/api/v1/chat/omemo',{method:'POST',body:JSON.stringify({to:peer,xml})});await save({key:chatBare(ev.from)+'|'+ev.id,body});return {...ev,body,encrypted:false,decrypted:true};
 });
}
async function omemoDecrypt(message){
 return omemoLocked(async ctx=>{
 const {lib,store,data,account,save}=ctx;if(!data.id)throw Error('Включите OMEMO на этом устройстве / Enable OMEMO on this device');const from=chatBare(message.from),cacheKey=from+'|'+message.id;
 const cached=await omemoRead('messages',[account,cacheKey]);if(cached!==undefined)return cached;
 const doc=omemoXML(message.omemo||'');if(doc.documentElement.namespaceURI!==omemoNS)throw Error('This OMEMO version is not supported');
 const header=omemoElement(doc,'header'),sid=Number(header?.getAttribute('sid'));if(!Number.isInteger(sid)||sid<1)throw Error('Invalid sender device');const keys=omemoElements(doc,'key').filter(k=>Number(k.getAttribute('rid'))===data.id);if(!keys.length)throw Error('Сообщение не адресовано этому устройству / Not encrypted for this device');
 const address=new lib.OMEMOAddress(from,sid),bundle=await omemoBundle(from,sid);if(!await store.isTrustedIdentity(address.toString(),bundle.identityKey))throw Error('Подтвердите ключ отправителя в OMEMO / Verify sender key: #'+sid);
 const key=keys[0],cipher=new lib.SessionCipher(store,address,omemoNS),raw=omemoBytes(key.textContent);const result=await (['true','1'].includes(key.getAttribute('prekey'))?cipher.decryptPreKeyWhisperMessage(raw):cipher.decryptWhisperMessage(raw));
 const material=new Uint8Array(result.plaintext),payload=omemoElement(doc,'payload');if(!payload){await save();await omemoInitialize(ctx);return ''}
 if(material.length!==32&&material.length!==16)throw Error('Invalid payload key');const encrypted=new Uint8Array(omemoBytes(payload.textContent)),combined=new Uint8Array(encrypted.length+(material.length===32?16:0));combined.set(encrypted);if(material.length===32)combined.set(material.slice(16),encrypted.length);
 const aes=await crypto.subtle.importKey('raw',material.slice(0,16),'AES-GCM',false,['decrypt']);const plaintext=await crypto.subtle.decrypt({name:'AES-GCM',iv:omemoBytes(omemoElement(doc,'iv').textContent)},aes,combined);const body=new TextDecoder('utf-8',{fatal:true}).decode(plaintext);
 await save({key:cacheKey,body});await omemoInitialize(ctx);return body;
 });
}
async function omemoRender(message,element){
 try{const body=await omemoDecrypt(message);if(!element.isConnected||element.dataset.encryption==='omemo')return;const currentPeer=chatState.peer;const actualPeer=chatBare(message.from)===omemoAccount()?chatBare(message.to):chatBare(message.from);if(currentPeer!==actualPeer)return;const anchor=document.createComment('omemo');element.replaceWith(anchor);const bubble=appendChatBubble({...message,body,encrypted:false,decrypted:true});if(bubble)anchor.replaceWith(bubble);else anchor.remove();scrollChatBottom()}
 catch(error){if(element.isConnected&&element.dataset.encryption!=='omemo')element.querySelector('p').textContent='🔒 '+error.message}
}
const omemoText=(ru,en)=>typeof lang!=='undefined'&&lang==='en'?en:ru;
function omemoNode(tag,className,text){const node=document.createElement(tag);if(className)node.className=className;if(text!==undefined)node.textContent=text;return node}
function omemoDeviceStatus(device,snapshot){
 if(device.current)return 'current';
 if(device.error)return 'unavailable';
 const trusted=snapshot.trust[device.address];
 return trusted===device.fingerprint?'trusted':trusted?'changed':'unverified';
}
async function omemoSettings(){
 const peer=chatState.peer,account=omemoAccount();
 if(!peer||chatState.roster.find(r=>r.jid===peer)?.group||document.querySelector('.omemo-dialog'))return;
 const dialog=omemoNode('dialog','document-preview omemo-dialog');
 dialog.setAttribute('aria-labelledby','omemo-dialog-title');
 const header=omemoNode('header'),title=omemoNode('strong','',omemoText('Шифрование чата','Chat encryption'));
 title.id='omemo-dialog-title';
 const close=omemoNode('button','btn-secondary', '×');close.type='button';close.setAttribute('aria-label',omemoText('Закрыть','Close'));close.onclick=()=>dialog.close();
 header.append(title,close);
 const body=omemoNode('div','omemo-body'),notice=omemoNode('p','omemo-notice');notice.setAttribute('role','status');notice.setAttribute('aria-live','polite');
 const content=omemoNode('div','omemo-content');
 const footer=omemoNode('footer','omemo-footer'),refresh=omemoNode('button','btn-secondary',omemoText('Обновить устройства','Refresh devices'));
 refresh.type='button';
 const hint=omemoNode('p','meta',omemoText('Ключи и названия устройств хранятся в этом браузере. Очистка его данных удалит ключи и доступ к старым сообщениям.','Keys and device names are stored in this browser. Clearing its data removes keys and access to old messages.'));
 footer.append(refresh,hint);body.append(notice,content);dialog.append(header,body,footer);document.body.append(dialog);
 let snapshot,devices=[],loading=false,busy=false,revision=0,changed=false;
 const active=()=>dialog.isConnected&&omemoAccount()===account;
 const guard=c=>{if(c.account!==account||!active())throw Error(omemoText('Учётная запись изменилась. Откройте настройки заново.','Account changed. Reopen these settings.'))};
 dialog.onclose=()=>{revision++;dialog.remove();if(omemoAccount()===account&&chatState.peer===peer){$('chat-omemo')?.focus();if(changed&&typeof openChatPeer==='function')void openChatPeer(peer,true)}};
 dialog.showModal();
 function disableActions(value){for(const b of dialog.querySelectorAll('button'))if(b!==close)b.disabled=value}
 async function change(action){
  if(busy||loading||!active())return;
  const focusedRow=document.activeElement?.closest('.omemo-device');const focusOwner=focusedRow?.dataset.deviceOwner,focusID=focusedRow?.dataset.deviceId;
  busy=true;disableActions(true);notice.textContent='';
  try{
   await omemoLocked(async c=>{guard(c);await action(c);guard(c);await c.save();snapshot.enabled=!!c.data.enabled[peer];snapshot.trust={...c.data.trust};snapshot.names={...c.data.deviceNames}});
   if(active()){changed=true;render();const row=Array.from(content.querySelectorAll('.omemo-device')).find(r=>r.dataset.deviceOwner===focusOwner&&r.dataset.deviceId===focusID);(row?.querySelector('summary')||content.querySelector('[data-omemo-action=toggle]'))?.focus();await renderOMEMOButton()}
  }catch(error){if(active())notice.textContent=error.message}
  finally{busy=false;if(active())disableActions(false)}
 }
 function keyDetails(device,row){
  const details=omemoNode('details','omemo-device-details'),status=omemoDeviceStatus(device,snapshot);
  const summary=omemoNode('summary','',status==='unverified'||status==='changed'?omemoText('Проверить устройство','Verify device'):omemoText('Подробнее','Details'));
  details.append(summary);
  const panel=omemoNode('div','omemo-key-details');
  panel.append(omemoNode('p','meta',device.jid+' · ID '+device.id));
  if(!device.error){
   panel.append(omemoNode('p','',device.current?omemoText('Это отпечаток данного браузера. Сравните его с ключом веб-устройства в Monal или Conversations.','This is this browser’s fingerprint. Compare it with the web device key in Monal or Conversations.'):omemoText('Сверьте этот отпечаток с отпечатком на самом устройстве или по другому доверенному каналу.','Compare this fingerprint with the one on the device, or through another trusted channel.')));
   const code=omemoNode('code','omemo-fingerprint',device.displayFingerprint);code.tabIndex=0;
   const copy=omemoNode('button','btn-secondary btn-sm',omemoText('Скопировать отпечаток','Copy fingerprint'));copy.type='button';
   copy.onclick=async()=>{try{await navigator.clipboard.writeText(device.displayFingerprint);if(active())notice.textContent=omemoText('Отпечаток скопирован.','Fingerprint copied.')}catch{const selection=getSelection(),range=document.createRange();range.selectNodeContents(code);selection.removeAllRanges();selection.addRange(range);notice.textContent=omemoText('Отпечаток выделен — скопируйте его вручную.','Fingerprint selected — copy it manually.')}};
   panel.append(code,copy);
   if(!device.current){
    const verify=omemoNode('button',status==='trusted'?'btn-secondary':'btn-spray',status==='trusted'?omemoText('Отозвать доверие','Revoke trust'):omemoText('Отпечатки совпадают — доверять','Fingerprints match — trust'));
    verify.type='button';verify.dataset.omemoAction=status==='trusted'?'revoke':'verify';
    verify.onclick=()=>change(async c=>{
     if(status==='trusted'){if(c.data.trust[device.address]===device.fingerprint)delete c.data.trust[device.address];return}
     const latest=await omemoBundle(device.jid,device.id);guard(c);
     if(omemoB64(latest.identityKey)!==device.fingerprint)throw Error(omemoText('Ключ изменился во время проверки. Обновите устройства и сравните новый отпечаток.','The key changed during verification. Refresh devices and compare the new fingerprint.'));
     c.data.trust[device.address]=device.fingerprint;c.store.removeSession(device.address);c.store.saveIdentity(device.address,latest.identityKey);
    });
    panel.append(verify);
   }
  }
  const label=omemoNode('label','omemo-device-label',omemoText('Название для вас','Your device label'));
  const input=omemoNode('input','field-input');input.type='text';input.maxLength=60;input.value=snapshot.names[device.address]||'';input.placeholder=device.current?omemoText('Этот браузер','This browser'):omemoText('Например, iPhone или рабочий ноутбук','For example, iPhone or work laptop');
  label.append(input);
  const save=omemoNode('button','btn-secondary btn-sm',omemoText('Сохранить название','Save label'));save.type='button';save.onclick=()=>change(c=>{c.data.deviceNames||={};const value=input.value.trim();if(value)c.data.deviceNames[device.address]=value;else delete c.data.deviceNames[device.address]});
  panel.append(label,save);details.append(panel);row.append(details);
 }
 function render(){
  content.replaceChildren();
  const overview=omemoNode('section','omemo-overview'),top=omemoNode('div','omemo-overview-heading');
  top.append(omemoNode('strong','',snapshot.enabled?omemoText('Шифрование включено','Encryption is on'):omemoText('Шифрование выключено','Encryption is off')));
  const toggle=omemoNode('button','btn-secondary',snapshot.enabled?omemoText('Выключить','Turn off'):omemoText('Включить','Turn on'));toggle.type='button';toggle.dataset.omemoAction='toggle';toggle.setAttribute('aria-label',snapshot.enabled?omemoText('Выключить шифрование чата','Turn off chat encryption'):omemoText('Включить шифрование чата','Turn on chat encryption'));toggle.onclick=()=>change(c=>{c.data.enabled[peer]=!snapshot.enabled});top.append(toggle);overview.append(top);
  overview.append(omemoNode('p','meta',chatState.roster.find(r=>r.jid===peer)?.name||peer));
  const others=devices.filter(d=>!d.current),pending=others.filter(d=>omemoDeviceStatus(d,snapshot)!=='trusted'),missing=others.some(d=>d.error),hasPeer=devices.some(d=>d.jid===peer&&!d.current);
  let text=!hasPeer?omemoText('У собеседника пока нет устройств с OMEMO. Попросите его включить шифрование.','Your contact has no OMEMO devices yet. Ask them to enable encryption.'):missing?omemoText('Некоторые ключи недоступны. Переподключите соответствующее устройство и обновите список.','Some keys are unavailable. Reconnect that device and refresh the list.'):pending.length?omemoText('Нужно проверить устройств: ','Devices to verify: ')+pending.length:snapshot.enabled?omemoText('Все устройства проверены. Можно отправлять зашифрованные сообщения.','All devices are verified. You can send encrypted messages.'):omemoText('Все устройства проверены. Включите шифрование для этого чата.','All devices are verified. Turn on encryption for this chat.');
  if(snapshot.enabled&&pending.length)text+=' '+omemoText('До проверки отправка будет приостановлена.','Sending is paused until verification.');
  overview.append(omemoNode('p','',text));content.append(overview);
  for(const jid of [...new Set([peer,account])]){
   const group=omemoNode('section','omemo-device-group');group.append(omemoNode('h3','',jid===account?omemoText('Мои устройства','My devices'):omemoText('Устройства собеседника','Contact’s devices')));
   const list=omemoNode('ul','omemo-device-list');
   for(const device of devices.filter(d=>d.jid===jid)){
    const status=omemoDeviceStatus(device,snapshot),row=omemoNode('li','omemo-device');row.dataset.deviceId=String(device.id);row.dataset.deviceOwner=jid;row.dataset.keyStatus=status;
    const heading=omemoNode('div','omemo-device-heading'),info=omemoNode('div');
    info.append(omemoNode('strong','',snapshot.names[device.address]||(device.current?omemoText('Этот браузер','This browser'):omemoText('Устройство ','Device ')+device.id)));
    const labels={current:omemoText('Ваш ключ','Your key'),trusted:omemoText('Проверено','Verified'),unverified:omemoText('Нужно проверить','Needs verification'),changed:omemoText('Ключ изменился','Key changed'),unavailable:omemoText('Ключ недоступен','Key unavailable')};
    const badge=omemoNode('span','omemo-device-status',labels[status]);badge.dataset.status=status;heading.append(info,badge);row.append(heading);
    if(status==='changed')row.append(omemoNode('p','omemo-device-warning',omemoText('Прежнее подтверждение больше не действует. Сверьте новый отпечаток.','The previous verification no longer applies. Compare the new fingerprint.')));
    if(device.error)row.append(omemoNode('p','meta',omemoText('Устройство не опубликовало доступный набор ключей.','This device has no accessible key bundle.')));
    keyDetails(device,row);list.append(row);
   }
   if(!list.childElementCount)group.append(omemoNode('p','meta',omemoText('Устройства пока не найдены.','No devices found yet.')));
   else group.append(list);
   content.append(group);
  }
 }
 async function load(){
  if(loading||busy||!active())return;
  loading=true;disableActions(true);const current=++revision;notice.textContent=omemoText('Получаем список устройств…','Loading devices…');
  try{
   snapshot=await omemoLocked(async c=>{guard(c);await omemoInitialize(c);guard(c);return {id:c.data.id,enabled:!!c.data.enabled[peer],trust:{...c.data.trust},names:{...c.data.deviceNames},lib:c.lib,identityKey:c.store.getIdentityKeyPair().pubKey}});
   const next=[];
   for(const jid of [...new Set([peer,account])]){
    const ids=await omemoDevices(jid);if(jid===account&&!ids.includes(snapshot.id))ids.unshift(snapshot.id);
    for(let i=0;i<ids.length;i+=4){
     next.push(...await Promise.all(ids.slice(i,i+4).map(async id=>{
      const device={jid,id,address:new snapshot.lib.OMEMOAddress(jid,id).toString(),current:jid===account&&id===snapshot.id};
      try{const key=device.current?snapshot.identityKey:(await omemoBundle(jid,id)).identityKey;device.fingerprint=omemoB64(key);device.displayFingerprint=omemoFingerprint(key)}catch(error){device.error=error.message}
      return device;
     })));
    }
   }
   if(!active()||current!==revision)return;
   devices=next;notice.textContent='';render();
  }catch(error){if(active()&&current===revision)notice.textContent=error.message}
  finally{loading=false;if(active())disableActions(false)}
 }
 refresh.onclick=load;
 await load();
}
async function renderOMEMOButton(){
 let b=$('chat-omemo');if(!b){b=omemoNode('button','btn-secondary btn-sm');b.id='chat-omemo';b.type='button';b.onclick=omemoSettings;$('chat-peer')?.after(b)}
 const peer=chatState.peer,account=omemoAccount();b.hidden=!peer||!!chatState.roster.find(r=>r.jid===peer)?.group;
 b.setAttribute('aria-haspopup','dialog');
 try{const enabled=await omemoEnabled(peer);if(peer===chatState.peer&&account===omemoAccount()){b.textContent=enabled?omemoText('Шифрование: вкл.','Encryption: on'):omemoText('Шифрование: выкл.','Encryption: off');b.title=omemoText('Устройства и ключи OMEMO','OMEMO devices and keys');b.removeAttribute('aria-pressed')}}catch{b.textContent=omemoText('Шифрование','Encryption')}
}

// XEP-0454: the URL fragment containing IV and key stays inside OMEMO.
const omemoFileLimit=64*1024*1024;
async function omemoEncryptFile(file){
 if(file.size>omemoFileLimit)throw Error('OMEMO: максимум 64 МиБ для шифрования в браузере');
 const key=crypto.getRandomValues(new Uint8Array(32)),iv=crypto.getRandomValues(new Uint8Array(12));
 const aes=await crypto.subtle.importKey('raw',key,'AES-GCM',false,['encrypt']);
 return {key,iv,blob:new Blob([await crypto.subtle.encrypt({name:'AES-GCM',iv},aes,await file.arrayBuffer())],{type:'application/octet-stream'})};
}
function omemoFileURI(url,file){const u=new URL(url);if(u.protocol!=='https:'||u.username||u.password)throw Error('OMEMO file requires HTTPS');return 'aesgcm:'+u.href.slice(6).split('#')[0]+'#'+omemoHex(file.iv)+omemoHex(file.key)}
function omemoFileDescriptor(body){
 const lines=String(body||'').split('\n');if(lines.length>2||(lines.length===2&&!/^data:image\/jpeg;base64,[A-Za-z0-9+/=]+$/.test(lines[1])))throw Error('Invalid encrypted attachment');const line=lines[0];if(!line.startsWith('aesgcm://'))return null;
 const u=new URL('https:'+line.slice(7));if(u.username||u.password||!/^#[a-f0-9]{88}$/i.test(u.hash))throw Error('Invalid encrypted file URL');
 const bytes=Uint8Array.from(u.hash.slice(1).match(/../g),x=>parseInt(x,16)),iv=bytes.slice(0,12),key=bytes.slice(12);u.hash='';
 return {url:u.href,iv,key,name:decodeURIComponent(u.pathname.split('/').pop())||'file'};
}
async function omemoDecryptFile(descriptor,blob){
 if(blob.size>omemoFileLimit+16)throw Error('OMEMO: файл превышает 64 МиБ');
 const aes=await crypto.subtle.importKey('raw',descriptor.key,'AES-GCM',false,['decrypt']);
 return new Blob([await crypto.subtle.decrypt({name:'AES-GCM',iv:descriptor.iv},aes,await blob.arrayBuffer())]);
}
function omemoAttachmentBubble(message,element){
 if(!String(message.body||'').startsWith('aesgcm://'))return;
 const text=element.querySelector('p');text.textContent='🔒 Зашифрованный файл';
 let file;try{file=omemoFileDescriptor(message.body)}catch(e){text.textContent=e.message;return}
 text.textContent='🔒 '+file.name;const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent='Открыть зашифрованный файл';element.append(button);
 button.onclick=async()=>{button.disabled=true;try{const response=await fetch(file.url,{credentials:'omit',referrerPolicy:'no-referrer'});if(!response.ok)throw Error('Файл недоступен');const encrypted=await boundedPreviewBlob(response,omemoFileLimit+16),blob=await omemoDecryptFile(file,encrypted);await openDocumentPreview({name:file.name,blob,size:blob.size})}catch(e){text.textContent='Не удалось расшифровать файл: '+e.message}finally{button.disabled=false}};
}
