// File details and access. Links and activity are returned only to the owner.
async function openFileCard(path,owner='') {
 const view=fileLibraryDialog(mailImageText('Карточка: ','Details: ')+path);
 view.dialog.classList.add('file-card-dialog');
 const identity=state.email,alive=()=>view.dialog.open&&state.email===identity;
 const timer=setInterval(()=>{if(state.email!==identity)view.dialog.close()},500);
 view.dialog.addEventListener('close',()=>clearInterval(timer),{once:true});
 const endpoint='/api/v1/files/card?path='+encodeURIComponent(path)+(owner?'&owner_id='+encodeURIComponent(owner):'');
 let busy=false,sequence=0;
 const text=(node,value)=>{node.textContent=value;return node};
 const section=title=>{const node=document.createElement('section');node.className='file-card-section';node.append(text(document.createElement('h3'),title));view.body.append(node);return node};
 const rightsLabel=value=>value==='upload'?mailImageText('Только загрузка','Upload only'):value==='manage'?mailImageText('Управление','Manage'):value==='owner'?mailImageText('Владелец','Owner'):value==='write'?mailImageText('Чтение и запись','Read and write'):mailImageText('Только чтение','Read only');
 const button=(label,run)=>fileAction(label,()=>act(run));
 const field=(form,label,input)=>{const row=document.createElement('label');row.textContent=label;input.className='field-input';row.append(input);form.append(row);return input};
 const rightsSelect=()=>{const select=document.createElement('select');for(const value of ['read','write'])select.add(new Option(rightsLabel(value),value));return select};
 const notify=message=>{if(alive())view.status.textContent=message};
 async function act(fn){if(busy||!alive())return;busy=true;view.body.inert=true;notify(mailImageText('Сохранение…','Saving…'));try{await fn();if(alive()){await load();notify(mailImageText('Сохранено','Saved'))}}catch(error){notify(libraryMessage(error))}finally{busy=false;view.body.inert=false}}
 async function mutate(url,method,body){if(!alive())return;return api(url,{method,...(body?{body:JSON.stringify(body)}:{})})}
 async function load(){
  const seq=++sequence,data=await api(endpoint);if(!alive()||seq!==sequence)return;
  view.body.replaceChildren();
  const props=section(mailImageText('Сведения','Details'));
  for(const [label,value] of [[mailImageText('Путь','Path'),data.path],[mailImageText('Тип','Type'),data.is_dir?mailImageText('Папка','Folder'):mailImageText('Файл','File')],...(!data.is_dir?[[mailImageText('Размер','Size'),fmtBytes(data.size)]]:[]),[mailImageText('Изменён','Modified'),new Date(data.mtime).toLocaleString()],[mailImageText('Ваши права','Your permissions'),rightsLabel(data.effective_rights)]]){const p=document.createElement('p');p.textContent=label+': '+value;props.append(p)}
  if(data.space){const space=section(mailImageText('Общее пространство','Shared space'));space.append(text(document.createElement('p'),data.space.name+' · '+rightsLabel(data.effective_rights)));for(const grant of data.space.groups||[])space.append(text(document.createElement('p'),grant.name+' · '+rightsLabel(grant.rights)));if(data.can_manage)space.append(fileAction(mailImageText('Настроить пространство','Space settings'),()=>{view.dialog.close();return openFileSpaceSettings(data.space.id)}))}
  const access=section(mailImageText('Доступ пользователей','User access'));
  const accessNote=document.createElement('p');accessNote.className='meta';accessNote.textContent=data.can_manage?mailImageText('Доступ из родительских папок продолжает действовать. При нескольких разрешениях действует наиболее широкое.','Access inherited from parent folders remains effective. The broadest permission applies.'):mailImageText('Показаны только ваши разрешения. Управление доступом и журнал доступны владельцу.','Only your permissions are shown. Access management and activity are available to the owner.');access.append(accessNote);
  for(const grant of data.access||[]){
   const row=document.createElement('div');row.className='file-card-row';row.append(text(document.createElement('span'),grant.email+' · '+rightsLabel(grant.rights)));
   if(grant.inherited){row.append(text(document.createElement('span'),mailImageText('Из папки: ','From folder: ')+grant.path));if(data.can_manage)row.append(button(mailImageText('Открыть папку','Open folder'),async()=>{view.dialog.close();await openFileCard(grant.path)}))}
   else if(data.can_manage){const select=rightsSelect();select.value=grant.rights;select.className='field-input';select.setAttribute('aria-label',mailImageText('Права: ','Permissions: ')+grant.email);row.append(select,button(mailImageText('Изменить','Change'),()=>mutate('/api/v1/files/access','POST',{path,email:grant.email,rights:select.value})),button(mailImageText('Отозвать','Revoke'),async()=>{if(await askConfirm(mailImageText('Отозвать прямой доступ для ','Revoke direct access for ')+grant.email+'?'))await mutate('/api/v1/files/access','DELETE',{path,grantee_id:grant.grantee_id})}))}
   access.append(row);
  }
  if(!data.access?.length)access.append(text(document.createElement('p'),mailImageText('Дополнительных разрешений нет','No additional permissions')));
  if(data.can_manage){
   {const form=document.createElement('form');form.className='file-card-form';const email=field(form,'Email',document.createElement('input'));email.type='email';email.required=true;email.autocomplete='off';const rights=field(form,mailImageText('Права','Permissions'),rightsSelect());const submit=fileAction(mailImageText('Предоставить доступ','Grant access'),()=>{});submit.type='submit';form.append(submit);form.onsubmit=event=>{event.preventDefault();void act(()=>mutate('/api/v1/files/access','POST',{path,email:email.value.trim(),rights:rights.value}))};access.append(form);}
   const links=section(mailImageText('Публичные ссылки','Public links'));
   for(const share of (data.shares||[]).filter(item=>item.rights!=='upload')){
    const row=document.createElement('div');row.className='file-card-row';const details=[rightsLabel(share.rights),share.active?mailImageText('Активна','Active'):mailImageText('Недействительна','Inactive'),share.password_protected?mailImageText('С паролем','Password protected'):mailImageText('Без пароля','No password'),share.expires_at?new Date(share.expires_at).toLocaleString():mailImageText('Бессрочно','No expiry'),mailImageText('Скачиваний: ','Downloads: ')+share.download_count+(share.max_downloads?' / '+share.max_downloads:'')];row.append(text(document.createElement('p'),details.join(' · ')));
    if(share.inherited){row.append(text(document.createElement('p'),mailImageText('Из папки: ','From folder: ')+share.path));row.append(button(mailImageText('Управлять в папке','Manage in folder'),async()=>{view.dialog.close();await openFileCard(share.path)}))}
    else{
     const url=document.createElement('input');url.className='field-input';url.readOnly=true;url.value=share.url;url.setAttribute('aria-label',mailImageText('Публичная ссылка','Public link'));url.onclick=()=>url.select();row.append(url,fileAction(mailImageText('Копировать','Copy'),async()=>{if(!alive())return;try{await navigator.clipboard.writeText(share.url);notify(mailImageText('Ссылка скопирована','Link copied'))}catch{url.focus();url.select()}}),button(mailImageText('Отозвать ссылку','Revoke link'),async()=>{if(await askConfirm(mailImageText('Отозвать публичную ссылку?','Revoke public link?'),{danger:true}))await mutate('/api/v1/files/shares/'+encodeURIComponent(share.id),'DELETE')}));
    }
    links.append(row);
   }
   if(!(data.shares||[]).some(item=>item.rights!=='upload'))links.append(text(document.createElement('p'),mailImageText('Публичных ссылок нет','No public links')));
   const form=document.createElement('form');form.className='file-card-form';
   const password=field(form,mailImageText('Пароль (необязательно)','Password (optional)'),document.createElement('input'));password.type='password';password.maxLength=256;password.autocomplete='new-password';
   const ttl=field(form,mailImageText('Срок действия, часов','Expires after, hours'),document.createElement('input'));ttl.type='number';ttl.min='1';ttl.max='8760';ttl.step='1';ttl.value='168';ttl.required=true;
   const max=field(form,mailImageText('Лимит скачиваний (0 — без лимита)','Download limit (0 — unlimited)'),document.createElement('input'));max.type='number';max.min='0';max.step='1';max.value='0';max.required=true;
   const rights=field(form,mailImageText('Права по ссылке','Link permissions'),rightsSelect());const submit=fileAction(mailImageText('Создать ссылку','Create link'),()=>{});submit.type='submit';form.append(submit);form.onsubmit=event=>{event.preventDefault();void act(()=>mutate('/api/v1/files/shares','POST',{path,password:password.value,ttl_hours:Number(ttl.value),max_downloads:Number(max.value),rights:rights.value}))};links.append(form);
   if(data.is_dir){
    const requests=section(mailImageText('Запросить файлы','Request files'));
    requests.append(text(document.createElement('p'),mailImageText('Получатель ссылки может только отправить документы. Просмотр папки и замена существующих файлов запрещены. После загрузки вы получите уведомление.','The link recipient can only upload documents. Folder browsing and overwriting existing files are forbidden. You will be notified after upload.')));
    for(const request of (data.shares||[]).filter(item=>item.rights==='upload')){
     const row=document.createElement('div');row.className='file-card-row';row.append(text(document.createElement('p'),(request.active?mailImageText('Приём открыт','Accepting files'):mailImageText('Приём закрыт','Closed'))+' · '+new Date(request.expires_at).toLocaleString()));
     if(request.inherited){row.append(text(document.createElement('p'),mailImageText('Запрос для родительской папки: ','Request for parent folder: ')+request.path),button(mailImageText('Управлять запросом в папке','Manage request in folder'),async()=>{view.dialog.close();await openFileCard(request.path)}))}
     else{const url=document.createElement('input');url.className='field-input';url.readOnly=true;url.value=request.url;url.setAttribute('aria-label',mailImageText('Ссылка для запроса файлов','File request link'));url.onclick=()=>url.select();row.append(url,fileAction(mailImageText('Копировать запрос','Copy request'),async()=>{if(!alive())return;try{await navigator.clipboard.writeText(request.url);notify(mailImageText('Ссылка скопирована','Link copied'))}catch{url.focus();url.select()}}),button(mailImageText('Закрыть приём','Close request'),async()=>{if(await askConfirm(mailImageText('Закрыть приём по этой ссылке?','Close this file request?')))await mutate('/api/v1/files/shares/'+encodeURIComponent(request.id),'DELETE')}))}
     requests.append(row);
    }
    const requestForm=document.createElement('form');requestForm.className='file-card-form';
    const expiry=field(requestForm,mailImageText('Срок запроса, часов','Request expiry, hours'),document.createElement('input'));expiry.type='number';expiry.min='1';expiry.max='8760';expiry.step='1';expiry.value='168';expiry.required=true;
    const create=fileAction(mailImageText('Создать запрос файлов','Create file request'),()=>{});create.type='submit';requestForm.append(create);requestForm.onsubmit=event=>{event.preventDefault();void act(()=>mutate('/api/v1/files/shares','POST',{path,rights:'upload',ttl_hours:Number(expiry.value)}))};requests.append(requestForm);
   }
   const events=section(mailImageText('Журнал событий','Activity'));
   const note=document.createElement('p');note.className='meta';note.textContent=mailImageText('События фиксируются с момента включения журнала. Скачивания и просмотры здесь не учитываются.','Events are recorded from the time activity logging is enabled. Downloads and views are not included.');events.append(note);
   const list=document.createElement('ol');list.className='file-card-events';events.append(list);
   let cursor=data.next_cursor;
   const renderEvents=entries=>{for(const event of entries){const row=document.createElement('li');const source=({web:'Web',webdav:'WebDAV',collabora:'Collabora',onlyoffice:'ONLYOFFICE',r7:'Р7',public_link:mailImageText('Публичная ссылка','Public link'),retention:mailImageText('Очистка по политике','Retention cleanup'),system:mailImageText('Система','System'),file_request:mailImageText('Запрос файлов','File request'),chat:mailImageText('Чат','Chat')})[event.actor?.source]||event.actor?.source||'';row.textContent=new Date(event.at).toLocaleString()+' · '+fileActivityLabel(event.action)+' · '+(event.actor_email||source)+(event.actor_email?' · '+source:'')+(event.subject?' · '+event.subject:'')+(event.rights?' · '+rightsLabel(event.rights):'')+(event.previous_path?' · '+event.previous_path+' → '+event.path:'');list.append(row)}};
   renderEvents(data.events||[]);if(!data.events?.length)events.append(text(document.createElement('p'),mailImageText('Событий пока нет','No events yet')));
   const more=fileAction(mailImageText('Ранее','Earlier'),async()=>{if(!cursor||busy||!alive())return;more.disabled=true;try{const page=await api(endpoint+'&before='+encodeURIComponent(cursor));if(!alive()||seq!==sequence)return;renderEvents(page.events||[]);cursor=page.next_cursor;more.hidden=!cursor}catch(error){notify(libraryMessage(error))}finally{more.disabled=false}});more.hidden=!cursor;events.append(more);
  }
  const refresh=fileAction(mailImageText('Обновить карточку','Refresh details'),async()=>{if(busy)return;refresh.disabled=true;try{await load()}catch(error){notify(libraryMessage(error))}finally{refresh.disabled=false}});view.body.append(refresh);
 }
 try{await load()}catch(error){notify(libraryMessage(error))}
}
function fileActivityLabel(action){return ({space_created:mailImageText('Пространство создано','Space created'),space_restored:mailImageText('Пространство восстановлено','Space restored'),space_updated:mailImageText('Настройки пространства изменены','Space settings updated'),file_received:mailImageText('Получен документ по запросу','Document received via file request'),created:mailImageText('Файл создан','File created'),updated:mailImageText('Файл изменён','File updated'),folder_created:mailImageText('Папка создана','Folder created'),moved:mailImageText('Перемещён / переименован','Moved / renamed'),trashed:mailImageText('Перемещён в корзину','Moved to trash'),restored:mailImageText('Восстановлен','Restored'),access_changed:mailImageText('Предоставлен / изменён доступ','Access granted / changed'),access_revoked:mailImageText('Доступ отозван','Access revoked'),link_created:mailImageText('Создана публичная ссылка','Public link created'),link_revoked:mailImageText('Публичная ссылка отозвана','Public link revoked'),versions_deleted:mailImageText('Версия удалена','Version deleted'),trash_deleted:mailImageText('Удалено из корзины','Trash permanently deleted'),versions_cleaned:mailImageText('Версия очищена по политике','Version removed by retention'),trash_cleaned:mailImageText('Корзина очищена по политике','Trash removed by retention'),protection_changed:mailImageText('Изменена защита версии','Version protection changed')})[action]||action}

async function openReceivedDocuments(){
 const view=fileLibraryDialog(mailImageText('Полученные документы','Received documents')),identity=state.email;
 const timer=setInterval(()=>{if(state.email!==identity)view.dialog.close()},500);view.dialog.addEventListener('close',()=>clearInterval(timer),{once:true});
 try{const result=await api('/api/v1/notifications?limit=100');if(!view.dialog.open||state.email!==identity){view.dialog.close();return}
  const list=document.createElement('ul');view.body.append(list);
  for(const event of (result.events||[]).filter(item=>item.kind==='files').reverse()){const row=document.createElement('li');row.textContent=new Date(event.created_at).toLocaleString()+' · '+event.body;list.append(row)}
  if(!list.children.length)view.status.textContent=mailImageText('Полученных документов пока нет','No received documents yet');
 }catch(error){view.status.textContent=libraryMessage(error)}
}
$('files-settings-menu')?.append(fileAction(mailImageText('Полученные документы','Received documents'),openReceivedDocuments));
