/* Address books share the same personal contacts and organization directory as suggestions. */
(() => {
 const words=workspaceWords;
 for(const id of ['compose-to','compose-cc']){
  const input=$(id);if(!input)continue;
  const row=document.createElement('div');row.className='compose-recipient-control';input.before(row);row.append(input);
  const button=document.createElement('button');button.type='button';button.className='btn-secondary action-icon';button.title=words('Адресная книга','Address book');button.setAttribute('aria-label',button.title);button.innerHTML=workspaceIcon('contacts');row.append(button);
  button.onclick=async()=>{
   const identity=state.tokens?.access_token;
   const dialog=document.createElement('dialog');dialog.className='recipient-dialog';dialog.setAttribute('aria-label',button.title);
   dialog.innerHTML='<header><strong>'+button.title+'</strong><button type="button" class="btn-secondary action-icon" data-close aria-label="'+words('Закрыть','Close')+'">'+designIcon('close')+'</button></header><input type="search" class="field-input" placeholder="'+words('Имя или адрес почты','Name or email')+'" aria-label="'+words('Поиск контактов','Search contacts')+'"><p class="meta" role="status">'+words('Загрузка контактов…','Loading contacts…')+'</p><div class="recipient-list"></div><footer><button type="button" class="btn-spray" data-add disabled>'+words('Добавить получателей','Add recipients')+'</button></footer>';
   document.body.append(dialog);dialog.showModal();const search=dialog.querySelector('input'),status=dialog.querySelector('[role=status]'),list=dialog.querySelector('.recipient-list'),add=dialog.querySelector('[data-add]'),selected=new Set();let contacts=[];
   dialog.querySelector('[data-close]').onclick=()=>dialog.close();dialog.onclose=()=>{dialog.remove();button.focus()};search.focus();
   const render=()=>{list.replaceChildren();const query=search.value.toLocaleLowerCase();const visible=contacts.filter(c=>(c.name+' '+c.email).toLocaleLowerCase().includes(query));status.textContent=words('Найдено: ','Found: ')+visible.length+words(' · Выбрано: ',' · Selected: ')+selected.size;for(const c of visible.slice(0,100)){const label=document.createElement('label'),check=document.createElement('input'),text=document.createElement('span'),name=document.createElement('strong'),email=document.createElement('small');check.type='checkbox';check.checked=selected.has(c.email);name.textContent=c.name;email.textContent=c.email;text.append(name,email);check.onchange=()=>{check.checked?selected.add(c.email):selected.delete(c.email);add.disabled=!selected.size;status.textContent=words('Выбрано: ','Selected: ')+selected.size};label.append(check,text);list.append(label)}if(visible.length>100)status.textContent+=' · '+words('Уточните поиск для остальных контактов','Refine the search to find more contacts')};
   search.oninput=render;add.onclick=()=>{if(identity!==state.tokens?.access_token){dialog.close();return}const existing=input.value.split(/[,;]/).map(s=>s.trim()).filter(Boolean),seen=new Set(existing.map(s=>s.toLowerCase()));for(const address of selected)if(!seen.has(address.toLowerCase())){seen.add(address.toLowerCase());existing.push(address)}input.value=existing.join(', ');input.dispatchEvent(new Event('change',{bubbles:true}));dialog.close();input.focus()};
   try{composeContactsCache=null;contacts=await composeContacts();if(!dialog.open||identity!==state.tokens?.access_token)return;render()}catch(e){if(dialog.open)status.textContent=e.message}
  };
 }
})();
