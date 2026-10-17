// Personal external mailbox collection. Credentials never enter browser storage.
let mailSourcesGeneration = 0;
async function refreshMailSources() {
  const host = $('app-profile'); if (!host || !state.tokens?.access_token) return;
  let card = $('mail-sources-card');
  if (!card) {card=document.createElement('section');card.id='mail-sources-card';card.className='card stack-gap';host.insertBefore(card,$('profile-security')?.previousElementSibling || null);}
  const generation=++mailSourcesGeneration, token=state.tokens.access_token;
  try {
    const data=await api('/api/v1/mail-sources');if(generation!==mailSourcesGeneration||state.tokens?.access_token!==token)return;
    card.replaceChildren();
    const header=document.createElement('div');header.className='mail-sources-header';
    const title=document.createElement('h3');title.className='heading';title.textContent=mailImageText('Почта с внешних аккаунтов','External mail accounts');
    const add=document.createElement('button');add.type='button';add.className='btn-spray';add.textContent=mailImageText('Добавить аккаунт','Add account');add.onclick=()=>editMailSource();
    const refresh=document.createElement('button');refresh.type='button';refresh.className='btn-secondary action-icon';refresh.innerHTML=designIcon('refresh');refresh.setAttribute('aria-label',t('refresh'));refresh.title=t('refresh');refresh.onclick=refreshMailSources;header.append(title,refresh,add);
    const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Автоматически забирает новые письма по расписанию. Оригиналы остаются на внешнем сервере. До 5 аккаунтов.','Collect new mail on a schedule. Originals stay on the external server. Up to 5 accounts.');
    const status=document.createElement('p');status.className='msg';status.setAttribute('role','status');
    const wrap=document.createElement('div');wrap.className='mail-sources-table-wrap';const table=document.createElement('table');table.className='admin-users-table';const head=document.createElement('thead'),tr=document.createElement('tr');
    for(const text of [mailImageText('Аккаунт / сервер','Account / server'),mailImageText('Папка назначения','Destination folder'),mailImageText('Расписание','Schedule'),mailImageText('Результат','Result'),mailImageText('Действия','Actions')]){const th=document.createElement('th');th.textContent=text;tr.append(th);}head.append(tr);table.append(head);const body=document.createElement('tbody');
    for(const source of data.sources||[]){
      const row=document.createElement('tr');const cell=()=>{const td=document.createElement('td');row.append(td);return td;};
      const identity=cell(),name=document.createElement('strong'),server=document.createElement('p');name.textContent=source.name;server.className='meta';server.textContent=`${source.username} · ${source.host}:${source.port} · ${source.protocol.toUpperCase()}${source.security==='tls'?'S':''}`;identity.append(name,server);
      cell().textContent=source.target_folder;
      const schedule=cell();schedule.textContent=source.enabled?mailImageText(`Каждые ${source.interval_minutes} мин.`,`Every ${source.interval_minutes} min.`):mailImageText('Приостановлено','Paused');
      const next=document.createElement('p');next.className='meta';next.textContent=source.enabled?(source.next_run?mailImageText('Следующая: ','Next: ')+new Date(source.next_run*1000).toLocaleString():mailImageText('Ожидает запуска','Waiting to start')):'';schedule.append(next);
      const result=cell();const labels={pending:mailImageText('Ожидает запуска','Waiting'),running:mailImageText('Загрузка…','Collecting…'),ok:mailImageText('Готово','Completed'),error:mailImageText('Ошибка','Error')};result.textContent=labels[source.status]||source.status;
      const details=document.createElement('p');details.className='meta';details.textContent=mailImageText(`Загружено: ${source.copied}`,`Collected: ${source.copied}`)+(source.last_run?' · '+new Date(source.last_run*1000).toLocaleString():'');result.append(details);
      if(source.last_error){const error=document.createElement('p');error.className='msg err';error.textContent=source.last_error;result.append(error);}
      const actions=cell();actions.className='mail-sources-actions';
      const action=(icon,ru,en,handler)=>{const b=document.createElement('button');b.type='button';b.className='btn-secondary action-icon';b.innerHTML=designIcon(icon);b.title=mailImageText(ru,en);b.setAttribute('aria-label',b.title);b.disabled=source.status==='running';b.onclick=handler;actions.append(b);return b;};
      action('edit','Настроить','Edit',()=>editMailSource(source));
      const run=action('refresh','Забрать сейчас','Collect now',async()=>{try{run.disabled=true;await api(`/api/v1/mail-sources/${encodeURIComponent(source.id)}/run`,{method:'POST'});await refreshMailSources();}catch(error){status.textContent=error.message;run.disabled=false;}});run.disabled=run.disabled||!source.enabled;
      action('trash','Удалить настройку','Remove source',async()=>{if(!confirm(mailImageText('Удалить настройку сбора? Уже загруженные письма останутся.','Remove this collection source? Downloaded messages will remain.')))return;try{await api(`/api/v1/mail-sources/${encodeURIComponent(source.id)}`,{method:'DELETE'});await refreshMailSources();}catch(error){status.textContent=error.message;}});
      body.append(row);
    }
    table.append(body);wrap.append(table);card.append(header,hint);if((data.sources||[]).length)card.append(wrap);else{const empty=document.createElement('p');empty.className='meta';empty.textContent=mailImageText('Подключите внешний почтовый ящик с доступом по IMAP/POP3 и паролем приложения.','Connect an external mailbox using IMAP or POP3.');card.append(empty);}card.append(status);
  } catch(error){if(generation===mailSourcesGeneration&&state.tokens?.access_token===token)card.textContent=error.message;}
}
async function editMailSource(source={}) {
  const dialog=document.createElement('dialog');dialog.className='mail-source-dialog';
  const form=document.createElement('form'),header=document.createElement('header'),title=document.createElement('h3'),close=document.createElement('button');title.className='heading';title.id='mail-source-title';title.textContent=mailImageText(source.id?'Настройка сбора почты':'Подключить внешний ящик',source.id?'Edit mail collection':'Connect external mailbox');dialog.setAttribute('aria-labelledby',title.id);close.type='button';close.className='btn-secondary action-icon';close.innerHTML=designIcon('close');close.setAttribute('aria-label',mailImageText('Закрыть','Close'));close.onclick=()=>dialog.close();header.append(title,close);
  const fields=document.createElement('div');fields.className='mail-source-fields';const inputs={};
  const field=(name,ru,en,value,options)=>{const label=document.createElement('label');label.className='field-label';label.textContent=mailImageText(ru,en);const input=document.createElement(options?'select':'input');input.className='field-input';input.name=name;input.id='mail-source-'+name;if(options)for(const [v,ru,en] of options){const o=document.createElement('option');o.value=v;o.textContent=mailImageText(ru,en);input.append(o);}input.value=value??'';label.append(input);fields.append(label);inputs[name]=input;return input;};
  field('name','Название','Name',source.name||'');
  const protocol=field('protocol','Протокол','Protocol',source.protocol||'imap',[['imap','IMAP','IMAP'],['pop3','POP3','POP3']]);
  const host=field('host','Сервер','Server',source.host||'');host.placeholder='imap.example.com';host.required=true;
  const security=field('security','Защита соединения','Connection security',source.security||'tls',[['tls','SSL/TLS (IMAPS / POP3S)','SSL/TLS (IMAPS / POP3S)'],['starttls','STARTTLS','STARTTLS'],['plain','Без шифрования','Unencrypted']]);
  const port=field('port','Порт','Port',source.port||993);port.type='number';port.min=1;port.max=65535;port.required=true;
  field('username','Имя пользователя','Username',source.username||'').required=true;
  const password=field('password',source.id?'Новый пароль (оставьте пустым, чтобы сохранить)':'Пароль приложения',source.id?'New password (leave blank to keep)':'Application password','');password.type='password';password.autocomplete='new-password';password.required=!source.id;
  const folder=field('source_folder','Папка на внешнем сервере','Remote folder',source.source_folder||'INBOX');folder.required=true;
  const destination=field('destination','Куда складывать письма','Save messages to','existing',[['existing','В существующую папку','Existing folder'],['new','В новую папку','New folder']]);
  const target=field('target_existing','Папка на Tayga','Tayga folder','INBOX',[['INBOX','Входящие','Inbox']]);
  const newTarget=field('target_new','Название новой папки','New folder name','');newTarget.placeholder=mailImageText('Внешняя почта','External mail');
  const interval=field('interval_minutes','Проверять каждые (минуты)','Check every (minutes)',source.interval_minutes||15);interval.type='number';interval.min=5;interval.max=1440;interval.required=true;
  const enabledLabel=document.createElement('label');enabledLabel.className='field-check';const enabled=document.createElement('input');enabled.type='checkbox';enabled.checked=source.enabled!==false;const enabledText=document.createElement('span');enabledText.textContent=mailImageText('Автоматический сбор включён','Enable scheduled collection');enabledLabel.append(enabled,enabledText);fields.append(enabledLabel);
  const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('При первом запуске загружаются все письма выбранной папки, затем — новые. Оригиналы остаются на источнике. Папка создаётся при первой загрузке. Для Gmail и других сервисов может понадобиться пароль приложения. Проверка TLS-сертификата обязательна.','The first run copies all mail in the selected folder, then only new mail. Originals are retained. The destination is created on the first download. Your provider may require an application password. TLS certificates are verified.');
  const status=document.createElement('p');status.className='msg';status.setAttribute('role','status');
  const actions=document.createElement('footer');const test=document.createElement('button');test.type='button';test.className='btn-secondary';test.textContent=mailImageText('Проверить соединение','Test connection');const save=document.createElement('button');save.type='submit';save.className='btn-spray';save.textContent=t('save');actions.append(test,save);
  const update=()=>{folder.parentElement.hidden=protocol.value==='pop3';newTarget.parentElement.hidden=destination.value!=='new';target.parentElement.hidden=destination.value!=='existing';newTarget.required=destination.value==='new';};
  const defaultPort=()=>{port.value=protocol.value==='imap'?(security.value==='tls'?993:143):(security.value==='tls'?995:110);update();};protocol.onchange=defaultPort;security.onchange=defaultPort;destination.onchange=update;update();
  const values=()=>({name:inputs.name.value,protocol:protocol.value,host:host.value,security:security.value,port:Number(port.value),username:inputs.username.value,password:password.value,source_folder:protocol.value==='imap'?folder.value:'INBOX',target_folder:destination.value==='new'?newTarget.value:target.value,interval_minutes:Number(interval.value),enabled:enabled.checked});
  const submit=async(check)=>{if(!form.reportValidity())return;test.disabled=save.disabled=true;status.textContent=mailImageText(check?'Проверяем соединение…':'Сохраняем…',check?'Testing connection…':'Saving…');try{const suffix=source.id?'/'+encodeURIComponent(source.id):'';await api('/api/v1/mail-sources'+suffix+(check?'/test':''),{method:check||!source.id?'POST':'PUT',body:JSON.stringify(values())});if(check){status.textContent=mailImageText('Соединение и доступ к ящику проверены.','Connection and mailbox access verified.');}else{dialog.close();await refreshMailSources();}}catch(error){status.textContent=error.message;}finally{test.disabled=save.disabled=false;}};
  test.onclick=()=>submit(true);form.onsubmit=e=>{e.preventDefault();submit(false);};form.append(header,fields,hint,status,actions);dialog.append(form);document.body.append(dialog);dialog.addEventListener('close',()=>{password.value='';dialog.remove();},{once:true});dialog.showModal();
  try{const data=await api('/api/v1/mail/mailboxes');for(const box of data.mailboxes||[]){if(box.name==='INBOX'||box.shared)continue;const option=document.createElement('option');option.value=box.name;option.textContent=box.name;target.append(option);}if(source.target_folder){if([...target.options].some(o=>o.value===source.target_folder)){target.value=source.target_folder}else{destination.value='new';newTarget.value=source.target_folder;}}update();}catch(error){status.textContent=error.message;}
}
