// Domain organization chart backed by distribution groups.
function directoryLabel(ru, en) { return mailImageText(ru, en); }
function directoryButton(text, action, primary=false) {
  const button=document.createElement('button');
  button.type='button';button.className=primary?'btn-spray':'btn-secondary';
  button.textContent=text;button.onclick=action;return button;
}
function directoryDialog(title, className='') {
  const dialog=document.createElement('dialog');dialog.className='org-dialog '+className;
  const header=document.createElement('header'),heading=document.createElement('h2');
  heading.id='org-title-'+crypto.randomUUID();heading.textContent=title;heading.className='heading';
  dialog.setAttribute('aria-labelledby',heading.id);
  const close=directoryButton(t('close'),()=>dialog.close());
  header.append(heading,close);dialog.append(header);
  dialog.addEventListener('close',()=>dialog.remove(),{once:true});
  document.body.append(dialog);dialog.showModal();return dialog;
}
function directoryErrorText(message) {
  const errors={
    'already exists':['Адрес уже занят пользователем, алиасом или группой.','Address is already in use.'],
    'directory changed; reload before saving':['Оргструктура изменилась. Обновите её и повторите действие.','The directory changed. Refresh it and try again.'],
    'department contains child departments':['Сначала перенесите или удалите вложенные отделы.','Move or remove child departments first.'],
    'distribution group has no active recipients':['В группе нет активных получателей.','The group has no active recipients.']
  };
  return errors[message]?directoryLabel(...errors[message]):message;
}
function directoryDescendants(groups, id) {
  const ids=new Set([id]);
  for(let changed=true;changed;){changed=false;for(const group of groups)if(ids.has(group.parent_id)&&!ids.has(group.id)){ids.add(group.id);changed=true;}}
  return ids;
}

