/* A selected delivery entry opens a visible, keyboard-accessible status sheet. */
(() => {
  const words=workspaceWords,esc=escapeHtml,old=$('maillog-lines');if(!old)return;
  const host=document.createElement('div');host.id='maillog-lines';host.className='workspace-mail-log';old.replaceWith(host);
  const dialog=document.createElement('dialog');dialog.className='workspace-delivery-dialog';dialog.setAttribute('aria-labelledby','delivery-detail-title');document.body.append(dialog);
  let generation=0,opener=null,selected='',identity='',watch=null;
  const statuses={
    delivered:['Доставлено в ящик','Delivered to mailbox','success'],sent:['Принято сервером получателя','Accepted by recipient server','success'],
    queued:['В очереди','Queued','pending'],deferred:['Ожидает повтора','Retry scheduled','warning'],failed:['Не доставлено','Delivery failed','error'],rejected:['Отклонено','Rejected','error'],
    smtp_dialogue:['SMTP-сеанс','SMTP session','neutral'],quarantined:['В карантине','Quarantined','warning'],quarantine:['В карантине','Quarantined','warning'],accepted:['Принято','Accepted','neutral']
  };
  function status(entry){const value=statuses[entry.event];return value?{label:words(value[0],value[1]),tone:value[2]}:{label:entry.event||'—',tone:'neutral'}}
  function badge(entry){const value=status(entry);const span=document.createElement('span');span.className='mail-log-event';span.dataset.tone=value.tone;span.textContent=value.label;return span}
  function date(entry){return new Date(entry.created_at).toLocaleString(calLocale())}
  function facts(entries){const dl=document.createElement('dl');dl.className='delivery-facts';for(const [label,value]of entries){if(value==null||value==='')continue;const row=document.createElement('div'),dt=document.createElement('dt'),dd=document.createElement('dd');dt.textContent=label;dd.textContent=String(value);row.append(dt,dd);dl.append(row)}return dl}
  function correlation(entry){const match=/^(transaction_id|queue_id)=([0-9a-f-]{36})(?:\s|$)/.exec(entry.detail||'');return match?{key:match[1],id:match[2]}:null}
  function metadata(entry,key){return new RegExp('(?:^|\\s)'+key+'=([^\\s]+)').exec(entry.detail||'')?.[1]||''}
  function reason(entry){return (entry.detail||'').replace(/^(?:transaction_id|queue_id)=[^\s]+\s*/, '').replace(/^attempt=\d+\s*/, '').trim()}
  function render(entry,events,loading=false,error='',truncated=false){
    dialog.replaceChildren();
    const header=document.createElement('header');header.className='delivery-detail-heading';const title=document.createElement('h2');title.id='delivery-detail-title';title.textContent=words('Статус письма','Message status');const close=document.createElement('button');close.type='button';close.className='btn-secondary action-icon';close.textContent='×';close.setAttribute('aria-label',words('Закрыть подробности','Close details'));close.onclick=()=>dialog.close();header.append(title,close);dialog.append(header);
    const ordered=[...events].sort((a,b)=>new Date(a.created_at)-new Date(b.created_at)||a.id.localeCompare(b.id));
    const latest=[...ordered].reverse().find(item=>item.event!=='smtp_dialogue')||entry;
    const summary=document.createElement('section');summary.className='delivery-summary';summary.dataset.tone=status(latest).tone;
    const caption=document.createElement('p');caption.className='meta';caption.textContent=words('Последний зафиксированный статус','Latest recorded status');summary.append(caption,badge(latest));
    const when=document.createElement('p');when.className='meta';when.textContent=date(latest);summary.append(when);
    if(latest.event==='sent'){const note=document.createElement('p');note.className='meta';note.textContent=words('Удалённый SMTP-сервер подтвердил приём. Доставка в конечную папку и прочтение этим статусом не подтверждаются.','The remote SMTP server confirmed acceptance. This does not confirm the final folder or reading.');summary.append(note)}
    if(['failed','rejected','deferred'].includes(latest.event)&&reason(latest)){const cause=document.createElement('p');cause.className='delivery-cause';cause.textContent=reason(latest);summary.append(cause)}
    dialog.append(summary);
    const link=correlation(entry);const attempt=metadata(latest,'attempt'),duration=metadata(latest,'duration_ms');
    dialog.append(facts([[words('Отправитель','Sender'),entry.mail_from||'<>'],[words('Получатель','Recipient'),entry.rcpt_to],['Message-ID',entry.message_id],
      [words('Направление','Direction'),entry.direction==='inbound'?words('Входящее','Incoming'):entry.direction==='outbound'?words('Исходящее','Outgoing'):entry.direction],
      [words('Сервер / IP','Server / IP'),latest.peer||metadata(latest,'peer')||entry.peer],[words('Размер','Size'),entry.size>0?fmtBytes(entry.size):words('Не записан','Not recorded')],
      [words('Попытка','Attempt'),attempt],[words('Длительность','Duration'),duration?duration+' '+words('мс','ms'):''],[words('Следующая попытка','Next attempt'),metadata(latest,'next_attempt')||metadata(latest,'next')],
      [link?.key==='queue_id'?'Queue ID':'Transaction ID',link?.id]]));
    const section=document.createElement('section');section.className='delivery-history';const heading=document.createElement('h3');heading.textContent=words('История обработки','Processing history');section.append(heading);
    const notice=document.createElement('p');notice.className='meta';notice.setAttribute('role','status');notice.textContent=loading?words('Загружаю связанные события…','Loading related events…'):error?words('Не удалось загрузить историю: ','Could not load history: ')+error:truncated?words('Показаны последние 200 событий.','Showing the latest 200 events.'):!link&&!entry.message_id?words('У записи нет идентификатора для поиска связанных событий.','This entry has no identifier for related event lookup.'):words('Сохранённые события для этого письма и получателя.','Recorded events for this message and recipient.');section.append(notice);
    const timeline=document.createElement('ol');timeline.className='delivery-timeline';
    for(const item of ordered){const li=document.createElement('li');li.dataset.tone=status(item).tone;const row=document.createElement('div');row.className='delivery-timeline-heading';const time=document.createElement('time');time.dateTime=item.created_at;time.textContent=date(item);row.append(badge(item),time);li.append(row);
      if(item.detail||item.line){const technical=document.createElement('details');if(item.event!=='smtp_dialogue')technical.open=true;const label=document.createElement('summary');label.textContent=item.event==='smtp_dialogue'?words('Диалог SMTP','SMTP dialogue'):words('Диагностика сервера','Server diagnostics');const pre=document.createElement('pre');pre.textContent=item.detail||item.line;technical.append(label,pre);li.append(technical)}timeline.append(li)}
    section.append(timeline);dialog.append(section);if(dialog.open)close.focus({preventScroll:true});
  }
  async function open(entry,button){identity=state.email;clearInterval(watch);watch=setInterval(()=>{if(state.email!==identity||!state.tokens?.access_token)dialog.close()},300);const check=()=>{if(state.email!==identity||!state.tokens?.access_token)throw Error('Учётная запись изменилась')};const request=++generation;opener=button||document.activeElement;selected=entry.id;render(entry,[entry],true);if(!dialog.open)dialog.showModal();
    host.querySelectorAll('[data-log-id]').forEach(row=>row.classList.toggle('is-active',row.dataset.logId===entry.id));
    const link=correlation(entry);if(!link&&!entry.message_id){render(entry,[entry]);return}
    try{const params=new URLSearchParams({limit:'201',mail_from:entry.mail_from||'',rcpt_to:entry.rcpt_to||''});if(entry.direction)params.set('direction',entry.direction);if(entry.tenant_id)params.set('tenant_id',entry.tenant_id);if(entry.domain)params.set('domain',entry.domain);
      if(link){params.set('correlation_key',link.key);params.set('correlation_id',link.id)}else params.set('message_id',entry.message_id);
      const data=await api('/api/v1/admin/mail-log?'+params,{_guard:check});if(request!==generation||!dialog.open)return;
      const list=(data.items||[]).slice(0,200);if(!list.some(item=>item.id===entry.id))list.push(entry);
      render(entry,list,false,'',(data.items||[]).length>200);
    }catch(error){if(request===generation&&dialog.open)render(entry,[entry],false,error.message)}
  }
  dialog.addEventListener('close',()=>{generation++;clearInterval(watch);dialog.replaceChildren();if(opener?.isConnected)opener.focus()});
  dialog.addEventListener('click',event=>{const rect=dialog.getBoundingClientRect();if(event.target===dialog&&(event.clientX<rect.left||event.clientX>rect.right||event.clientY<rect.top||event.clientY>rect.bottom))dialog.close()});
  refreshMailLog=async function(reset=true){if(reset)maillogOffset=0;const request=++maillogRequest;
    try{const params=new URLSearchParams({limit:String(maillogPageSize+1),offset:String(maillogOffset)}),query=$('maillog-q').value.trim();if(query)params.set('q',query);
      const data=await api('/api/v1/admin/mail-log?'+params);if(request!==maillogRequest)return;const items=(data.items||[]).slice(0,maillogPageSize);
      host.innerHTML='<div class="workspace-record-table"><table><thead><tr><th>'+words('Время','Time')+'</th><th>'+words('Отправитель','Sender')+'</th><th>'+words('Получатель','Recipient')+'</th><th>'+words('Статус','Status')+'</th><th>Message-ID</th></tr></thead><tbody></tbody></table></div>';
      const tbody=host.querySelector('tbody');
      for(const item of items){const row=document.createElement('tr');row.dataset.logId=item.id;row.classList.toggle('is-active',item.id===selected);
        const when=document.createElement('td'),button=document.createElement('button');button.type='button';button.className='workspace-log-open';button.textContent=new Date(item.created_at).toLocaleTimeString(calLocale(),{hour:'2-digit',minute:'2-digit'});button.setAttribute('aria-haspopup','dialog');button.setAttribute('aria-label',words('Подробный статус письма: ','Detailed message status: ')+(item.mail_from||'<>')+' → '+(item.rcpt_to||'—'));when.append(button);row.append(when);
        for(const value of [item.mail_from||'<>',item.rcpt_to||'—']){const td=document.createElement('td');td.textContent=value;row.append(td)}
        const state=document.createElement('td');state.append(badge(item));const id=document.createElement('td');id.textContent=item.message_id||'—';id.title=item.message_id||'';row.append(state,id);row.addEventListener('click',()=>open(item,button));tbody.append(row)}
      if(!items.length){const row=document.createElement('tr');const td=document.createElement('td');td.colSpan=5;td.className='workspace-table-empty';td.textContent=words('Записей пока нет','No log entries');row.append(td);tbody.append(row)}
      const scope=$('maillog-scope');if(scope)scope.textContent=words('Приём, доставка и исходящая очередь. Нажмите на письмо, чтобы открыть подробный статус.','Incoming mail, delivery and outgoing queue. Select an entry to see its detailed status.');
      $('maillog-prev').disabled=maillogOffset===0;$('maillog-next').disabled=(data.items||[]).length<=maillogPageSize;setMsg($('maillog-msg'),items.length?(maillogOffset+1)+'—'+(maillogOffset+items.length):'0');
    }catch(error){if(request===maillogRequest)setMsg($('maillog-msg'),error.message,'err')}
  };
})();
