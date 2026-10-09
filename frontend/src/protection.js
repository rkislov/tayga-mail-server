/* Keep technical reports out of the message list. Never render quarantine HTML. */
async function refreshProtectionMessages() {
  const list = $("quarantine-list");
  if (!list) return;
  const params = new URLSearchParams({limit:"200"});
  for (const [key,id] of [["folder","q-folder"],["kind","q-kind"],["q","q-search"]]) {
    const value = $(id)?.value.trim(); if (value) params.set(key,value);
  }
  list.innerHTML = '<tr><td colspan="7">Загрузка…</td></tr>';
  try {
    const data = await api('/api/v1/admin/quarantine?'+params);
    const items = data.items || [];
    list.innerHTML = items.length ? items.map(it => `<tr>
      <td><button type="button" class="protection-subject" data-q-details="${escapeHtml(it.id)}" title="${escapeHtml(it.subject||'Без темы')}">${escapeHtml(it.subject||'Без темы')}</button></td>
      <td title="${escapeHtml(it.from||'')}">${escapeHtml(it.from||'—')}</td>
      <td title="${escapeHtml(it.email)}">${escapeHtml(it.email)}<small>${it.folder==='Junk'?'Спам':'Карантин'}</small></td>
      <td>${escapeHtml(mailDateFull(it.date))}</td><td>${escapeHtml(fmtBytes(it.size))}</td>
      <td>${escapeHtml(it.virus_name || it.spam_score || (it.kind==='virus'?'Вирус':'—'))}</td>
      <td><div class="protection-row-actions"><button type="button" class="btn-secondary" data-q-details="${escapeHtml(it.id)}">Просмотр</button><button type="button" class="btn-secondary" data-q-release="${escapeHtml(it.id)}">Не спам</button><button type="button" class="btn-secondary" data-q-delete="${escapeHtml(it.id)}">Удалить</button></div></td></tr>`).join('') : '<tr><td colspan="7" class="meta">Писем, соответствующих фильтрам, нет.</td></tr>';
    setMsg($('protection-msg'),items.length===200?'Показано 200 писем. Уточните поиск, чтобы найти остальные.':'');
    list.querySelectorAll('[data-q-details]').forEach(button=>button.addEventListener('click',()=>openProtectionMessage(button.dataset.qDetails)));
    list.querySelectorAll('[data-q-release]').forEach(button=>button.addEventListener('click',()=>protectionMessageAction(button.dataset.qRelease,'release')));
    list.querySelectorAll('[data-q-delete]').forEach(button=>button.addEventListener('click',()=>protectionMessageAction(button.dataset.qDelete,'delete')));
  } catch(err) {list.innerHTML='<tr><td colspan="7">Не удалось загрузить письма.</td></tr>';setMsg($('protection-msg'),err.message,'err');}
}
async function protectionMessageAction(id,action,dialog) {
  if(dialog) dialog.close();
  if(action==='delete' && !await askConfirm('Удалить письмо без возможности восстановления?',{danger:true})) {if(dialog) dialog.showModal();return;}
  if(action==='release' && !await askConfirm('Переместить письмо во входящие? Это не обучает Rspamd и не отключает проверку отправителя.')) {if(dialog) dialog.showModal();return;}
  try {
    await api('/api/v1/admin/quarantine/'+encodeURIComponent(id)+(action==='release'?'/release':''),{method:action==='release'?'POST':'DELETE'});
    if(dialog) dialog.remove();
    setMsg($('protection-msg'),action==='release'?'Письмо перемещено во входящие.':'Письмо удалено.','ok');
    await refreshQuarantine();
  } catch(err) {setMsg($('protection-msg'),err.message,'err');if(dialog)dialog.showModal();}
}
async function openProtectionMessage(id) {
  try {
    const it=await api('/api/v1/admin/quarantine/'+encodeURIComponent(id));
    $('protection-reader')?.remove();
    const dialog=document.createElement('dialog');dialog.id='protection-reader';dialog.className='protection-reader';
    const title=document.createElement('h3');title.id='protection-reader-title';title.className='heading';title.textContent=it.subject||'Без темы';dialog.setAttribute('aria-labelledby',title.id);
    const close=document.createElement('button');close.type='button';close.className='btn-secondary';close.textContent='Закрыть';close.addEventListener('click',()=>dialog.close());
    const head=document.createElement('div');head.className='protection-reader-head';head.append(title,close);dialog.append(head);
    const meta=document.createElement('p');meta.className='meta';meta.textContent=`${it.from||'—'} → ${it.email} · ${mailDateFull(it.date)} · ${fmtBytes(it.size)}`;dialog.append(meta);
    const body=document.createElement('pre');body.className='protection-preview';body.textContent=it.preview||'Текст письма отсутствует.';dialog.append(body);
    const report=document.createElement('details');const summary=document.createElement('summary');summary.textContent='Подробный отчёт проверки';const pre=document.createElement('pre');pre.className='protection-preview';pre.textContent=[['Тип',it.kind],['Оценка',it.spam_score],['Антиспам',it.spam_status],['Антивирус',it.virus_status],['Вирус',it.virus_name],['SPF / DKIM / DMARC',it.auth_results]].filter(([,v])=>v).map(([k,v])=>`${k}: ${v}`).join('\n');report.append(summary,pre);dialog.append(report);
    const actions=document.createElement('div');actions.className='actions';for(const [action,label] of [['release','Не спам — во входящие'],['delete','Удалить']]) {const button=document.createElement('button');button.type='button';button.className='btn-secondary';button.textContent=label;button.addEventListener('click',()=>protectionMessageAction(id,action,dialog));actions.append(button);}dialog.append(actions);
    document.body.append(dialog);dialog.showModal();
  }catch(err){setMsg($('protection-msg'),err.message,'err');}
}
function renderProtectionStats(stats) {
  const host=$('protection-stats');if(!host)return;
  const labels={pass:'Пропущено',tag:'Помечено',quarantine:'В спам',reject:'Отклонено',greylist:'Отложено',error:'Ошибки проверки'};
  renderHBars(host,Object.entries(labels).map(([key,label])=>({label,value:stats?.decisions?.[key]||0,display:fmtNum(stats?.decisions?.[key]||0)})),{color:'#d97706'});
  const bulk=document.createElement('p');bulk.className='meta';bulk.textContent='Проверено рассылок: '+fmtNum(stats?.newsletters||0);host.append(bulk);
}
document.querySelectorAll('[data-protection-config]').forEach(button=>button.addEventListener('click',async()=>{await refreshSettings();await openAdminConfig(button.dataset.protectionConfig);}));
$('monitor-open-antispam')?.addEventListener('click',()=>showApp('antispam'));
