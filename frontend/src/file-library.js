// File library UI: all mutations use the same versioned server storage as DAV/WOPI.
function fileLibraryDialog(title, wide=false) {
  const focus=document.activeElement, dialog=document.createElement('dialog');
  dialog.className='document-preview file-library-dialog'+(wide?' file-office-dialog':'');dialog.setAttribute('aria-label',title);
  const head=document.createElement('header'),name=document.createElement('strong'),close=document.createElement('button');
  name.textContent=title;close.type='button';close.className='btn-secondary action-icon';close.title=t('close');close.setAttribute('aria-label',close.title);close.innerHTML=designIcon('close');close.onclick=()=>dialog.close();head.append(name,close);
  const body=document.createElement('div');body.className='file-library-body';const status=document.createElement('p');status.className='meta';status.setAttribute('role','status');
  dialog.append(head,body,status);dialog.onclose=()=>{dialog.remove();if(focus?.isConnected)focus.focus()};document.body.append(dialog);dialog.showModal();return {dialog,head,body,status,close};
}
function fileAction(label,run,icon='') {
  const button=document.createElement('button');button.type='button';button.className=icon?'btn-secondary action-icon':'btn-secondary btn-sm';button.title=label;button.setAttribute('aria-label',label);
  if(icon)button.innerHTML=designIcon(icon);else button.textContent=label;button.onclick=run;return button;
}
function libraryMessage(error) {
  return ({507:mailImageText('Квота файлов исчерпана. Очистите корзину или удалите ненужные версии.','File quota exceeded. Empty trash or remove old versions.'),409:mailImageText('Файл изменён или путь уже занят. Обновите список и повторите действие.','File changed or the destination exists. Refresh and retry.'),423:mailImageText('Документ открыт для редактирования. Закройте редактор и повторите действие.','Document is locked for editing. Close the editor and retry.'),503:mailImageText('Офисный редактор ещё не подключён. Администратор может подключить Collabora, ONLYOFFICE или Р7 в настройках сервера → Файлы.','Office is not connected yet. Configure Collabora, ONLYOFFICE or R7 in server settings → Files.')})[error.status]||error.message;
}
async function openFileHistory(kind,path='') {
  const view=fileLibraryDialog(kind==='trash'?mailImageText('Корзина','Trash'):mailImageText('Версии файла','File versions')+' · '+path);
  const search=document.createElement('input');search.type='search';search.className='field-input';search.placeholder=mailImageText('Поиск по имени или пути','Search name or path');search.setAttribute('aria-label',search.placeholder);
  const note=document.createElement('p');note.className='meta';note.textContent=kind==='trash'?mailImageText('Восстановление возвращает файл на прежнее место. Общий доступ после удаления нужно предоставить заново.','Restoring returns the file to its original location. Share it again after restoring.'):mailImageText('При восстановлении текущий файл также сохраняется в истории.','Restoring also preserves the current file in history.');
  const table=document.createElement('table');table.className='file-library-table';const nav=document.createElement('div');nav.className='file-library-nav';view.body.append(search,note,table,nav);
  let items=[],page=0;
  const load=async()=>{view.status.textContent=mailImageText('Загрузка…','Loading…');try{const data=await api('/api/v1/files/library/'+kind+(path?'?path='+encodeURIComponent(path):''));items=data.entries||[];render();view.status.textContent=''}catch(error){view.status.textContent=libraryMessage(error)}};
  const render=()=>{
    const q=search.value.trim().toLocaleLowerCase(),filtered=items.filter(e=>e.path.toLocaleLowerCase().includes(q));page=Math.min(page,Math.max(0,Math.ceil(filtered.length/25)-1));
    table.innerHTML='<thead><tr><th>'+escapeHtml(mailImageText('Файл','File'))+'</th><th>'+escapeHtml(mailImageText('Дата','Date'))+'</th><th>'+escapeHtml(t('col_size'))+'</th><th>'+escapeHtml(mailImageText('Действия','Actions'))+'</th></tr></thead>';const tbody=document.createElement('tbody');table.append(tbody);
    for(const item of filtered.slice(page*25,(page+1)*25)){
      const row=document.createElement('tr'),name=document.createElement('td'),date=document.createElement('td'),size=document.createElement('td'),actions=document.createElement('td');name.textContent=item.path;date.textContent=new Date(item.at).toLocaleString();size.textContent=item.is_dir?'—':fmtBytes(item.size);actions.className='file-library-actions';
      const act=async remove=>{
        const question=remove?mailImageText('Удалить без возможности восстановления: ','Permanently delete: '):mailImageText('Восстановить: ','Restore: ');
        if(!await askConfirm(question+item.path+'?',{danger:remove}))return;
        actions.querySelectorAll('button').forEach(b=>b.disabled=true);
        try{await api('/api/v1/files/library/'+(remove?kind:'restore'),{method:remove?'DELETE':'POST',body:JSON.stringify({id:item.id,kind})});await load();await refreshFiles()}catch(error){view.status.textContent=libraryMessage(error);actions.querySelectorAll('button').forEach(b=>b.disabled=false)}
      };
      actions.append(fileAction(mailImageText('Восстановить','Restore'),()=>act(false),'reply'));actions.append(fileAction(mailImageText('Удалить навсегда','Delete permanently'),()=>act(true),'delete'));row.append(name,date,size,actions);tbody.append(row);
	  if(kind==='versions'&&!item.is_dir&&/\.(docx|doc|odt|rtf|txt|md|csv|xlsx|ods|pptx|odp|pdf)$/i.test(item.path))actions.prepend(fileAction(mailImageText('Сравнить','Compare'),()=>openFileComparison(item.path,item.id)));
    }
    if(!filtered.length){const row=tbody.insertRow(),cell=row.insertCell();cell.colSpan=4;cell.textContent=mailImageText('Нет файлов','No files')}
    const label=document.createElement('span');label.textContent=filtered.length?`${page*25+1}–${Math.min((page+1)*25,filtered.length)} / ${filtered.length}`:'0';const prev=fileAction('‹',()=>{page--;render()}),next=fileAction('›',()=>{page++;render()});prev.setAttribute('aria-label',mailImageText('Предыдущая страница','Previous page'));next.setAttribute('aria-label',mailImageText('Следующая страница','Next page'));prev.disabled=page===0;next.disabled=(page+1)*25>=filtered.length;nav.replaceChildren(prev,label,next);
  };search.oninput=()=>{page=0;render()};await load();
}
async function openFileDAVInfo() {
  const v=fileLibraryDialog(mailImageText('Подключение по WebDAV','Connect with WebDAV'));
  try {
    const info=await api('/api/v1/files/office');
    for(const [title,address] of [[mailImageText('Адрес личного хранилища','Personal storage address'),info.webdav_url],[mailImageText('Адрес общих папок','Shared folders address'),info.shared_webdav_url]]) {
      if(!address)continue;
      const label=document.createElement('label'),url=document.createElement('input');
      label.textContent=title;url.className='field-input';url.readOnly=true;url.value=address;url.setAttribute('aria-label',title);
      v.body.append(label,url,fileAction(mailImageText('Скопировать адрес','Copy address'),async()=>{try{await navigator.clipboard.writeText(url.value);v.status.textContent=mailImageText('Адрес скопирован','Address copied')}catch{url.select()}}));
    }
    const hint=document.createElement('p');
    hint.textContent=mailImageText('Логин — полный почтовый адрес. Используйте WebDAV-клиент с HTTPS. Общие папки доступны с правами, выданными владельцем. Passkey применяется только для входа в веб-интерфейс. При обязательном TOTP нужен клиент с поддержкой Bearer-токена.','Use your full email address and an HTTPS WebDAV client. Shared folders use the permissions granted by their owner. Passkeys are for web sign-in only. Enforced TOTP requires a client that supports Bearer tokens.');
    v.body.append(hint);
  }catch(error){v.status.textContent=libraryMessage(error)}
}
function mountFileReaderActions(entry) {
  let versions=$('btn-files-versions'),office=$('btn-files-office');
  if(!versions){versions=fileAction(mailImageText('История версий','Version history'),()=>{});versions.id='btn-files-versions';$('btn-files-download').after(versions)}
  if(!office){office=fileAction(mailImageText('Открыть в офисе','Open in office'),()=>{});office.id='btn-files-office';$('btn-files-download').after(office)}
  versions.hidden=entry.is_dir||!!filesState.ownerID;versions.onclick=()=>openFileHistory('versions',joinPath(filesState.path,entry.name));
  office.hidden=entry.is_dir||!(/\.(odt|ods|odp|odg|docx?|xlsx?|pptx?|rtf|csv|txt)$/i.test(entry.name));office.onclick=()=>openOfficeDocument(joinPath(filesState.path,entry.name),entry.name);
	let template=$('btn-file-use-template');
	if(!template){template=fileAction(mailImageText('Использовать как шаблон','Use as template'),()=>{});template.id='btn-file-use-template';office.after(template);}
	template.hidden=entry.is_dir||!(/\.(docx|xlsx|pptx|odt|ods|odp|rtf|txt|csv)$/i.test(entry.name));
	template.onclick=()=>openDocumentTemplate({name:entry.name,path:joinPath(filesState.path,entry.name),owner:filesState.ownerID});
}
async function openOfficeDocument(path,name) {
  const owner=filesState.ownerID,v=fileLibraryDialog(name,true);v.status.textContent=mailImageText('Подключение к редактору…','Connecting to editor…');
  try{
    const session=await api('/api/v1/files/office?lang='+encodeURIComponent(document.documentElement.lang||'ru'),{method:'POST',body:JSON.stringify({path,owner_id:owner})});if(!v.dialog.open)return;
    if(session.provider==='onlyoffice'||session.provider==='r7'){
      const endpoint=new URL(session.url);if(endpoint.origin!==location.origin||endpoint.protocol!=='https:')throw new Error('Invalid office origin');
      const frame=document.createElement('iframe');frame.title=name;frame.allow='clipboard-read; clipboard-write; fullscreen';frame.referrerPolicy='no-referrer';frame.src=endpoint.href;v.body.append(frame);
      v.status.textContent=session.writable?mailImageText('Сохранение — в меню редактора. После закрытия дождитесь сохранения файла на сервере.','Use Save in the editor. After closing, allow time for the file to be saved on the server.'):mailImageText('Только просмотр','Read only');
      const close=async()=>{if(session.writable&&!await askConfirm(mailImageText('Закрыть редактор? Изменения сохранятся после завершения сеанса; это может занять несколько секунд.','Close the editor? Changes are saved after the session ends; this may take a few seconds.')))return;v.dialog.close();refreshFiles()};v.close.onclick=close;v.dialog.addEventListener('cancel',event=>{event.preventDefault();close()});return;
    }
    const endpoint=new URL(session.url);if(endpoint.protocol!=='https:')throw new Error('HTTPS required');
    const frame=document.createElement('iframe');frame.name='tayga-office-'+crypto.randomUUID();frame.title=name;frame.allow='clipboard-read; clipboard-write; fullscreen';frame.referrerPolicy='no-referrer';
    const form=document.createElement('form');form.method='POST';form.action=endpoint.href;form.target=frame.name;form.hidden=true;
    for(const key of ['access_token','access_token_ttl']){const input=document.createElement('input');input.type='hidden';input.name=key;input.value=String(session[key]);form.append(input)}
    v.body.append(frame,form);const send=(MessageId,Values={})=>frame.contentWindow?.postMessage(JSON.stringify({MessageId,SendTime:Date.now(),Values}),endpoint.origin);
    const save=fileAction(t('save'),()=>{send('Action_Save',{DontTerminateEdit:true,Notify:true});v.status.textContent=mailImageText('Сохранение…','Saving…')});save.hidden=!session.writable;v.head.insertBefore(save,v.close);
    const close=async()=>{if(session.writable&&!await askConfirm(mailImageText('Закрыть редактор? Убедитесь, что последние изменения сохранены.','Close the editor? Make sure your latest changes have been saved.')))return;v.dialog.close();refreshFiles()};v.close.onclick=close;v.dialog.addEventListener('cancel',event=>{event.preventDefault();close()});
    const receive=event=>{if(event.source!==frame.contentWindow||event.origin!==endpoint.origin)return;let data;try{data=typeof event.data==='string'?JSON.parse(event.data):event.data}catch{return}
      if(data?.MessageId==='App_LoadingStatus'&&data.Values?.Status==='Document_Loaded')v.status.textContent=session.writable?mailImageText('Изменения сохраняются на сервере Tayga.','Changes are saved on the Tayga server.'):mailImageText('Только просмотр','Read only');
      if(data?.MessageId==='Action_Save_Resp')v.status.textContent=data.Values?.success===false?mailImageText('Не удалось сохранить документ. Не закрывайте редактор.','Save failed. Keep the editor open.'):mailImageText('Сохранено','Saved');
      if(data?.MessageId==='UI_Close')close();
    };window.addEventListener('message',receive);v.dialog.addEventListener('close',()=>window.removeEventListener('message',receive),{once:true});frame.onload=()=>send('Host_PostmessageReady');form.submit();form.remove();
  }catch(error){v.status.textContent=libraryMessage(error)}
}
const fileBrowser={page:0,identity:'',selected:new Set()};
function fileEntriesPage(entries) {
	if($('btn-files-create-document'))$('btn-files-create-document').disabled=!!filesState.ownerID&&filesState.sharedRights!=='write';
  const identity=[state.email,filesState.ownerID,filesState.path,filesState.listTarget].join('|');if(identity!==fileBrowser.identity){fileBrowser.identity=identity;fileBrowser.page=0;fileBrowser.selected.clear();if($('files-search'))$('files-search').value=''}
  const query=($('files-search')?.value||'').trim().toLocaleLowerCase();const sorted=entries.filter(e=>e.name.toLocaleLowerCase().includes(query)).sort((a,b)=>Number(b.is_dir)-Number(a.is_dir)||a.name.localeCompare(b.name));
  const available=new Set(entries.map(e=>e.name));for(const name of fileBrowser.selected)if(!available.has(name))fileBrowser.selected.delete(name);
  fileBrowser.page=Math.min(fileBrowser.page,Math.max(0,Math.ceil(sorted.length/50)-1));
  const nav=$('files-pagination');if(nav){const label=document.createElement('span');label.textContent=sorted.length?`${fileBrowser.page*50+1}–${Math.min((fileBrowser.page+1)*50,sorted.length)} / ${sorted.length}`:'0';const prev=fileAction('‹',()=>{fileBrowser.page--;refreshFiles()}),next=fileAction('›',()=>{fileBrowser.page++;refreshFiles()});prev.setAttribute('aria-label',mailImageText('Предыдущая страница','Previous page'));next.setAttribute('aria-label',mailImageText('Следующая страница','Next page'));prev.disabled=fileBrowser.page===0;next.disabled=(fileBrowser.page+1)*50>=sorted.length;nav.replaceChildren(prev,label,next)}
  if($('btn-files-search-all'))$('btn-files-search-all').hidden=!!filesState.ownerID;
  return sorted.slice(fileBrowser.page*50,(fileBrowser.page+1)*50);
}
function fileRowsRendered() {
  for(const row of $('files-list').querySelectorAll('.file-row')){const name=row.querySelector('[data-name]').dataset.name,check=document.createElement('input');check.type='checkbox';check.checked=fileBrowser.selected.has(name);check.setAttribute('aria-label',mailImageText('Выбрать ','Select ')+name);check.onchange=()=>{if(check.checked)fileBrowser.selected.add(name);else fileBrowser.selected.delete(name);updateFileSelection()};row.prepend(check)}updateFileSelection();
}
function updateFileSelection(){
  const label=$('files-selection-label');if(label)label.textContent=mailImageText('Выбрано: ','Selected: ')+fileBrowser.selected.size;
  const remove=$('btn-files-trash-selected');if(remove)remove.disabled=!fileBrowser.selected.size||(!!filesState.ownerID&&filesState.sharedRights!=='write');
  const all=$('files-select-page'),visible=[...$('files-list').querySelectorAll('.file-row input[type=checkbox]')];if(all){all.checked=visible.length>0&&visible.every(e=>e.checked);all.indeterminate=visible.some(e=>e.checked)&&!all.checked;all.disabled=!visible.length}
}
async function trashSelectedFiles(){
  const names=[...fileBrowser.selected],path=filesState.path,owner=filesState.ownerID;if(!names.length||!await askConfirm(mailImageText('Переместить выбранные файлы в корзину: ','Move selected items to trash: ')+names.length+'?'))return;
  const button=$('btn-files-trash-selected');button.disabled=true;let moved=0;
  try{for(const name of names){if(filesState.ownerID!==owner||filesState.path!==path)throw new Error(mailImageText('Папка изменилась; операция остановлена.','Folder changed; operation stopped.'));await api('/api/v1/files?path='+encodeURIComponent(joinPath(path,name)),{method:'DELETE'});fileBrowser.selected.delete(name);moved++}setMsg($('files-msg'),mailImageText('В корзине: ','Moved to trash: ')+moved)}catch(error){setMsg($('files-msg'),libraryMessage(error),'err')}finally{await refreshFiles();updateFileSelection()}
}
async function openFileSearch(){
  const v=fileLibraryDialog(mailImageText('Поиск во всех моих файлах','Search all my files'));const form=document.createElement('form'),query=document.createElement('input'),submit=fileAction(mailImageText('Найти','Search'),()=>{});submit.type='submit';query.type='search';query.required=true;query.maxLength=256;query.className='field-input';query.placeholder=mailImageText('Имя файла или папки','File or folder name');query.setAttribute('aria-label',query.placeholder);query.value=$('files-search')?.value||'';form.className='file-library-search';form.append(query,submit);const results=document.createElement('div'),nav=document.createElement('div');nav.className='file-library-nav';v.body.append(form,results,nav);let page=1,seq=0;
  const search=async()=>{const id=++seq;if(!query.value.trim())return;v.status.textContent=mailImageText('Поиск…','Searching…');try{const data=await api('/api/v1/files/library/search?q='+encodeURIComponent(query.value)+'&page='+page);if(id!==seq||!v.dialog.open)return;results.replaceChildren();for(const entry of data.entries||[]){const row=fileAction(entry.path,async()=>{filesState.ownerID='';filesState.listTarget='';filesState.sharedRoot='';filesState.path=entry.is_dir?entry.path:entry.path.split('/').slice(0,-1).join('/');filesState.selected=null;v.dialog.close();await refreshFiles();if(!entry.is_dir)selectFileEntry(entry.name)});row.className='file-search-result';results.append(row)}v.status.textContent=data.truncated?mailImageText('Просмотрено 100 000 элементов. Уточните запрос или папку.','Scanned 100,000 entries. Narrow your query or folder.'):mailImageText('Найдено: ','Found: ')+data.total;const prev=fileAction('‹',()=>{page--;search()}),next=fileAction('›',()=>{page++;search()}),label=document.createElement('span');label.textContent=String(page);prev.disabled=page<=1;next.disabled=page*data.page_size>=data.total;nav.replaceChildren(prev,label,next)}catch(error){v.status.textContent=libraryMessage(error)}};form.onsubmit=event=>{event.preventDefault();page=1;search()};if(query.value)await search();else query.focus();
}
function mountFileLibrary(){
  const toolbar=document.createElement('div');toolbar.className='file-browser-tools';const search=document.createElement('input');search.id='files-search';search.type='search';search.className='field-input';search.placeholder=mailImageText('Поиск в папке','Search this folder');search.setAttribute('aria-label',search.placeholder);let timer;search.oninput=()=>{clearTimeout(timer);fileBrowser.page=0;timer=setTimeout(refreshFiles,200)};
  const global=fileAction(mailImageText('Искать везде','Search all'),openFileSearch);global.id='btn-files-search-all';toolbar.append(search,global);$('files-upload-panel').before(toolbar);
  const bulk=document.createElement('div');bulk.className='file-browser-selection';const all=document.createElement('input');all.type='checkbox';all.id='files-select-page';all.setAttribute('aria-label',mailImageText('Выбрать страницу','Select page'));all.onchange=()=>{for(const check of $('files-list').querySelectorAll('.file-row input[type=checkbox]')){check.checked=all.checked;const name=check.parentElement.querySelector('[data-name]').dataset.name;if(all.checked)fileBrowser.selected.add(name);else fileBrowser.selected.delete(name)}updateFileSelection()};const label=document.createElement('span');label.id='files-selection-label';const trash=fileAction(mailImageText('В корзину','Move to trash'),trashSelectedFiles,'delete');trash.id='btn-files-trash-selected';const nav=document.createElement('div');nav.id='files-pagination';nav.className='file-library-nav';bulk.append(all,label,trash,nav);$('files-list').before(bulk);
  const menu=$('files-settings-menu');menu?.append(fileAction(mailImageText('Незавершённые загрузки','Unfinished uploads'),openPendingFileUploads),fileAction(mailImageText('Корзина','Trash'),()=>openFileHistory('trash')),fileAction(mailImageText('Подключить WebDAV','Connect WebDAV'),openFileDAVInfo));
}
mountFileLibrary();