async function openOrganization(domainID='') {
  const admin=!!domainID;
  const dialog=directoryDialog(directoryLabel('Оргструктура','Organization'));
  const toolbar=document.createElement('div');toolbar.className='org-toolbar';
  const search=document.createElement('input');search.type='search';search.className='field-input';
  search.placeholder=directoryLabel('Подразделение, имя или email…','Department, name or email…');search.setAttribute('aria-label',search.placeholder);
  const status=document.createElement('p');status.className='msg';status.setAttribute('role','status');
  const layout=document.createElement('div');layout.className='org-layout';
  const tree=document.createElement('nav');tree.className='org-tree';tree.setAttribute('aria-label',directoryLabel('Дерево подразделений','Department hierarchy'));
  const detail=document.createElement('section');detail.className='org-detail';
  const endpoint=admin?'/api/v1/admin/directory':'/api/v1/directory';
  const url=endpoint+(admin?'?domain_id='+encodeURIComponent(domainID):'');
  let data,selected='',page=0,includeChildren=true,sortColumn=0,sortDirection=1;
  const expanded=new Set(['']),limits=new Map();
  const refresh=directoryButton(t('refresh'),()=>load());
  toolbar.append(search,refresh);
  if(admin)toolbar.append(directoryButton(directoryLabel('Добавить подразделение','Add department'),()=>editDepartment(data,null,selected==='unassigned'?'':selected,load),true));
  layout.append(tree,detail);dialog.append(toolbar,status,layout);
  function compose(email){dialog.close();openCompose({to:email});}
  function select(id){selected=id;page=0;render();}
  function render(){
    if(!data||!dialog.isConnected)return;
    const query=search.value.trim().toLocaleLowerCase();
    const users=new Map(data.users.map(user=>[user.id,user]));
    const matches=(...values)=>!query||values.join(' ').toLocaleLowerCase().includes(query);
    const assigned=new Set(data.groups.flatMap(group=>group.member_ids));
    const children=id=>data.groups.filter(group=>group.parent_id===id);
    const branches=new Set(),matchedGroups=new Set(data.groups.filter(g=>matches(g.name,g.email)).map(g=>g.id));
    for(const group of data.groups){
      if(matchedGroups.has(group.id)||group.member_ids.some(id=>{const u=users.get(id);return u&&matches(u.name,u.email);})){
        let node=group;while(node&&!branches.has(node.id)){branches.add(node.id);node=data.groups.find(g=>g.id===node.parent_id);}
      }
    }
    tree.replaceChildren();
    const root=directoryButton(data.domain.name,()=>select(''));root.className='org-root'+(!selected?' is-active':'');tree.append(root);
    const build=(parent,host)=>{
      for(const group of children(parent)){
        if(query&&!branches.has(group.id))continue;
        const li=document.createElement('li'),branch=document.createElement('details'),summary=document.createElement('summary');
        branch.open=query?true:expanded.has(group.id);
        const label=document.createElement('span'),email=document.createElement('small');
        label.textContent=group.name;email.textContent=group.email;label.append(email);summary.append(label);
        summary.classList.toggle('is-active',selected===group.id);
        summary.addEventListener('click',()=>{selected=group.id;page=0;renderDetail();tree.querySelectorAll('.is-active').forEach(el=>el.classList.remove('is-active'));summary.classList.add('is-active');});
        branch.addEventListener('toggle',()=>{if(!query&&branch.isConnected){branch.open?expanded.add(group.id):expanded.delete(group.id);}});
        const list=document.createElement('ul');branch.append(summary,list);build(group.id,list);
        const direct=group.member_ids.map(id=>users.get(id)).filter(u=>u&&(!query||matchedGroups.has(group.id)||matches(u.name,u.email))).sort((a,b)=>(a.name||a.email).localeCompare(b.name||b.email,lang,{numeric:true,sensitivity:'base'}));
        const limit=limits.get(group.id)||25;
        for(const user of direct.slice(0,limit)){
          const item=document.createElement('li'),button=directoryButton(user.name||user.email,()=>compose(user.email)),address=document.createElement('small');
          button.className='org-mailbox';address.textContent=user.email;button.append(address);button.title=directoryLabel('Написать: ','Write to: ')+user.email;item.append(button);list.append(item);
        }
        if(direct.length>limit){const more=document.createElement('li');more.append(directoryButton(directoryLabel('Показать ещё','Show more'),()=>{limits.set(group.id,limit+25);render();}));list.append(more);}
        li.append(branch);host.append(li);
      }
    };
    const list=document.createElement('ul');build('',list);tree.append(list);
    tree.append(directoryButton(directoryLabel('Без подразделения','Unassigned')+' · '+data.users.filter(u=>!assigned.has(u.id)).length,()=>select('unassigned')));
    renderDetail();
  }
  function renderDetail(){
    if(!data)return;
    const group=data.groups.find(g=>g.id===selected),query=search.value.trim().toLocaleLowerCase();
    const selectedGroups=group?(includeChildren?directoryDescendants(data.groups,group.id):new Set([group.id])):null;
    const assigned=new Set(data.groups.flatMap(g=>g.member_ids));
    const memberIDs=selectedGroups?new Set(data.groups.filter(g=>selectedGroups.has(g.id)).flatMap(g=>g.member_ids)):null;
    const departmentsOf=user=>data.groups.filter(g=>g.member_ids.includes(user.id)).map(g=>g.name).join(', ');
    const visible=data.users.filter(u=>(selected==='unassigned'?!assigned.has(u.id):!memberIDs||memberIDs.has(u.id))&&(!query||[u.name,u.email,...data.groups.filter(g=>g.member_ids.includes(u.id)).map(g=>g.name+' '+g.email)].join(' ').toLocaleLowerCase().includes(query)));
    const sortValue=user=>[user.name||user.email,user.email,departmentsOf(user)][sortColumn];
    visible.sort((a,b)=>sortDirection*(sortValue(a).localeCompare(sortValue(b),lang,{numeric:true,sensitivity:'base'})||a.email.localeCompare(b.email)));
    detail.replaceChildren();
    const title=document.createElement('h3');title.className='heading';title.textContent=group?.name||(selected==='unassigned'?directoryLabel('Без подразделения','Unassigned'):data.domain.name);
    const address=document.createElement('p');address.className='meta';address.textContent=group?.email||directoryLabel('Адресный справочник домена','Domain address directory');
    const actions=document.createElement('div');actions.className='org-toolbar';
    if(group){
      actions.append(directoryButton(directoryLabel('Написать подразделению','Email department'),()=>compose(group.email)));
      if(admin){
        actions.append(directoryButton(directoryLabel('Изменить','Edit'),()=>editDepartment(data,group,'',load)));
        const remove=directoryButton(directoryLabel('Удалить','Delete'),async()=>{
          if(!await askConfirm(directoryLabel('Удалить подразделение и его адрес рассылки? Почтовые ящики сотрудников сохранятся.','Delete this department and its distribution address? Employee mailboxes will be kept.'),{danger:true}))return;
          remove.disabled=true;
          try{await api(endpoint+'/'+encodeURIComponent(group.id)+'?domain_id='+encodeURIComponent(domainID)+'&revision='+group.revision,{method:'DELETE'});selected=group.parent_id;await load();}
          catch(e){status.textContent=directoryErrorText(e.message);remove.disabled=false;}
        });
        remove.disabled=data.groups.some(g=>g.parent_id===group.id);
        if(remove.disabled)remove.title=directoryLabel('Сначала перенесите или удалите вложенные отделы','Move or remove child departments first');
        actions.append(remove);
      }
    }
    detail.append(title,address,actions);
    if(group?.description){const description=document.createElement('p');description.textContent=group.description;detail.append(description);}
    if(group){
      const label=document.createElement('label'),check=document.createElement('input');label.className='field-check';
      check.type='checkbox';check.checked=includeChildren;check.onchange=()=>{includeChildren=check.checked;page=0;renderDetail();};
      label.append(check,document.createTextNode(directoryLabel('Включая вложенные отделы','Include subdepartments')));detail.append(label);
    }
    const count=document.createElement('p');count.className='meta';count.textContent=directoryLabel('Сотрудников: ','Employees: ')+visible.length;detail.append(count);
    page=Math.max(0,Math.min(page,Math.ceil(visible.length/25)-1));
    const wrap=document.createElement('div');wrap.className='org-table-wrap';
    const table=document.createElement('table');table.className='admin-users-table';
    const head=document.createElement('thead'),row=document.createElement('tr');
    [directoryLabel('Имя','Name'),'Email',directoryLabel('Подразделения','Departments')].forEach((label,column)=>{
      const th=document.createElement('th');th.scope='col';th.dataset.sortMounted='true';
      const button=directoryButton(label,()=>{sortDirection=sortColumn===column?-sortDirection:1;sortColumn=column;page=0;renderDetail();detail.querySelectorAll('th button')[column]?.focus();});
      button.className='org-sort';
      if(sortColumn===column){th.setAttribute('aria-sort',sortDirection===1?'ascending':'descending');button.append(document.createTextNode(sortDirection===1?' ↑':' ↓'));}
      th.append(button);row.append(th);
    });head.append(row);table.append(head);
    const body=document.createElement('tbody');
    for(const user of visible.slice(page*25,(page+1)*25)){
      const tr=document.createElement('tr');tr.className='org-person-row';tr.tabIndex=0;tr.setAttribute('aria-label',directoryLabel('Написать: ','Write to: ')+(user.name||user.email));
      const name=document.createElement('td'),email=document.createElement('td'),departments=document.createElement('td');
      name.textContent=user.name||user.email;if(!user.enabled)name.append(document.createTextNode(directoryLabel(' · отключён',' · disabled')));
      email.textContent=user.email;departments.textContent=departmentsOf(user)||'—';
      tr.append(name,email,departments);tr.onclick=()=>compose(user.email);tr.onkeydown=e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();compose(user.email);}};body.append(tr);
    }
    table.append(body);wrap.append(table);detail.append(wrap);
    if(!visible.length){const empty=document.createElement('p');empty.className='meta';empty.textContent=directoryLabel('Сотрудники не найдены','No employees found');detail.append(empty);}
    const pager=document.createElement('div');pager.className='org-toolbar';
    const prev=directoryButton('‹',()=>{page--;renderDetail();}),next=directoryButton('›',()=>{page++;renderDetail();}),range=document.createElement('span');
    prev.setAttribute('aria-label',directoryLabel('Предыдущая страница','Previous page'));next.setAttribute('aria-label',directoryLabel('Следующая страница','Next page'));prev.disabled=page===0;next.disabled=(page+1)*25>=visible.length;
    range.textContent=(visible.length?page*25+1:0)+'–'+Math.min((page+1)*25,visible.length)+' / '+visible.length;
    pager.append(prev,range,next);detail.append(pager);
  }
  async function load(){
    status.textContent=directoryLabel('Загрузка…','Loading…');refresh.disabled=true;
    try{
      const result=await api(url);if(!dialog.isConnected)return;
      if(!data)result.groups.filter(g=>!g.parent_id).forEach(g=>expanded.add(g.id));
      data=result;
      data.users.sort((a,b)=>(a.name||a.email).localeCompare(b.name||b.email,lang,{numeric:true,sensitivity:'base'}));
      if(selected&&selected!=='unassigned'&&!data.groups.some(g=>g.id===selected))selected='';
      status.textContent='';render();
    }
    catch(e){status.textContent=directoryErrorText(e.message);}
    finally{refresh.disabled=false;}
  }
  search.oninput=()=>{page=0;render();};
  await load();
}

