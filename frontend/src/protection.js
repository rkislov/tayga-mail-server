/* Keep technical reports out of the message list. Never render quarantine HTML. */
async function refreshProtectionMessages() {
  refreshProtectionDiagnostics();
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
  if(action==='release' && !await askConfirm('Переместить письмо во входящие? Для письма из папки спама будет запланировано обучение Rspamd как обычной почты, если оно включено.')) {if(dialog) dialog.showModal();return;}
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
  renderTimeChart(host,'protection',Object.entries(labels).map(([key,label])=>({label,value:stats?.decisions?.[key]||0})),'Решения с запуска сервера');
  const bulk=document.createElement('p');bulk.className='meta';bulk.textContent='Проверено рассылок: '+fmtNum(stats?.newsletters||0);host.append(bulk);
}
document.querySelectorAll('[data-protection-config]').forEach(button=>button.addEventListener('click',async()=>{await refreshSettings();await openAdminConfig(button.dataset.protectionConfig);}));
$('monitor-open-antispam')?.addEventListener('click',()=>showApp('antispam'));

async function refreshProtectionDiagnostics(){
 let host=$('protection-diagnostics');
 if(!host){host=document.createElement('section');host.id='protection-diagnostics';host.className='protection-diagnostics';document.querySelector('#app-antispam .protection-settings')?.after(host);
 const head=document.createElement('div');head.className='actions';const title=document.createElement('h3');title.className='heading';title.textContent=mailImageText('Обучение и антивирус','Learning and antivirus');const select=document.createElement('select');select.className='field-input';select.id='protection-period';select.setAttribute('aria-label',mailImageText('Период статистики','Statistics period'));for(const [value,text]of [['hour',mailImageText('Текущий час','Current hour')],['day',mailImageText('Последние сутки','Last 24 hours')],['30d',mailImageText('Последние 30 дней','Last 30 days')]]){const o=document.createElement('option');o.value=value;o.textContent=text;select.append(o)}select.value='day';select.onchange=refreshProtectionDiagnostics;head.append(title,select);const content=document.createElement('div');content.id='protection-diagnostic-content';host.append(head,content)}
 const content=$('protection-diagnostic-content');content.setAttribute('aria-live','polite');content.textContent=mailImageText('Загрузка…','Loading…');
 try{const d=await api('/api/v1/admin/protection/stats?period='+$('protection-period').value);content.replaceChildren();
 const note=document.createElement('p');note.className='meta';note.textContent=mailImageText('Счётчики сохраняются после перезапуска; данные собираются с установки 0.9.18, по часовым интервалам. Антивирус — проверки по получателям.','Counters survive restarts; data starts with 0.9.18 and uses hourly buckets. Antivirus counts per-recipient checks.');content.append(note);
 const grid=document.createElement('div');grid.className='protection-stat-grid';content.append(grid);
 const card=(name,rows)=>{const box=document.createElement('div');box.className='protection-stat-card';const h=document.createElement('h4');h.textContent=name;const dl=document.createElement('dl');for(const [label,value]of rows){const dt=document.createElement('dt'),dd=document.createElement('dd');dt.textContent=label;dd.textContent=String(value??0);dl.append(dt,dd)}box.append(h,dl);grid.append(box);return box};
 const learning=d.counts?.learning||{},av=d.counts?.antivirus||{};
 card('Rspamd · '+mailImageText('обучение','learning'),[[mailImageText('Автообучение','Automatic training'),d.learning_enabled?mailImageText('Включено','Enabled'):mailImageText('Выключено','Disabled')],['Spam',learning.spam],['Ham',learning.ham],[mailImageText('Уже обучено','Already learned'),learning.duplicate],[mailImageText('Пропущено','Skipped'),learning.skipped],[mailImageText('Ошибки попыток','Failed attempts'),learning.error],[mailImageText('В очереди сейчас','Pending now'),d.queue?.pending],[mailImageText('Исчерпаны попытки','Retries exhausted'),d.queue?.failed]]);
 card(mailImageText('Антивирус','Antivirus'),[[mailImageText('Состояние','Status'),d.antivirus_enabled?d.antivirus_backend:mailImageText('Выключен','Disabled')],[mailImageText('Чистые','Clean'),av.clean],[mailImageText('Отклонено','Rejected'),av.reject],[mailImageText('Карантин','Quarantined'),av.quarantine],[mailImageText('Помечено','Tagged'),av.tag],[mailImageText('Ошибки','Errors'),av.error]]);
 const versions=[];if(d.antivirus_version){versions.push(['ClamAV',d.antivirus_version.engine],[mailImageText('Версия базы','Database version'),d.antivirus_version.database_version],[mailImageText('Дата базы','Database date'),d.antivirus_version.database_date])}else if(d.antivirus_enabled)versions.push([mailImageText('Версия базы','Database version'),mailImageText('Нет данных от сканера','Scanner did not provide data')]);
 if(d.rspamd){versions.push(['Rspamd',d.rspamd.version||'—'],[mailImageText('Обучено в Rspamd всего','Total learned in Rspamd'),d.rspamd.learned??'—']);for(const f of d.rspamd.statfiles||[])versions.push([f.symbol||'Bayes',f.revision??f.used??'—'])}
 if(versions.length)card(mailImageText('Движки и базы','Engines and databases'),versions);
 for(const e of [d.rspamd_error,d.antivirus_error])if(e){const p=document.createElement('p');p.className='msg err';p.textContent=e;content.append(p)}
 }catch(e){content.textContent=e.message}
}