async function openFileStorageUsage(){
 const v=fileLibraryDialog(mailImageText('Место в хранилище','Storage usage'));
 try{const usage=await api('/api/v1/files/library/usage');const total=document.createElement('strong');total.textContent=fmtBytes(usage.used_bytes)+' / '+(usage.quota_bytes>0?fmtBytes(usage.quota_bytes):'∞');v.body.append(total);
 if(usage.quota_bytes>0){const progress=document.createElement('progress');progress.max=usage.quota_bytes;progress.value=usage.used_bytes;progress.setAttribute('aria-label',mailImageText('Использовано места','Storage used'));progress.style.width='100%';v.body.append(progress)}
 for(const [label,value] of [[mailImageText('Файлы','Files'),usage.live_bytes],[mailImageText('Корзина','Trash'),usage.trash_bytes],[mailImageText('Версии','Versions'),usage.version_bytes]]){const p=document.createElement('p');p.textContent=label+': '+fmtBytes(value);v.body.append(p)}
 const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Квота учитывает каждую сохранённую версию и корзину. Для освобождения места удалите ненужные версии или очистите корзину. Незавершённые загрузки ограничены отдельно.','Every retained version and trashed file counts toward your quota. Remove old versions or empty trash to free space. Pending uploads have separate limits.');v.body.append(hint,fileAction(mailImageText('Открыть корзину','Open trash'),()=>{v.dialog.close();openFileHistory('trash')}),fileAction(mailImageText('Все версии','All versions'),()=>{v.dialog.close();openFileHistory('versions')}));
 }catch(error){v.status.textContent=libraryMessage(error)}
}
$('files-settings-menu')?.append(fileAction(mailImageText('Место в хранилище','Storage usage'),openFileStorageUsage));

