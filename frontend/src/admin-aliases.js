function aliasErrorText(message) {
  const messages={
    'address already exists':['Этот адрес уже занят доменом, ящиком, алиасом или группой.','This address is already used by a domain, mailbox, alias or group.'],
    'invalid alias or target mailbox':['Проверьте адрес алиаса и целевой ящик.','Check the alias address and target mailbox.'],
    'alias or target not found':['Алиас или ящик уже удалён. Обновите список.','The alias or mailbox was deleted. Reload the list.'],
    'forbidden':['Недостаточно прав для управления этим адресом.','You do not have permission to manage this address.'],
    'global admin required':['Алиасами доменов управляет глобальный администратор.','Domain aliases require a global administrator.'],
    'cannot save alias':['Не удалось сохранить алиас. Повторите попытку.','Could not save the alias. Please try again.']
  };
  return messages[message]?directoryLabel(...messages[message]):message;
}
async function openAdminAliases(kind, target) {
  const isDomain=kind==='domain';
  const title=isDomain?directoryLabel('Алиасы домена','Domain aliases'):directoryLabel('Алиасы ящика','Mailbox aliases');
  const dialog=directoryDialog(title+' — '+(isDomain?target.name:target.email),'org-editor');
  dialog.classList.add('alias-dialog');
  const description=document.createElement('p');description.className='meta';
  description.textContent=isDomain
    ?directoryLabel('Адреса с тем же именем получателя доставляются в основной домен. Пользователи могут отправлять с этих адресов. Настройте MX, SPF и DKIM домена-алиаса.','Addresses with the same local part deliver to the primary domain and are available for sending. Configure MX, SPF and DKIM for the alias domain.')
    :directoryLabel('Дополнительные адреса доступны для получения и отправки почты. Вход выполняется по основному адресу.','Additional addresses are available for receiving and sending mail. Sign in with the primary address.');
  const status=document.createElement('p');status.className='msg';status.setAttribute('role','status');
  const list=document.createElement('div');list.className='alias-list';
  const form=document.createElement('form');form.className='alias-form';
  const addressLabel=document.createElement('label');addressLabel.className='field';
  addressLabel.textContent=isDomain?directoryLabel('Домен-алиас','Alias domain'):directoryLabel('Имя до @','Local part before @');
  const input=document.createElement('input');input.className='field-input';input.required=true;input.maxLength=isDomain?253:64;input.autocomplete='off';input.spellcheck=false;
  input.placeholder=isDomain?'example.org':'sales';addressLabel.append(input);form.append(addressLabel);
  const domainLabel=document.createElement('label');domainLabel.className='field';domainLabel.textContent=directoryLabel('Домен адреса','Address domain');
  const domainSelect=document.createElement('select');domainSelect.className='field-input';domainLabel.append(domainSelect);
  if(!isDomain)form.append(domainLabel);
  const actions=document.createElement('div');actions.className='org-toolbar';
  const save=directoryButton(directoryLabel('Добавить алиас','Add alias'),null,true);save.type='submit';
  const cancel=directoryButton(t('cancel'),()=>reset());cancel.hidden=true;actions.append(save,cancel);form.append(actions);
  dialog.append(description,status,list,form);
  const base='/api/v1/admin/'+(isDomain?'domains/':'users/')+encodeURIComponent(target.id)+'/aliases';
  let aliases=[],editing=null,busy=false;
  const writable=!isDomain||!!state.me?.is_global_admin;
  form.hidden=!writable;
  function reset(){editing=null;input.value='';domainSelect.disabled=false;save.textContent=directoryLabel('Добавить алиас','Add alias');cancel.hidden=true;}
  function lock(value){busy=value;dialog.querySelectorAll('button,input,select').forEach(el=>el.disabled=value);if(!value&&editing)domainSelect.disabled=true;}
  function render(){
    list.replaceChildren();
    if(!aliases.length){const empty=document.createElement('p');empty.className='meta';empty.textContent=directoryLabel('Алиасов пока нет.','No aliases yet.');list.append(empty);}
    for(const alias of aliases){
      const row=document.createElement('div');row.className='alias-row';
      const address=document.createElement('span');address.textContent=isDomain?alias.name:alias.email;row.append(address);
      if(writable){
        const controls=document.createElement('div');controls.className='org-toolbar';
        controls.append(directoryButton(directoryLabel('Изменить','Edit'),()=>{
          editing=alias;input.value=isDomain?alias.name:alias.local_part;
          domainSelect.value=alias.domain_id;domainSelect.disabled=true;
          save.textContent=directoryLabel('Сохранить','Save');cancel.hidden=false;input.focus();
        }),directoryButton(directoryLabel('Удалить','Delete'),async()=>{
          if(busy||!await askConfirm(directoryLabel('Удалить алиас ','Delete alias ')+(isDomain?alias.name:alias.email)+'?',{danger:true}))return;
          lock(true);setMsg(status,'','');
          try{await api(base+'/'+encodeURIComponent(alias.id),{method:'DELETE'});reset();await reload();}
          catch(error){setMsg(status,aliasErrorText(error.message),'err');}finally{lock(false);}
        }));row.append(controls);
      }
      list.append(row);
    }
  }
  async function reload(){const data=await api(base);if(!dialog.isConnected)return;aliases=data.aliases||[];render();}
  form.addEventListener('submit',async event=>{
    event.preventDefault();if(busy)return;lock(true);setMsg(status,'','');
    const payload=isDomain?{name:input.value.trim()}:{local_part:input.value.trim(),domain_id:editing?editing.domain_id:domainSelect.value};
    try{await api(base+(editing?'/'+encodeURIComponent(editing.id):''),{method:editing?'PATCH':'POST',body:JSON.stringify(payload)});reset();await reload();setMsg(status,directoryLabel('Алиас сохранён.','Alias saved.'),'ok');}
    catch(error){setMsg(status,aliasErrorText(error.message),'err');}finally{lock(false);}
  });
  lock(true);
  try{
    if(!isDomain){
      const data=await api('/api/v1/admin/domains?tenant_id='+encodeURIComponent(target.tenant_id));
      for(const domain of data.domains||[]){const option=document.createElement('option');option.value=domain.id;option.textContent='@'+domain.name;domainSelect.append(option);}
      domainSelect.value=target.domain_id;
    }
    await reload();
  }catch(error){setMsg(status,aliasErrorText(error.message),'err');}finally{lock(false);}
}
