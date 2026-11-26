/* Authenticated TMS conferencing. Meeting links never contain JWTs. */
let vksMeetingAPI = null;
let vksEpoch = 0;
let vksScopes = [];
let vksScopeEpoch = 0;
function vksGuard(epoch, account) { return () => { if (epoch !== vksEpoch || state.me?.id !== account) throw new Error('Учётная запись изменилась'); }; }
async function refreshVKS() {
 const epoch=++vksEpoch, account=state.me?.id, guard=vksGuard(epoch,account);
 try {
  const out=await api('/api/v1/vks/meetings',{_guard:guard});
  $('vks-meetings').replaceChildren();
  for(const meeting of out.meetings||[]) {
   const item=document.createElement('div');item.className='flex items-center gap-3 py-2';
   const title=document.createElement('span');title.className='flex-1';title.textContent=meeting.title;
   const join=document.createElement('button');join.className='btn-spray';join.textContent='Войти';join.onclick=()=>joinVKS(meeting.room);
   const share=document.createElement('button');share.className='btn-secondary';share.textContent='Ссылка';share.onclick=async()=>{const link=new URL('/',location.origin);link.searchParams.set('app','vks');link.searchParams.set('room',meeting.room);await navigator.clipboard.writeText(link.href);setMsg($('vks-msg'),'Ссылка скопирована. Доступ получат приглашённые участники.','ok');};
   item.append(title,join,share);$('vks-meetings').append(item);
  }
  setMsg($('vks-msg'),out.meetings?.length?'':'Создайте встречу и пригласите пользователей своего домена.');
 }catch(e){if(epoch===vksEpoch)setMsg($('vks-msg'),e.message,'err');}
 const admin=document.querySelector('#nav-admin');$('vks-admin').classList.toggle('hidden',!!admin?.classList.contains('hidden'));
 if(!admin?.classList.contains('hidden')) {
  try {const out=await api('/api/v1/admin/mattermost/scopes',{_guard:guard});vksScopes=out.scopes||[];$('vks-scope').innerHTML=vksScopes.map((s,i)=>`<option value="${i}">${escapeHtml(s.label)}</option>`).join('');await loadVKSScope();}catch(e){if(epoch===vksEpoch)setMsg($('vks-policy-msg'),e.message,'err');}
 }
 const room=new URL(location.href).searchParams.get('room');if(room&&epoch===vksEpoch) await joinVKS(room);
}
async function loadVKSScope(){
 const item=vksScopes[Number($('vks-scope').value)];if(!item)return;const epoch=++vksScopeEpoch,account=state.me?.id;
 const guard=()=>{if(epoch!==vksScopeEpoch||account!==state.me?.id)throw new Error('Область изменилась');};
 $('vks-policy-form').inert=true;
 try {
  const value=await api('/api/v1/admin/vks'+mmQuery(item.scope),{_guard:guard});
  const own=value.overrides||{},effective=value.effective;
  $('vks-enabled').value=mmBool(own.enabled);
  $('vks-provider').value=own.provider||'inherit';
  $('vks-kontur-inherit').checked=own.kontur_url===undefined;
  $('vks-kontur-url').disabled=own.kontur_url===undefined;
  $('vks-kontur-url').value=own.kontur_url??effective.kontur_url??'';
  for(const kind of ['stun','turn']){
   const urls=own[kind+'_urls'];
   $('vks-'+kind).value=urls?.join('\n')||'';
   $('vks-'+kind+'-inherit').checked=urls===undefined;
  }
  const status=value.available?'доступна':effective.enabled?'требует настройки':'выключена';
  const provider=effective.provider==='kontur_talk'?'Контур.Толк':'Jitsi Meet';
  $('vks-policy-status').textContent=`ВКС ${status} · ${provider}`;
  setMsg($('vks-policy-msg'),'');
 }catch(e){if(epoch===vksScopeEpoch)setMsg($('vks-policy-msg'),e.message,'err');}finally{if(epoch===vksScopeEpoch)$('vks-policy-form').inert=false;}
}
$('vks-scope')?.addEventListener('change',loadVKSScope);
$('vks-kontur-inherit')?.addEventListener('change',event=>{
 $('vks-kontur-url').disabled=event.currentTarget.checked;
});
$('vks-policy-form')?.addEventListener('submit',async event=>{
 event.preventDefault();const item=vksScopes[Number($('vks-scope').value)];if(!item)return;const epoch=vksScopeEpoch,account=state.me?.id,guard=()=>{if(epoch!==vksScopeEpoch||account!==state.me?.id)throw new Error('Область изменилась');};
 const body={};if($('vks-enabled').value!=='inherit')body.enabled=$('vks-enabled').value==='on';
 const provider=$('vks-provider').value;
 if(provider!=='inherit')body.provider=provider;
 if(!$('vks-kontur-inherit').checked)body.kontur_url=$('vks-kontur-url').value.trim();
 for(const kind of ['stun','turn'])if(!$('vks-'+kind+'-inherit').checked)body[kind+'_urls']=$('vks-'+kind).value.split(/\s+/).filter(Boolean);
 try{await api('/api/v1/admin/vks'+mmQuery(item.scope),{method:'PUT',body:JSON.stringify(body),_guard:guard});await loadVKSScope();setMsg($('vks-policy-msg'),'Настройки области сохранены','ok');}catch(e){if(epoch===vksScopeEpoch)setMsg($('vks-policy-msg'),e.message,'err');}
});
$('vks-create')?.addEventListener('submit',async event=>{
 event.preventDefault();const epoch=vksEpoch,guard=vksGuard(epoch,state.me?.id);
 try{const resolved=$('vks-participants').value.split(/[\s,]+/).filter(Boolean);
 const meeting=await api('/api/v1/vks/meetings',{method:'POST',body:JSON.stringify({title:$('vks-title').value,participants:resolved}),_guard:guard});await refreshVKS();await joinVKS(meeting.room);
 }catch(e){if(epoch===vksEpoch)setMsg($('vks-msg'),e.message,'err');}
});
async function joinVKS(room){
 const epoch=vksEpoch,guard=vksGuard(epoch,state.me?.id);
 try{
  const out=await api('/api/v1/vks/join/'+encodeURIComponent(room),{method:'POST',_guard:guard});
  if(out.provider==='kontur_talk'){
   const destination=new URL(out.join_url);
   if(destination.protocol!=='https:')throw new Error('Для ВКС требуется HTTPS');
   guard();vksMeetingAPI?.dispose();vksMeetingAPI=null;
   const open=document.createElement('a');
   open.className='btn-spray';open.textContent='Открыть встречу в Контур.Толке';
   open.href=destination.href;open.target='_blank';open.rel='noopener noreferrer';
   $('vks-conference-title').textContent='Контур.Толк';
   $('vks-conference').replaceChildren(open);
   $('vks-leave').classList.remove('hidden');open.focus();
   return;
  }
  const host=new URL(out.public_url);if(host.protocol!=='https:')throw new Error('Для ВКС требуется HTTPS');
  if(!window.JitsiMeetExternalAPI){await new Promise((resolve,reject)=>{const script=document.createElement('script');script.src=host.origin+'/external_api.js';script.onload=resolve;script.onerror=()=>reject(new Error('Не удалось загрузить Jitsi'));document.head.append(script);});}
  guard();vksMeetingAPI?.dispose();$('vks-conference').replaceChildren();$('vks-conference-title').textContent=out.branding?.name||'Видеоконференция';
  vksMeetingAPI=new JitsiMeetExternalAPI(host.host,{roomName:out.room,jwt:out.jwt,parentNode:$('vks-conference'),width:'100%',height:640,configOverwrite:{disableDeepLinking:true},interfaceConfigOverwrite:{APP_NAME:out.branding?.name||'TMS',NATIVE_APP_NAME:out.branding?.name||'TMS'}});
  $('vks-leave').classList.remove('hidden');
 }catch(e){if(epoch===vksEpoch)setMsg($('vks-msg'),e.message,'err');}
}
$('vks-leave')?.addEventListener('click',()=>{vksMeetingAPI?.dispose();vksMeetingAPI=null;$('vks-conference').replaceChildren();$('vks-leave').classList.add('hidden');});
$('btn-logout')?.addEventListener('click',()=>{++vksEpoch;++vksScopeEpoch;vksMeetingAPI?.dispose();vksMeetingAPI=null;});