async function openDocumentTemplate(source=null) {
  const identity=state.email;
  const personalCopy=source&&filesState.ownerID&&filesState.sharedRights!=='write';
  const owner=personalCopy?'':filesState.ownerID;
  const directory=personalCopy?'':filesState.path;
  const v=fileLibraryDialog(mailImageText('Создать из шаблона','Create from template'));
  const form=document.createElement('form'),select=document.createElement('select'),name=document.createElement('input');
  select.className='field-input';select.setAttribute('aria-label',mailImageText('Шаблон','Template'));
  name.className='field-input';name.required=true;name.maxLength=255;name.setAttribute('aria-label',mailImageText('Имя документа','Document name'));
  const location=document.createElement('p');location.className='meta';location.textContent=mailImageText('Папка: ','Folder: ')+(directory||'/');
  const openLabel=document.createElement('label'),open=document.createElement('input');open.type='checkbox';openLabel.append(open,document.createTextNode(' '+mailImageText('Открыть в офисном редакторе','Open in office editor')));
  const submit=fileAction(mailImageText('Создать','Create'),()=>{});submit.type='submit';submit.disabled=true;
  form.append(location,select,name,openLabel,submit);v.body.append(form);
	form.onsubmit=event=>event.preventDefault();
  let templates=[],busy=false,created=false;
  try {
    const data=await api('/api/v1/files/templates');
    if(state.email!==identity||!v.dialog.open)return;
    templates=data.templates||[];openLabel.hidden=!data.office_enabled;open.checked=!!data.office_enabled;
    if(source){select.add(new Option(mailImageText('На основе: ','Based on: ')+source.name,'source'));name.value=source.name.replace(/(\.[^.]+)$/,' '+mailImageText('(копия)','(copy)')+'$1');}
    for(const item of templates)select.add(new Option(document.documentElement.lang==='en'?item.title_en:item.title,item.id));
    const updateName=()=>{if(select.value==='source')name.value=source.name.replace(/(\.[^.]+)$/,' '+mailImageText('(копия)','(copy)')+'$1');else {const item=templates.find(item=>item.id===select.value);if(item)name.value=mailImageText('Новый документ','New document')+item.extension;}};
    select.onchange=updateName;if(!source)updateName();submit.disabled=false;name.focus();name.select();
  }catch(error){v.status.textContent=libraryMessage(error)}
  form.onsubmit=async event=>{
    event.preventDefault();if(busy||created||!name.value.trim())return;
    if(state.email!==identity){v.dialog.close();return;}
    busy=true;submit.disabled=true;v.close.disabled=true;select.disabled=true;name.disabled=true;
    v.status.textContent=mailImageText('Создание…','Creating…');
    const payload={directory,name:name.value.trim(),owner_id:owner||''};
    if(select.value==='source'){payload.source_path=source.path;payload.source_owner_id=source.owner||'';}else payload.template_id=select.value;
    try {
      const result=await api('/api/v1/files/templates',{method:'POST',body:JSON.stringify(payload)});created=true;
      if(state.email!==identity){v.dialog.close();return;}
      v.dialog.close();
      if(filesState.ownerID===owner&&filesState.path===directory){await refreshFiles();selectFileEntry(result.name);if(open.checked&&result.office_enabled)openOfficeDocument(result.path,result.name);}
      else setMsg($('files-msg'),mailImageText('Создан документ: ','Document created: ')+result.path,'ok');
    }catch(error){v.status.textContent=libraryMessage(error);submit.disabled=false;select.disabled=false;name.disabled=false;}
    finally{busy=false;v.close.disabled=false;}
  };
  v.dialog.addEventListener('cancel',event=>{if(busy)event.preventDefault()});
}
const createDocumentButton=fileAction(mailImageText('Создать документ','Create document'),()=>openDocumentTemplate());
createDocumentButton.id='btn-files-create-document';$('btn-files-mkdir')?.after(createDocumentButton);

