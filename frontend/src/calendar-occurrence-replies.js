// Instance RSVP uses the invitation's owner and the original recurrence identity.
let calOccurrenceReplyBusy = false;

function renderCalOccurrenceReply(ev) {
  $('cal-occurrence-reply')?.remove();
  if(!ev?.occurrence_invite_id || !ev.recurrence_id || ev.busy_only || ev.status==='CANCELLED')return;
  const host=document.createElement('section');host.id='cal-occurrence-reply';host.className='cal-invite-item';
  const label=mailImageText('Ответ только на это повторение','Reply to this occurrence only');
  host.setAttribute('aria-label',label);
  host.innerHTML=`<div class="cal-invite-main"><strong>${escapeHtml(label)}</strong><span>${escapeHtml(fmtCalEventWhen(ev,ev.start))}</span><span class="meta">${escapeHtml(mailImageText('Ответ на всю серию и остальные даты не изменятся.','The series response and other dates stay unchanged.'))}</span><span>${escapeHtml(partstatLabel(ev.occurrence_partstat))}</span></div><div class="cal-invite-actions"></div><p class="msg" role="status"></p>`;
  const actions=host.querySelector('.cal-invite-actions');
  const hasProposal=!!ev.occurrence_proposed_start;
  if(hasProposal){
    const pending=document.createElement('span');pending.className='meta';pending.textContent=mailImageText('Предложено: ','Proposed: ')+fmtCalEventWhen({all_day:false},ev.occurrence_proposed_start)+' — '+fmtCalEventWhen({all_day:false},ev.occurrence_proposed_end);host.querySelector('.cal-invite-main').append(pending);
    const note=document.createElement('span');note.className='meta';note.textContent=mailImageText('Новый ответ на приглашение снимет это предложение времени.','Responding to the invitation withdraws this time proposal.');host.querySelector('.cal-invite-main').append(note);
  }
  const userID=state.me?.id;
  for(const [action,status,key] of [['accept','ACCEPTED','cal_accept'],['tentative','TENTATIVE','cal_tentative'],['decline','DECLINED','cal_decline']]){
    const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent=t(key);
    button.disabled=calOccurrenceReplyBusy || !hasProposal && ev.occurrence_partstat===status;
    button.addEventListener('click',()=>{if(state.me?.id===userID)replyCalOccurrence(ev,action,status,host);});actions.append(button);
  }
  const counter=document.createElement('button');counter.type='button';counter.className='btn-secondary btn-sm';counter.textContent=t('cal_counter');counter.disabled=calOccurrenceReplyBusy;
  counter.onclick=()=>{if(state.me?.id===userID)replyCalOccurrence(ev,'counter',null,host);};actions.append(counter);
  $('cal-reader')?.append(host);
}

async function replyCalOccurrence(ev,action,status,host) {
  if(calOccurrenceReplyBusy || calState.currentEvent!==ev)return;
  const userID=state.me?.id;
  const selected=()=>state.me?.id===userID && calState.currentEvent?.id===ev.id;
  const read=()=>api('/api/v1/calendar/events/'+encodeURIComponent(ev.object_id)+'?'+new URLSearchParams({recurrence_id:ev.recurrence_id}));
  calOccurrenceReplyBusy=true;host.querySelectorAll('button').forEach(button=>button.disabled=true);
  const message=host.querySelector('[role=status]');setMsg(message,'…');
  let sent=false,inputError='';
  try{
    const body={action,etag:ev.etag,recurrence_id:ev.recurrence_id};
    if(action==='counter'){
      const start=await askPrompt(t('col_start'),{value:calendarLocalInput(ev.occurrence_proposed_start || ev.start),body:mailImageText('Новое время только этого повторения (YYYY-MM-DDTHH:mm)','New time for this occurrence only (YYYY-MM-DDTHH:mm)')});
      if(start==null || !String(start).trim() || !selected())return;
      const end=await askPrompt(t('col_end'),{value:calendarLocalInput(ev.occurrence_proposed_end || ev.end),body:mailImageText('Конец этого повторения (YYYY-MM-DDTHH:mm)','End of this occurrence (YYYY-MM-DDTHH:mm)')});
      if(end==null || !String(end).trim() || !selected())return;
      body.start=localInputToISO(String(start).trim());body.end=localInputToISO(String(end).trim());
      if(!Number.isFinite(Date.parse(body.start)) || !Number.isFinite(Date.parse(body.end)) || new Date(body.end)<=new Date(body.start))throw Error(mailImageText('Конец должен быть позже начала','End must be after start'));
      if(new Date(body.end)-new Date(body.start)>31*24*60*60*1000)throw Error(mailImageText('Продолжительность повторения не должна превышать 31 день','An occurrence must not exceed 31 days'));
    }
    if(!selected())return;sent=true;
    const result=await api('/api/v1/calendar/invites/'+encodeURIComponent(ev.occurrence_invite_id)+'/reply',{method:'POST',body:JSON.stringify(body)});
    if(!selected())return;
    if(result.id!==ev.occurrence_invite_id || result.recurrence_id!==ev.recurrence_id || (action==='counter'?result.action!=='counter':result.partstat!==status))throw Error(mailImageText('Ответ сервера не подтверждён. Обновите встречу.','The response was not confirmed. Refresh the event.'));
    const updated=await read();if(!selected())return;
    calOccurrenceReplyBusy=false;showCalReader(updated);
    await loadCalEvents();
  }catch(error){
    if(!selected())return;
    if(!sent){inputError=error.message;return;}
    setMsg(message,mailImageText('Не удалось подтвердить ответ. Обновите встречу перед следующей попыткой. ','Could not confirm the response. Refresh the event before trying again. ')+error.message,'err');
    const refresh=document.createElement('button');refresh.type='button';refresh.className='btn-secondary btn-sm';refresh.textContent=mailImageText('Обновить встречу','Refresh event');
    refresh.onclick=async()=>{
      if(!selected())return;refresh.disabled=true;
      try{const updated=await read();if(selected())showCalReader(updated);}catch(error){if(selected()){setMsg(message,error.message,'err');refresh.disabled=false;}}
    };
    host.append(refresh);
  }finally{
    calOccurrenceReplyBusy=false;
    if(!selected() || !sent){renderCalOccurrenceReply(calState.currentEvent);if(selected()&&inputError)setMsg($('cal-occurrence-reply')?.querySelector('[role=status]'),inputError,'err');}
  }
}