function editDepartment(data, group, parent, saved) {
  if(!data)return;
  const dialog=directoryDialog(directoryLabel(group?'Изменить подразделение':'Новое подразделение',group?'Edit department':'New department'),'org-editor');
  const form=document.createElement('form');form.className='stack-gap';
  function field(title,control){const label=document.createElement('label');label.className='field';const text=document.createElement('span');text.className='field-label';text.textContent=title;label.append(text,control);form.append(label);return control;}
  const name=field(directoryLabel('Название','Name'),document.createElement('input'));name.className='field-input';name.required=true;name.maxLength=200;name.value=group?.name||'';
  const local=field(directoryLabel('Адрес группы — до @'+data.domain.name,'Group address — before @'+data.domain.name),document.createElement('input'));local.className='field-input';local.required=true;local.maxLength=64;local.value=group?.local_part||'';
  const parentSelect=field(directoryLabel('Входит в подразделение','Parent department'),document.createElement('select'));parentSelect.className='field-input';
  const excluded=group?directoryDescendants(data.groups,group.id):new Set();
  for(const entry of [{id:'',name:data.domain.name},...data.groups.filter(g=>!excluded.has(g.id))]){const option=document.createElement('option');option.value=entry.id;option.textContent=entry.name+(entry.email?' · '+entry.email:'');parentSelect.append(option);}parentSelect.value=group?.parent_id||parent;
  const description=field(directoryLabel('Описание','Description'),document.createElement('textarea'));description.className='field-input';description.maxLength=2000;description.value=group?.description||'';
  const hint=document.createElement('p');hint.className='meta';hint.textContent=directoryLabel('Письма на адрес группы получают выбранные сотрудники и сотрудники вложенных отделов. Один сотрудник может входить в несколько подразделений.','Mail to this group reaches selected employees and subdepartments. Employees can belong to multiple departments.');form.append(hint);
  const search=field(directoryLabel('Состав подразделения','Department members'),document.createElement('input'));search.type='search';search.className='field-input';search.placeholder=directoryLabel('Поиск имени или email','Search name or email');
  const members=new Set(group?.member_ids||[]),list=document.createElement('div'),pager=document.createElement('div'),count=document.createElement('p');
  list.className='org-member-picker';pager.className='org-toolbar';count.className='meta';count.setAttribute('role','status');form.append(count,list,pager);
  let page=0;
  function renderMembers(){
    const q=search.value.trim().toLocaleLowerCase(),users=data.users.filter(u=>!q||(u.name+' '+u.email).toLocaleLowerCase().includes(q));page=Math.max(0,Math.min(page,Math.ceil(users.length/50)-1));list.replaceChildren();
    count.textContent=directoryLabel('Выбрано: ','Selected: ')+members.size;
    for(const user of users.slice(page*50,(page+1)*50)){
      const label=document.createElement('label'),check=document.createElement('input'),text=document.createElement('span');label.className='field-check';check.type='checkbox';check.checked=members.has(user.id);
      text.textContent=(user.name||user.email)+' · '+user.email+(!user.enabled?directoryLabel(' · отключён',' · disabled'):'');
      check.onchange=()=>{check.checked?members.add(user.id):members.delete(user.id);count.textContent=directoryLabel('Выбрано: ','Selected: ')+members.size;};label.append(check,text);list.append(label);
    }
    pager.replaceChildren();const prev=directoryButton('‹',()=>{page--;renderMembers();}),next=directoryButton('›',()=>{page++;renderMembers();}),range=document.createElement('span');
    prev.setAttribute('aria-label',directoryLabel('Предыдущая страница','Previous page'));next.setAttribute('aria-label',directoryLabel('Следующая страница','Next page'));prev.disabled=page===0;next.disabled=(page+1)*50>=users.length;range.textContent=(users.length?page*50+1:0)+'–'+Math.min((page+1)*50,users.length)+' / '+users.length;pager.append(prev,range,next);
  }
  search.oninput=()=>{page=0;renderMembers();};renderMembers();
  const status=document.createElement('p');status.className='msg';status.setAttribute('role','status');
  const actions=document.createElement('div');actions.className='org-toolbar';const save=directoryButton(directoryLabel('Сохранить','Save'),null,true);save.type='submit';actions.append(directoryButton(t('cancel'),()=>dialog.close()),save);form.append(status,actions);dialog.append(form);
  form.onsubmit=async event=>{
    event.preventDefault();if(save.disabled)return;save.disabled=true;
    try{
      await api('/api/v1/admin/directory'+(group?'/'+encodeURIComponent(group.id):'')+'?domain_id='+encodeURIComponent(data.domain.id),{method:group?'PUT':'POST',body:JSON.stringify({name:name.value,local_part:local.value,parent_id:parentSelect.value,description:description.value,member_ids:[...members],revision:group?.revision||0})});
      composeContactsCache=null;dialog.close();await saved();
    }catch(e){status.textContent=directoryErrorText(e.message);save.disabled=false;}
  };name.focus();
}
$('btn-org-structure')?.addEventListener('click',()=>openOrganization());
$('btn-org-structure-admin')?.addEventListener('click',()=>{if(tenantNav.domainID)openOrganization(tenantNav.domainID);});