async function openFileComparison(path,leftID) {
  const identity=state.email,v=fileLibraryDialog(mailImageText('Сравнение версий','Compare versions')+' · '+path);
  const form=document.createElement('form');form.className='file-comparison-form';
  const left=document.createElement('select'),right=document.createElement('select'),mode=document.createElement('select');
  const field=(title,control)=>{const label=document.createElement('label');label.textContent=title;control.className='field-input';control.setAttribute('aria-label',title);label.append(control);return label};
  const submit=fileAction(mailImageText('Сравнить','Compare'),()=>{});submit.type='submit';submit.disabled=true;
  const note=document.createElement('p');note.className='meta';
  const updateNote=()=>{if(mode.value==='motion'){note.textContent=mailImageText('Сравнение кадров и воспроизведение поддерживаемых эффектов. Для неподдерживаемых эффектов слайд остаётся статичным и показывается предупреждение.','Frame comparison and supported effect playback. Unsupported effects leave the slide static with a warning.');return;}if(mode.value==='visual'){note.textContent=mailImageText('Статичные страницы и слайды: рядом, наложение или разница пикселей. Сопоставление по номеру страницы; эффекты не воспроизводятся.','Static pages and slides: side by side, overlay or pixel difference. Pages are paired by number; effects are not played.');return;}if(mode.value==='details'){note.textContent=mailImageText('Сравниваются также сохранённые свойства оформления, диаграмм, переходов и анимации. Это параметры документа: отрисовка и воспроизведение эффектов не выполняются.','Also compares saved appearance, chart, transition and animation properties. Documents are not rendered and effects are not played.');return;}note.textContent=mode.value==='structured'?(/\.(xlsx|ods)$/i.test(path)?mailImageText('Сравниваются листы, значения и формулы ячеек. Формулы не вычисляются; цвета, шрифты, диаграммы и комментарии не учитываются.','Compares sheets, cell values and formulas. Formulas are not calculated; colors, fonts, charts and comments are excluded.'):mailImageText('Сравниваются текст, порядок слайдов и содержимое изображений. Оформление, диаграммы, анимация и заметки не учитываются.','Compares text, slide order and image content. Layout, charts, animation and notes are excluded.')):mode.value==='office'?mailImageText('Сравнение открывается только для просмотра. Исходные файлы не изменяются.','Comparison opens read-only. Original files are unchanged.'):mailImageText('Сравнивается основной текст. Оформление, изображения, колонтитулы и комментарии не учитываются.','Compares body text. Formatting, images, headers, footers and comments are excluded.');};mode.onchange=updateNote;updateNote();
  const output=document.createElement('div');output.className='file-comparison-output';output.setAttribute('aria-label',mailImageText('Изменения','Changes'));
  form.append(field(mailImageText('Исходная версия','Original version'),left),field(mailImageText('Новая версия','Revised version'),right),field(mailImageText('Режим','Mode'),mode),submit);form.onsubmit=event=>event.preventDefault();v.body.append(form,note,output);
  let busy=false;
  try{
    const data=await api('/api/v1/files/library/versions?path='+encodeURIComponent(path));
    if(!v.dialog.open||state.email!==identity)return;
    right.add(new Option(mailImageText('Текущий файл','Current file'),'current'));
    for(const item of data.entries||[]){const title=new Date(item.at).toLocaleString()+' · '+fmtBytes(item.size)+' · '+item.id.slice(0,8);left.add(new Option(title,item.id));right.add(new Option(title,item.id));}
    left.value=leftID;
    if(/\.(docx|odt|txt|md|csv)$/i.test(path))mode.add(new Option(mailImageText('Текстовые изменения','Text changes'),'text'));
	if(/\.(xlsx|ods|pptx|odp)$/i.test(path))mode.add(new Option(/\.(xlsx|ods)$/i.test(path)?mailImageText('Листы и ячейки','Sheets and cells'):mailImageText('Слайды и изображения','Slides and images'),'structured'));
	if(/\.(xlsx|ods|pptx|odp)$/i.test(path))mode.add(new Option(mailImageText('Подробное сравнение','Detailed comparison'),'details'));
    if(data.office_compare_enabled&&/\.(docx|doc|odt|rtf|txt)$/i.test(path))mode.add(new Option('ONLYOFFICE / Р7','office'));
    if(/\.pdf$/i.test(path)||((data.visual_compare_enabled||data.office_compare_enabled)&&/\.(docx|doc|odt|rtf|txt|xlsx|ods|pptx|odp)$/i.test(path)))mode.add(new Option(mailImageText('Визуальное сравнение','Visual comparison'),'visual'));
    if((data.visual_compare_enabled||data.office_compare_enabled)&&/\.(pptx|odp)$/i.test(path))mode.add(new Option(mailImageText('Кадры и анимация','Frames and animation'),'motion'));
	updateNote();
    if(!mode.options.length){v.status.textContent=mailImageText('Для этого формата требуется подключённый ONLYOFFICE/Р7.','This format requires a configured ONLYOFFICE/R7 server.');return;}
    submit.disabled=!left.value;
  }catch(error){v.status.textContent=libraryMessage(error);return;}
  form.onsubmit=async event=>{
    event.preventDefault();if(busy)return;if(state.email!==identity){v.dialog.close();return;}
    if(left.value===right.value){v.status.textContent=mailImageText('Выберите две разные версии.','Choose two different versions.');return;}
    busy=true;submit.disabled=true;left.disabled=true;right.disabled=true;mode.disabled=true;output.replaceChildren();v.status.textContent=mailImageText('Сравнение…','Comparing…');
    try{
      const result=await api('/api/v1/files/library/compare',{method:'POST',body:JSON.stringify({path,left:left.value,right:right.value,mode:mode.value,lang:document.documentElement.lang||'ru'})});
      if(!v.dialog.open||state.email!==identity)return;
      if(result.mode==='motion'){
        await openFileMotionComparison(path,result,identity);v.status.textContent=mailImageText('Сравнение кадров открыто.','Frame comparison opened.');
      }else if(result.mode==='visual'){
        await openRenderedFileComparison(path,result,identity);v.status.textContent=mailImageText('Визуальное сравнение открыто.','Visual comparison opened.');
      }else if(result.mode==='office'){
        const address=new URL(result.url,location.href);if(address.origin!==location.origin)throw new Error(mailImageText('Недопустимый адрес редактора.','Invalid editor address.'));
        const editor=fileLibraryDialog(mailImageText('Сравнение в редакторе','Compare in editor')+' · '+path,true),frame=document.createElement('iframe');frame.title=path;frame.referrerPolicy='no-referrer';frame.src=address.href;editor.body.append(frame);v.status.textContent=mailImageText('Сравнение открыто только для просмотра. Исходные версии не изменяются.','Comparison opened read-only. Original versions are unchanged.');
      }else if(result.mode==='structured'){
		const changes=result.changes||[];let page=0;
		const renderChanges=()=>{
		const fragment=document.createDocumentFragment();
		const sectionName=section=>({workbook:mailImageText('Система дат','Date system'),sheet:mailImageText('Лист','Sheet'),cell:mailImageText('Ячейка','Cell'),slide:mailImageText('Слайд','Slide'),'slide-text':mailImageText('Текст слайда','Slide text'),'slide-images':mailImageText('Изображения слайда','Slide images'),'cell-style':mailImageText('Оформление ячейки','Cell appearance'),'row-style':mailImageText('Оформление строки','Row appearance'),'column-style':mailImageText('Оформление столбца','Column appearance'),'sheet-layout':mailImageText('Разметка листа','Sheet layout'),'sheet-chart':mailImageText('Диаграмма листа','Sheet chart'),'slide-style':mailImageText('Оформление слайда','Slide appearance'),'slide-chart':mailImageText('Диаграмма слайда','Slide chart'),'slide-transition':mailImageText('Переход слайда','Slide transition'),'slide-animation':mailImageText('Анимация слайда','Slide animation'),'presentation-layout':mailImageText('Параметры презентации','Presentation properties'),theme:mailImageText('Темы, макеты и стили','Themes, layouts and styles')})[section]||section;
		for(const change of changes.slice(page*100,(page+1)*100)){
		  const card=document.createElement('section');card.className='file-structured-change';
		  const title=document.createElement('strong');title.textContent=sectionName(change.section)+(change.group?' · '+change.group:'')+(change.address?' · '+change.address:'');
		  const badge=document.createElement('span');badge.className='meta';badge.textContent=change.kind==='insert'?mailImageText('Добавлено','Added'):change.kind==='delete'?mailImageText('Удалено','Removed'):mailImageText('Изменено','Changed');card.append(title,badge);
		  const sides=document.createElement('div');sides.className='file-structured-sides';
		  for(const [label,text,kind] of [[mailImageText('Было','Before'),change.before,'delete'],[mailImageText('Стало','After'),change.after,'insert']]){
			const side=document.createElement('div');side.className='file-structured-side file-comparison-'+kind;const caption=document.createElement('span');caption.className='meta';caption.textContent=label;const value=document.createElement('pre');value.textContent=change.section==='slide-images'&&text?mailImageText('Изображений: ','Images: ')+text.split('\n').length+'\n'+text.split('\n').map(hash=>/^[a-f0-9]{64}$/.test(hash)?'SHA-256: '+hash.slice(0,16)+'…':hash).join('\n'):(text||'—');side.append(caption,value);sides.append(side);
		  }
		  card.append(sides);fragment.append(card);
		}
		if(!result.changes?.length){const message=document.createElement('p');message.textContent=mailImageText('В сравниваемом содержимом изменений нет.','No changes in the compared content.');fragment.append(message);}
		if(changes.length>100){const nav=document.createElement('div');nav.className='file-library-nav';const previous=fileAction('‹',()=>{page--;renderChanges()}),next=fileAction('›',()=>{page++;renderChanges()}),label=document.createElement('span');previous.setAttribute('aria-label',mailImageText('Предыдущая страница','Previous page'));next.setAttribute('aria-label',mailImageText('Следующая страница','Next page'));previous.disabled=page===0;next.disabled=(page+1)*100>=changes.length;label.textContent=`${page*100+1}–${Math.min((page+1)*100,changes.length)} / ${changes.length}`;nav.append(previous,label,next);fragment.append(nav);}
		output.replaceChildren(fragment);
		};renderChanges();v.status.textContent=mailImageText('Добавлено: ','Added: ')+result.added+mailImageText('; удалено: ','; removed: ')+result.removed+mailImageText('; изменено: ','; changed: ')+result.modified;
	  }else{
        const fragment=document.createDocumentFragment();
        for(const line of result.lines||[]){
          const row=document.createElement('div');row.className='file-comparison-line '+({'insert':'file-comparison-insert','delete':'file-comparison-delete'}[line.kind]||'');
          const label=mailImageText('Без изменений','Unchanged');row.setAttribute('aria-label',line.kind==='insert'?mailImageText('Добавлено','Added'):line.kind==='delete'?mailImageText('Удалено','Removed'):label);
          for(const value of [line.left||'',line.right||'']){const number=document.createElement('span');number.className='file-comparison-number';number.textContent=value;row.append(number);}
          const text=document.createElement('span');text.className='file-comparison-text';text.textContent=(line.kind==='insert'?'+ ':line.kind==='delete'?'− ':'  ')+line.text;row.append(text);fragment.append(row);
        }
        output.replaceChildren(fragment);v.status.textContent=mailImageText('Добавлено строк: ','Added lines: ')+result.added+mailImageText('; удалено: ','; removed: ')+result.removed;
      }
    }catch(error){v.status.textContent=libraryMessage(error)}finally{busy=false;submit.disabled=false;left.disabled=false;right.disabled=false;mode.disabled=false;}
  };
}

