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
  return ({409:mailImageText('Файл изменён или путь уже занят. Обновите список и повторите действие.','File changed or the destination exists. Refresh and retry.'),423:mailImageText('Документ открыт для редактирования. Закройте редактор и повторите действие.','Document is locked for editing. Close the editor and retry.'),503:mailImageText('Офисный редактор ещё не подключён. Администратор может указать адрес Collabora в настройках сервера → Файлы.','Office is not connected yet. Configure Collabora in server settings → Files.')})[error.status]||error.message;
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
        try{await api('/api/v1/files/library/'+(remove?'trash':'restore'),{method:remove?'DELETE':'POST',body:JSON.stringify({id:item.id,kind})});await load();await refreshFiles()}catch(error){view.status.textContent=libraryMessage(error);actions.querySelectorAll('button').forEach(b=>b.disabled=false)}
      };
      actions.append(fileAction(mailImageText('Восстановить','Restore'),()=>act(false),'reply'));if(kind==='trash')actions.append(fileAction(mailImageText('Удалить навсегда','Delete permanently'),()=>act(true),'delete'));row.append(name,date,size,actions);tbody.append(row);
    }
    if(!filtered.length){const row=tbody.insertRow(),cell=row.insertCell();cell.colSpan=4;cell.textContent=mailImageText('Нет файлов','No files')}
    const label=document.createElement('span');label.textContent=filtered.length?`${page*25+1}–${Math.min((page+1)*25,filtered.length)} / ${filtered.length}`:'0';const prev=fileAction('‹',()=>{page--;render()}),next=fileAction('›',()=>{page++;render()});prev.setAttribute('aria-label',mailImageText('Предыдущая страница','Previous page'));next.setAttribute('aria-label',mailImageText('Следующая страница','Next page'));prev.disabled=page===0;next.disabled=(page+1)*25>=filtered.length;nav.replaceChildren(prev,label,next);
  };search.oninput=()=>{page=0;render()};await load();
}
async function openFileDAVInfo() {
  const v=fileLibraryDialog(mailImageText('Подключение по WebDAV','Connect with WebDAV'));
  try{const info=await api('/api/v1/files/office');const label=document.createElement('label'),url=document.createElement('input'),hint=document.createElement('p');label.textContent=mailImageText('Адрес личного хранилища','Personal storage address');url.className='field-input';url.readOnly=true;url.value=info.webdav_url;url.setAttribute('aria-label',label.textContent);hint.textContent=mailImageText('Логин — полный почтовый адрес. Используйте WebDAV-клиент с HTTPS. Passkey применяется только для входа в веб-интерфейс. При обязательном TOTP нужен клиент с поддержкой Bearer-токена.','Use your full email address and an HTTPS WebDAV client. Passkeys are for web sign-in only. Enforced TOTP requires a client that supports Bearer tokens.');v.body.append(label,url,fileAction(mailImageText('Скопировать адрес','Copy address'),async()=>{try{await navigator.clipboard.writeText(url.value);v.status.textContent=mailImageText('Адрес скопирован','Address copied')}catch{url.select()}}),hint)}catch(error){v.status.textContent=libraryMessage(error)}
}
function mountFileReaderActions(entry) {
  let versions=$('btn-files-versions'),office=$('btn-files-office');
  if(!versions){versions=fileAction(mailImageText('История версий','Version history'),()=>{});versions.id='btn-files-versions';$('btn-files-download').after(versions)}
  if(!office){office=fileAction(mailImageText('Открыть в офисе','Open in office'),()=>{});office.id='btn-files-office';$('btn-files-download').after(office)}
  versions.hidden=entry.is_dir||!!filesState.ownerID;versions.onclick=()=>openFileHistory('versions',joinPath(filesState.path,entry.name));
  office.hidden=entry.is_dir||!(/\.(odt|ods|odp|odg|docx?|xlsx?|pptx?|rtf|csv|txt)$/i.test(entry.name));office.onclick=()=>openOfficeDocument(joinPath(filesState.path,entry.name),entry.name);
}
async function openOfficeDocument(path,name) {
  const owner=filesState.ownerID,v=fileLibraryDialog(name,true);v.status.textContent=mailImageText('Подключение к редактору…','Connecting to editor…');
  try{
    const session=await api('/api/v1/files/office?lang='+encodeURIComponent(document.documentElement.lang||'ru'),{method:'POST',body:JSON.stringify({path,owner_id:owner})});if(!v.dialog.open)return;
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
  const menu=$('files-settings-menu');menu?.append(fileAction(mailImageText('Корзина','Trash'),()=>openFileHistory('trash')),fileAction(mailImageText('Подключить WebDAV','Connect WebDAV'),openFileDAVInfo));
}
mountFileLibrary();
