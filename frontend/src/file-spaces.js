// Organization-owned storage. Group roles and settings are authoritative on the server.
function fileSpaceRoleLabel(value){return ({read:mailImageText('Чтение','Read'),write:mailImageText('Чтение и запись','Read and write'),manage:mailImageText('Управление','Manage')})[value]||value}
async function openFileSpaces(){
 const view=fileLibraryDialog(mailImageText('Общие пространства','Shared spaces')),identity=state.email;
 const timer=setInterval(()=>{if(state.email!==identity)view.dialog.close()},500);view.dialog.addEventListener('close',()=>clearInterval(timer),{once:true});
 try{const data=await api('/api/v1/files/spaces');if(!view.dialog.open||state.email!==identity)return;
  const note=document.createElement('p');note.textContent=mailImageText('Пространства принадлежат организации. Доступ определяется группами; файлы сохраняются при смене сотрудников.','Spaces belong to the organization. Group membership determines access; files survive staff changes.');view.body.append(note);
  for(const space of data.spaces){const row=document.createElement('section');row.className='file-card-section';const title=document.createElement('strong');title.textContent=space.name;const info=document.createElement('p');info.textContent=(space.kind==='department'?mailImageText('Отдел','Department'):mailImageText('Проект','Project'))+' · '+fileSpaceRoleLabel(space.rights)+' · '+(space.quota_bytes?fmtBytes(space.quota_bytes):mailImageText('Без ограничения квоты','Unlimited quota'));row.append(title,info);
   row.append(fileAction(mailImageText('Открыть','Open'),async()=>{if(state.email!==identity)return;filesState.ownerID=space.id;filesState.spaceID=space.id;filesState.spaceName=space.name;filesState.spaceRights=space.rights;filesState.sharedRights=space.rights==='read'?'read':'write';filesState.sharedRoot='';filesState.listTarget='';filesState.path='';filesState.selected=null;view.dialog.close();await refreshFiles();if(typeof updateCreateButton==='function')updateCreateButton('files')}));
   if(space.rights==='manage')row.append(fileAction(mailImageText('Резервирование и откат','Backup and recovery'),()=>{view.dialog.close();return openSpaceRecovery(space.id)}));
   if(space.rights==='manage')row.append(fileAction(mailImageText('Настройки','Settings'),()=>{view.dialog.close();return openFileSpaceSettings(space.id)}));
   view.body.append(row);
  }
  if(!data.spaces.length){const empty=document.createElement('p');empty.textContent=mailImageText('Доступных пространств пока нет','No shared spaces available');view.body.append(empty)}
  if(data.can_create)view.body.append(fileAction(mailImageText('Восстановить пространство из архива','Restore space from archive'),()=>{view.dialog.close();return openSpaceRecovery()}));
  if(data.can_create)view.body.append(fileAction(mailImageText('Создать пространство','Create space'),()=>{view.dialog.close();return openFileSpaceSettings('',data.tenant_id)}));
 }catch(error){view.status.textContent=libraryMessage(error)}
}
async function openFileSpaceSettings(id='',tenant=''){
 const view=fileLibraryDialog(id?mailImageText('Настройки пространства','Space settings'):mailImageText('Новое пространство','New space')),identity=state.email;
 const timer=setInterval(()=>{if(state.email!==identity)view.dialog.close()},500);view.dialog.addEventListener('close',()=>clearInterval(timer),{once:true});
 try{
  let space=id?await api('/api/v1/files/spaces/'+encodeURIComponent(id)):{tenant_id:tenant,name:'',kind:'project',description:'',quota_bytes:0,version_days:0,trash_days:0,keep_versions:0,automatic:false,revision:0,groups:[]};
  if(!view.dialog.open||state.email!==identity)return;
  if(id&&space.rights!=='manage'){view.status.textContent=mailImageText('Требуются права управления','Management rights required');return}
  const choices=await api('/api/v1/files/spaces/groups?'+(id?'space_id='+encodeURIComponent(id):'tenant_id='+encodeURIComponent(tenant)));if(!view.dialog.open||state.email!==identity)return;
  const form=document.createElement('form');form.className='file-card-form';view.body.append(form);
  const field=(label,input)=>{const row=document.createElement('label');row.textContent=label;input.className='field-input';input.setAttribute('aria-label',label);row.append(input);form.append(row);return input};
  const name=field(mailImageText('Название','Name'),document.createElement('input'));name.required=true;name.maxLength=200;name.value=space.name;
  const kind=field(mailImageText('Тип пространства','Space type'),document.createElement('select'));kind.add(new Option(mailImageText('Отдел','Department'),'department'));kind.add(new Option(mailImageText('Проект','Project'),'project'));kind.value=space.kind;
  const description=field(mailImageText('Описание','Description'),document.createElement('textarea'));description.maxLength=2000;description.value=space.description;
  const number=(label,value,max)=>{const input=field(label,document.createElement('input'));input.type='number';input.min='0';input.max=String(max);input.step='1';input.value=String(value);input.required=true;return input};
  const quota=number(mailImageText('Квота в байтах (0 — без ограничения)','Quota in bytes (0 — unlimited)'),space.quota_bytes,Number.MAX_SAFE_INTEGER);
  const versions=number(mailImageText('Хранить версии, дней (0 — бессрочно)','Version retention, days (0 — forever)'),space.version_days,36500);
  const trash=number(mailImageText('Хранить корзину, дней (0 — бессрочно)','Trash retention, days (0 — forever)'),space.trash_days,36500);
  const keep=number(mailImageText('Минимум последних версий','Minimum latest versions'),space.keep_versions,10000);
  const automatic=document.createElement('input');automatic.type='checkbox';automatic.checked=space.automatic;const automaticLabel=document.createElement('label');automaticLabel.textContent=mailImageText('Автоматическая очистка','Automatic cleanup');automaticLabel.prepend(automatic);form.append(automaticLabel);
  const hint=document.createElement('p');hint.className='meta';hint.textContent=mailImageText('Квота учитывает текущие файлы, версии и корзину. Понижение квоты не удаляет данные. 0 дней — бессрочное хранение. Администраторы организации сохраняют управление независимо от состава групп.','Quota includes current files, versions and trash. Lowering the quota does not delete data. 0 days means keep forever. Organization administrators retain management regardless of group membership.');form.append(hint);
  const groups=document.createElement('div');form.append(groups);const grants=new Map((space.groups||[]).map(grant=>[grant.group_id,grant.rights]));
  const render=()=>{groups.replaceChildren();for(const [groupID,rights] of grants){const group=choices.groups.find(g=>g.id===groupID),row=document.createElement('div');row.className='file-card-row';const label=document.createElement('span');label.textContent=group?group.name+' · '+group.email:groupID;const select=document.createElement('select');select.className='field-input';select.setAttribute('aria-label',mailImageText('Права группы: ','Group role: ')+(group?.name||groupID));for(const role of ['read','write','manage'])select.add(new Option(fileSpaceRoleLabel(role),role));select.value=rights;select.onchange=()=>grants.set(groupID,select.value);row.append(label,select,fileAction(mailImageText('Убрать группу','Remove group'),()=>{grants.delete(groupID);render()}));groups.append(row)}};
  const group=field(mailImageText('Добавить группу','Add group'),document.createElement('select'));group.add(new Option(mailImageText('Выберите группу','Select a group'),''));for(const choice of choices.groups)group.add(new Option(choice.name+' · '+choice.email,choice.id));group.onchange=()=>{if(group.value){grants.set(group.value,'read');group.value='';render()}};render();
  const statusNote=document.createElement('p');statusNote.className='meta';form.append(statusNote);
  if(id){const usage=await api('/api/v1/files/library/usage?owner_id='+encodeURIComponent(id));statusNote.textContent=mailImageText('Занято: ','Used: ')+fmtBytes(usage.used_bytes)+' · '+mailImageText('WebDAV: ','WebDAV: ')+location.origin+'/dav/spaces/'+id+'/'}
  const save=fileAction(mailImageText('Сохранить пространство','Save space'),()=>{});save.type='submit';form.append(save);let busy=false;
  form.onsubmit=async event=>{event.preventDefault();if(busy||!view.dialog.open||state.email!==identity)return;busy=true;save.disabled=true;
   try{const value={...space,name:name.value,kind:kind.value,description:description.value,quota_bytes:Number(quota.value),version_days:Number(versions.value),trash_days:Number(trash.value),keep_versions:Number(keep.value),automatic:automatic.checked,groups:[...grants].map(([group_id,rights])=>({group_id,rights}))};
    const saved=await api('/api/v1/files/spaces'+(id?'/'+encodeURIComponent(id):''),{method:id?'PUT':'POST',body:JSON.stringify(value)});if(!view.dialog.open||state.email!==identity)return;space=saved;if(filesState.spaceID===saved.id){filesState.spaceName=saved.name;filesState.spaceRights=saved.rights;filesState.sharedRights=saved.rights==='read'?'read':'write';if(!saved.rights){filesState.ownerID='';filesState.spaceID='';filesState.path='';filesState.listTarget='';filesState.sharedRoot=''}await refreshFiles()}if(!id){view.dialog.close();await openFileSpaces()}else{view.status.textContent=mailImageText('Пространство сохранено','Space saved');if(saved.rights!=='manage')view.dialog.close()}
   }catch(error){view.status.textContent=error.status===409?mailImageText('Пространство изменено другим управляющим. Откройте настройки заново; введённые значения сохранены в форме.','Another manager changed the space. Reopen settings; your inputs remain in this form.'):libraryMessage(error)}finally{busy=false;save.disabled=false}
  };
 }catch(error){view.status.textContent=libraryMessage(error)}
}
const sharedSpacesButton=fileAction(mailImageText('Общие пространства','Shared spaces'),openFileSpaces);$('btn-files-refresh')?.after(sharedSpacesButton);