// Render only the selected page pair. The shared scale preserves changes in paper/slide size.
async function openRenderedFileComparison(path,result,identity) {
  const v=fileLibraryDialog(mailImageText('Визуальное сравнение','Visual comparison')+' · '+path,true);
  v.dialog.classList.add('file-render-dialog');
  const abort=new AbortController(),documents=[],loads=[],tasks=[];
  const identityTimer=setInterval(()=>{if(state.email!==identity)v.dialog.close()},500);
  const alive=()=>v.dialog.open&&state.email===identity;
  v.dialog.addEventListener('close',()=>{clearInterval(identityTimer);abort.abort();for(const task of tasks)task.cancel();for(const load of loads)load.destroy().catch(()=>{});},{once:true});
  const toolbar=document.createElement('div');toolbar.className='file-render-toolbar';
  const page=document.createElement('input');page.type='number';page.min='1';page.value='1';page.className='field-input';page.setAttribute('aria-label',mailImageText('Страница / слайд','Page / slide'));
  const mode=document.createElement('select');mode.className='field-input';mode.setAttribute('aria-label',mailImageText('Вид сравнения','Comparison view'));
  for(const [value,ru,en] of [['sides','Рядом','Side by side'],['overlay','Наложение 50%','Overlay 50%'],['diff','Разница пикселей','Pixel difference']])mode.add(new Option(mailImageText(ru,en),value));
  const label=document.createElement('span'),previous=fileAction('‹',()=>navigate(-1)),next=fileAction('›',()=>navigate(1));previous.setAttribute('aria-label',mailImageText('Предыдущая страница / слайд','Previous page / slide'));next.setAttribute('aria-label',mailImageText('Следующая страница / слайд','Next page / slide'));
  toolbar.append(previous,page,next,label,mode);const content=document.createElement('div');content.className='file-render-content';v.body.append(toolbar,content);
  let total=0,busy=false,canvases=[],pairMissing=false;
  const paint=()=>{
    if(!alive()||canvases.length!==2)return;
    v.status.textContent=mailImageText('Страницы сопоставлены по номеру. Общий масштаб; анимация не воспроизводится.','Pages paired by number. Shared scale; animations are not played.')+(pairMissing?mailImageText(' Страница одной из версий отсутствует.',' A page is absent in one version.'):'');
    content.replaceChildren();content.classList.toggle('file-render-sides',mode.value==='sides');
    if(mode.value==='sides'){
      canvases.forEach((canvas,i)=>{const side=document.createElement('figure'),caption=document.createElement('figcaption');caption.textContent=mailImageText(i?'Стало':'Было',i?'After':'Before')+(Number(page.value)>documents[i].numPages?mailImageText(' · страница отсутствует',' · page absent'):'');side.append(caption,canvas);content.append(side)});return;
    }
    const [a,b]=canvases,width=a.width,height=a.height,canvas=document.createElement('canvas');canvas.width=width;canvas.height=height;canvas.setAttribute('aria-label',mode.selectedOptions[0].textContent);const ctx=canvas.getContext('2d');
    if(mode.value==='overlay'){ctx.drawImage(a,0,0);ctx.globalAlpha=.5;ctx.drawImage(b,0,0);content.append(canvas);return;}
    const left=a.getContext('2d').getImageData(0,0,width,height),right=b.getContext('2d').getImageData(0,0,width,height),out=ctx.createImageData(width,height);let changed=0;
    for(let i=0;i<left.data.length;i+=4){const delta=Math.max(Math.abs(left.data[i]-right.data[i]),Math.abs(left.data[i+1]-right.data[i+1]),Math.abs(left.data[i+2]-right.data[i+2]));const different=delta>24;changed+=different?1:0;out.data[i]=different?220:Math.round((left.data[i]+left.data[i+1]+left.data[i+2])/3*.25+191);out.data[i+1]=different?30:out.data[i];out.data[i+2]=different?90:out.data[i];out.data[i+3]=255;}
    ctx.putImageData(out,0,0);content.append(canvas);v.status.textContent=mailImageText('Отличается пикселей: ','Different pixels: ')+(changed/(width*height)*100).toFixed(2)+'%'+mailImageText(' · порог 24/255; статичная отрисовка, не доля изменений документа.',' · threshold 24/255; static rendering, not the fraction of document changes.')+(pairMissing?mailImageText(' Страница одной из версий отсутствует.',' A page is absent in one version.'):'');
  };
  async function render(){
    if(!alive()||busy||!total)return;
    let index=Math.trunc(Number(page.value));if(!Number.isFinite(index))index=1;index=Math.max(1,Math.min(total,index));page.value=String(index);busy=true;for(const control of [previous,next,page,mode])control.disabled=true;
    content.replaceChildren();v.status.textContent=mailImageText('Отрисовка…','Rendering…');
    try{
      const pages=await Promise.all(documents.map(pdf=>index<=pdf.numPages?pdf.getPage(index):null));if(!alive())return;
      const views=pages.map(p=>p?.getViewport({scale:1})),width=Math.max(...views.filter(Boolean).map(p=>p.width)),height=Math.max(...views.filter(Boolean).map(p=>p.height));
      if(!Number.isFinite(width)||!Number.isFinite(height)||width<=0||height<=0)throw Error(mailImageText('Недопустимый размер страницы.','Invalid page size.'));
      const scale=Math.min(1.5,1800/width,1800/height,Math.sqrt(2000000/(width*height))),w=Math.max(1,Math.ceil(width*scale)),h=Math.max(1,Math.ceil(height*scale));
      const pair=[];pairMissing=pages.some(p=>!p);
      for(let i=0;i<2;i++){
        const canvas=document.createElement('canvas');canvas.width=w;canvas.height=h;const ctx=canvas.getContext('2d');ctx.fillStyle='#fff';ctx.fillRect(0,0,w,h);canvas.setAttribute('aria-label',mailImageText(i?'Стало':'Было',i?'After':'Before'));
        if(pages[i]){const task=pages[i].render({canvasContext:ctx,viewport:pages[i].getViewport({scale}),background:'rgb(255,255,255)'});tasks.push(task);try{await task.promise}finally{tasks.splice(tasks.indexOf(task),1);pages[i].cleanup()}}
        pair.push(canvas);
      }
      if(!alive())return;canvases=pair;label.textContent=`${index} / ${total} (${documents[0].numPages} → ${documents[1].numPages})`;v.status.textContent=mailImageText('Страницы сопоставлены по номеру. Общий масштаб; анимация не воспроизводится.','Pages paired by number. Shared scale; animations are not played.');paint();
    }catch(error){if(alive())v.status.textContent=error.message}finally{busy=false;for(const control of [previous,next,page,mode])control.disabled=false;previous.disabled=index<=1;next.disabled=index>=total;}
  }
  function navigate(delta){if(!busy){page.value=String(Number(page.value)+delta);void render()}}
  page.onchange=()=>void render();mode.onchange=paint;
  try{
    for(const control of [previous,next,page,mode])control.disabled=true;
    v.status.textContent=mailImageText('Подготовка страниц…','Preparing pages…');
    const pdfjs=await import('/assets/pdfjs/pdf.mjs');pdfjs.GlobalWorkerOptions.workerSrc='/assets/pdfjs/pdf.worker.mjs';
    for(const address of [result.left_url,result.right_url]){
      const url=new URL(address,location.href);if(url.origin!==location.origin||url.pathname!=='/office/compare/pdf')throw Error(mailImageText('Недопустимый адрес сравнения.','Invalid comparison address.'));
      const response=await fetch(url,{credentials:'same-origin',signal:abort.signal,cache:'no-store'});if(!response.ok)throw Error(mailImageText('Не удалось подготовить PDF. Откройте сравнение заново. Код: ','Could not prepare PDF. Reopen comparison. Status: ')+response.status);
      // Streaming bound also covers chunked responses without Content-Length.
      const reader=response.body.getReader(),chunks=[];let size=0;
      while(true){const {value,done}=await reader.read();if(done)break;size+=value.length;if(size>32*1024*1024){await reader.cancel();throw Error('PDF > 32 MiB')}chunks.push(value)}
      if(!alive())return;const data=new Uint8Array(size);let offset=0;for(const chunk of chunks){data.set(chunk,offset);offset+=chunk.length}
      const load=pdfjs.getDocument({data,isEvalSupported:false,stopAtErrors:true,maxImageSize:16000000,canvasMaxAreaInBytes:16000000,cMapUrl:'/assets/pdfjs/cmaps/',cMapPacked:true,standardFontDataUrl:'/assets/pdfjs/standard_fonts/',wasmUrl:'/assets/pdfjs/wasm/'});loads.push(load);const pdf=await load.promise;if(!alive())return;if(pdf.numPages>500)throw Error(mailImageText('Не более 500 страниц на версию.','Up to 500 pages per version.'));documents.push(pdf);
    }
    total=Math.max(...documents.map(pdf=>pdf.numPages));page.max=String(total);await render();
  }catch(error){if(alive())v.status.textContent=error.message;for(const load of loads)void load.destroy().catch(()=>{});}
}
