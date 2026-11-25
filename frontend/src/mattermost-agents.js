/* Native TMS Agents product. Tenant, domain and user are bound by the server. */
(function installAgents() {
  const anchor=document.querySelector('[data-app="chat"]'),parent=document.getElementById('app-chat')?.parentElement;
  if(!anchor||!parent)return;
  if(!APPS.includes('agents'))APPS.push('agents');
  const button=document.createElement('button');button.type='button';button.className=anchor.className;button.classList.remove('is-active');button.dataset.app='agents';button.textContent='✧ Агенты';button.title='Агенты';anchor.after(button);
  const section=document.createElement('section');section.id='app-agents';section.className='hidden';
  section.innerHTML=`<div class="tms-agents"><aside class="agents-sidebar"><h2>✧ Агенты</h2><button type="button" data-view="catalog">⌕ Найти агентов</button><button type="button" data-view="catalog">Все агенты</button><button type="button" data-view="history">Мои диалоги</button><button type="button" data-view="skills">Навыки</button><button type="button" data-view="settings">Настройки</button><div class="agents-side-history"></div><p class="agents-scope"></p></aside><div class="agents-workspace"><div class="agents-error" role="alert"></div><main class="agents-main"></main></div></div>`;
  parent.append(section);
  const main=section.querySelector('.agents-main'),error=section.querySelector('.agents-error'),sidebar=section.querySelector('.agents-side-history');
  let owner='',ownerToken='',epoch=0,revision=0,loadRevision=0,boot=null,config=null,history=[],active='',view='catalog',sending=false,controller=new AbortController();
  const node=(tag,text,className)=>{const el=document.createElement(tag);if(text!==undefined)el.textContent=text;if(className)el.className=className;return el;};
  const action=(text,run,className)=>{const el=node('button',text,className);el.type='button';el.onclick=run;return el;};
  function guard(){const e=epoch,email=owner,token=ownerToken;return()=>{if(e!==epoch||email!==state.email||!token||token!==state.tokens?.access_token)throw Error('Учётная запись изменилась');};}
  async function request(path,options={}){const check=guard();try{check();const out=await api('/api/v4/tms/agents'+path,{...options,signal:controller.signal,_guard:check});check();return out;}catch(err){if(ownerToken!==state.tokens?.access_token){reset();if(state.tokens?.access_token&&!section.classList.contains('hidden'))queueMicrotask(()=>run(load));}throw err;}}
  function report(err){if(err.name!=='AbortError')error.textContent=err.data?.message||err.message;}
  async function run(fn){const version=epoch;error.textContent='';try{await fn();}catch(err){if(version===epoch)report(err);}}
  function reset(){epoch++;revision++;loadRevision++;controller.abort();controller=new AbortController();owner='';ownerToken='';boot=null;config=null;history=[];active='';sending=false;sidebar.replaceChildren();main.replaceChildren();error.textContent='';section.querySelector('.agents-scope').textContent='';document.querySelectorAll('dialog[data-agents-dialog]').forEach(el=>el.remove());}
  async function load(){
    if(!state.tokens?.access_token){reset();return;}
    if(owner!==state.email||ownerToken!==state.tokens.access_token){reset();owner=state.email;ownerToken=state.tokens.access_token;}
    const check=guard(),version=++loadRevision;const nextBoot=await request('');check();
    const nextHistory=await request('/conversations');check();if(version!==loadRevision)return;boot=nextBoot;history=nextHistory;
    section.querySelector('.agents-scope').textContent=state.email||'Мой домен';renderSidebar();if(active)await openChat(active);else await show(view==='chat'?'catalog':view);
  }
  function renderSidebar(){
    sidebar.replaceChildren();
    for(const agent of boot.catalog.agents){
      const heading=node('div',undefined,'agents-history-heading');heading.append(node('span',agent.name),action('+',()=>run(()=>newChat(agent.id))));heading.lastChild.setAttribute('aria-label','Новый диалог: '+agent.name);sidebar.append(heading);
      for(const chat of history.filter(c=>c.agent_id===agent.id).sort((a,b)=>b.updated_at-a.updated_at)){
        const b=action(chat.title,()=>run(()=>openChat(chat.id)),'agents-history-link');b.classList.toggle('is-selected',active===chat.id);sidebar.append(b);
      }
    }
  }
  function title(text,subtitle){const header=node('header',undefined,'agents-header');const block=node('div');block.append(node('h1',text));if(subtitle)block.append(node('p',subtitle));header.append(block);main.append(header);return header;}
  async function show(next){
    view=next;revision++;active='';main.replaceChildren();renderSidebar();
    if(!boot)return;
    if(next==='catalog'){renderCatalog();return;}
    if(next==='skills'){renderSkills();return;}
    if(next==='history'){title('Мои диалоги','Только ваша история');if(!history.length)main.append(node('p','Начните диалог с агентом из каталога.'));for(const chat of [...history].sort((a,b)=>b.updated_at-a.updated_at)){const row=node('article',undefined,'agents-provider');row.append(node('strong',chat.title),action('Открыть',()=>run(()=>openChat(chat.id))));main.append(row);}return;}
    title('Настройки агентов','Модели и агенты только для текущего домена');
    if(!boot.can_manage){main.append(node('p','Настройки моделей доступны администратору домена.'));return;}
    const version=revision;config=await request('/config');if(version!==revision)return;
    renderSettings();
  }
  function renderCatalog(){
    const header=title('Все агенты','Выберите агента и начните личный диалог');
    if(boot.can_manage)header.append(action('+ Создать агента',()=>run(()=>editAgent()),'agents-primary'));
    const search=node('input');search.type='search';search.placeholder='Поиск агентов';search.setAttribute('aria-label','Поиск агентов');search.className='agents-search';main.append(search);
    const grid=node('div',undefined,'agents-catalog');main.append(grid);
    const fill=()=>{grid.replaceChildren();const term=search.value.toLocaleLowerCase();const list=boot.catalog.agents.filter(a=>(a.name+' '+a.description).toLocaleLowerCase().includes(term));
      if(!list.length)grid.append(node('p',boot.catalog.agents.length?'Агенты не найдены':'В этом домене пока нет агентов. Администратор может добавить модель и создать первого агента.'));
      for(const agent of list){const card=node('article',undefined,'agents-card');card.append(node('div','✧','agents-avatar'),node('h2',agent.name),node('p',agent.description||'Помощник вашего домена'),node('small','Личный диалог · без доступа к подключениям'),action('Начать диалог',()=>run(()=>newChat(agent.id)),'agents-primary'));if(boot.can_manage)card.append(action('Настроить',()=>run(()=>editAgent(agent.id))));grid.append(card);}
    };search.oninput=fill;fill();
  }
  function renderSkills(){
    const header=title('Навыки','Инструкции, которые администратор подключает к агентам');
    if(boot.can_manage)header.append(action('+ Добавить навык',()=>run(()=>editSkill()),'agents-primary'));
    const grid=node('div',undefined,'agents-catalog');main.append(grid);
    if(!boot.catalog.skills.length)grid.append(node('p','Навыки ещё не добавлены.'));
    for(const skill of boot.catalog.skills){const card=node('article',undefined,'agents-card');card.append(node('h2',skill.name));if(boot.can_manage)card.append(action('Изменить',()=>run(()=>editSkill(skill.id))));grid.append(card);}
  }
  async function newChat(id){const check=guard();const chat=await request('/conversations',{method:'POST',body:JSON.stringify({agent_id:id})});check();history.push(chat);await openChat(chat.id);}
  async function openChat(id){
    const version=++revision;const chat=await request('/conversations/'+encodeURIComponent(id));if(version!==revision)return;
    active=id;view='chat';main.replaceChildren();renderSidebar();
    const agent=boot.catalog.agents.find(a=>a.id===chat.agent_id);const header=title(agent?.name||'Агент','Личный диалог');
    header.append(action('Новый диалог',()=>run(()=>newChat(chat.agent_id))),action('Удалить',()=>run(async()=>{if(!window.confirm('Удалить этот диалог?'))return;await request('/conversations/'+encodeURIComponent(id),{method:'DELETE'});history=history.filter(c=>c.id!==id);await show('catalog');})));
    const messages=node('div',undefined,'agents-messages');messages.setAttribute('role','log');messages.setAttribute('aria-label','Сообщения диалога');main.append(messages);
    const draw=()=>{messages.replaceChildren();if(!chat.messages.length)messages.append(node('div','Чем могу помочь?','agents-welcome'));for(const m of chat.messages){const article=node('article',undefined,'agents-message '+m.role);article.append(node('strong',m.role==='user'?'Вы':agent?.name||'Агент'),node('div',m.content));messages.append(article);}messages.scrollTop=messages.scrollHeight;};draw();
    const form=node('form',undefined,'agents-composer'),input=node('textarea');input.rows=3;input.maxLength=16000;input.placeholder='Напишите сообщение…';input.setAttribute('aria-label','Сообщение агенту');
    const send=node('button','Отправить','agents-primary');send.type='submit';const status=node('span','', 'agents-send-status');status.setAttribute('role','status');form.append(input,send,status);main.append(form,node('small','Ответы ИИ могут содержать ошибки. Сообщения передаются настроенному администратором провайдеру.','agents-notice'));
    form.onsubmit=event=>{event.preventDefault();if(sending||!input.value.trim())return;const content=input.value;run(async()=>{
      sending=true;send.disabled=true;input.disabled=true;status.textContent='Агент готовит ответ…';const check=guard();
      try{const updated=await request('/conversations/'+encodeURIComponent(id)+'/messages',{method:'POST',body:JSON.stringify({content})});check();const i=history.findIndex(c=>c.id===id);if(i>=0)history[i]=updated;if(active===id&&version===revision){chat.messages=updated.messages;input.value='';draw();renderSidebar();}}
      finally{check();sending=false;send.disabled=false;input.disabled=false;status.textContent='';}
    });};
    input.onkeydown=event=>{if(event.key==='Enter'&&(event.ctrlKey||event.metaKey)){event.preventDefault();form.requestSubmit();}};
  }
  function modal(titleText){
    const dialog=node('dialog');dialog.dataset.agentsDialog='';dialog.className='agents-dialog';const form=node('form');form.append(node('h2',titleText));dialog.append(form);section.append(dialog);dialog.addEventListener('close',()=>dialog.remove());dialog.showModal();return {dialog,form};
  }
  function field(form,label,value='',type='text'){
    const wrap=node('label',label),input=node(type==='textarea'?'textarea':'input');if(type!=='textarea')input.type=type;else input.rows=5;input.value=value;wrap.append(input);form.append(wrap);return input;
  }
  function select(form,label,items,value){const wrap=node('label',label),input=node('select');for(const [id,text]of items)input.add(new Option(text,id));input.value=value||items[0]?.[0]||'';wrap.append(input);form.append(wrap);return input;}
  function finishEditor(dialog,form,save,remove){const controls=node('div',undefined,'agents-actions');controls.append(action('Отмена',()=>dialog.close()));if(remove)controls.append(action('Удалить',()=>run(async()=>{await remove();dialog.close();await reload();})));const submit=node('button','Сохранить','agents-primary');submit.type='submit';controls.append(submit);form.append(controls);const localError=node('p','','agents-error');localError.setAttribute('role','alert');form.append(localError);form.onsubmit=async event=>{event.preventDefault();submit.disabled=true;try{await save();dialog.close();await reload();}catch(err){localError.textContent=err.data?.message||err.message;}finally{submit.disabled=false;}};}
  async function adminConfig(){if(!boot.can_manage)throw Error('Нужны права администратора домена');return request('/config');}
  async function saveConfig(next){await request('/config',{method:'PUT',body:JSON.stringify(next)});}
  async function reload(){const check=guard();boot=await request('');check();renderSidebar();await show(view==='chat'?'catalog':view);}
  async function editAgent(id){
    const next=await adminConfig();if(!next.providers.length){await show('settings');error.textContent='Сначала добавьте подключение к модели';return;}
    const original=next.agents.find(a=>a.id===id),a=original||{id:crypto.randomUUID(),name:'',description:'',instructions:'',provider_id:next.providers[0].id,enabled:true,skill_ids:[]};
    const {dialog,form}=modal(original?'Настройки агента':'Новый агент');
    const name=field(form,'Имя',a.name);name.required=true;name.maxLength=100;const description=field(form,'Описание',a.description);description.maxLength=1000;
    const provider=select(form,'Модель',next.providers.map(p=>[p.id,p.name+' · '+p.model]),a.provider_id),instructions=field(form,'Инструкции агента',a.instructions,'textarea');instructions.maxLength=32000;
    const enabled=field(form,'Доступен пользователям домена','','checkbox');enabled.checked=a.enabled;
    const skills=[];for(const skill of next.skills){const checkbox=field(form,skill.name,'','checkbox');checkbox.checked=a.skill_ids?.includes(skill.id);skills.push([skill.id,checkbox]);}
    finishEditor(dialog,form,async()=>{const updated={...a,name:name.value,description:description.value,provider_id:provider.value,instructions:instructions.value,enabled:enabled.checked,skill_ids:skills.filter(([,el])=>el.checked).map(([key])=>key)};next.agents=next.agents.filter(v=>v.id!==a.id);next.agents.push(updated);await saveConfig(next);},original?async()=>{next.agents=next.agents.filter(v=>v.id!==a.id);await saveConfig(next);}:null);
  }
  async function editSkill(id){const next=await adminConfig(),original=next.skills.find(s=>s.id===id),skill=original||{id:crypto.randomUUID(),name:'',instructions:''};const {dialog,form}=modal('Навык');const name=field(form,'Название',skill.name),instructions=field(form,'Инструкции',skill.instructions,'textarea');name.required=true;name.maxLength=100;instructions.maxLength=16000;
    finishEditor(dialog,form,async()=>{next.skills=next.skills.filter(s=>s.id!==skill.id);next.skills.push({...skill,name:name.value,instructions:instructions.value});await saveConfig(next);},original?async()=>{next.skills=next.skills.filter(s=>s.id!==skill.id);next.agents.forEach(a=>{a.skill_ids=a.skill_ids.filter(k=>k!==skill.id);});await saveConfig(next);}:null);
  }
  function renderSettings(){const header=main.querySelector('header');header.append(action('+ Подключить модель',()=>run(()=>editProvider()),'agents-primary'));main.append(node('p','Совместимые с OpenAI API провайдеры. Эти подключения используются только агентами текущего домена.'));
    for(const p of config.providers){const card=node('article',undefined,'agents-provider');card.append(node('h2',p.name),node('p',p.model+' · '+p.url),node('small',p.has_key?'Ключ настроен':'Без API-ключа'),action('Изменить',()=>run(()=>editProvider(p.id))));main.append(card);}
    const header2=node('div',undefined,'agents-actions');header2.append(node('h2','Агенты'),action('+ Создать',()=>run(()=>editAgent()),'agents-primary'));main.append(header2);
    for(const a of config.agents){const row=node('article',undefined,'agents-provider');row.append(node('strong',a.name+(a.enabled?'':' · отключён')),action('Настроить',()=>run(()=>editAgent(a.id))));main.append(row);}
  }
  async function editProvider(id){
    const next=await adminConfig(),original=next.providers.find(p=>p.id===id),p=original||{id:crypto.randomUUID(),kind:'deepseek',thinking:false,name:'DeepSeek',url:'https://api.deepseek.com',model:'deepseek-flash',api_key:''};const {dialog,form}=modal('Подключение к модели');
    const kind=select(form,'Провайдер',[['deepseek','DeepSeek'],['openai-compatible','Другой OpenAI-совместимый API']],p.kind||'openai-compatible');
    const name=field(form,'Название',p.name),url=field(form,'Базовый URL API',p.url,'url'),model=field(form,'Идентификатор модели',p.model),key=field(form,p.has_key?'Новый API-ключ (пусто — сохранить прежний)':'API-ключ', '', 'password');name.required=url.required=model.required=true;key.autocomplete='new-password';name.maxLength=100;model.maxLength=200;
    const suggestions=node('datalist');suggestions.id='agents-model-suggestions';for(const id of ['deepseek-flash','deepseek-v4-pro']){const option=node('option');option.value=id;suggestions.append(option);}form.append(suggestions);
    const thinking=field(form,'DeepSeek: режим рассуждений','','checkbox');thinking.checked=!!p.thinking;
    const hint=node('small','Для DeepSeek используйте API-ключ из platform.deepseek.com. Он хранится только на сервере для текущего домена.');form.append(hint);
    const sync=()=>{const deepseek=kind.value==='deepseek';thinking.parentElement.hidden=!deepseek;hint.hidden=!deepseek;if(deepseek)model.setAttribute('list',suggestions.id);else model.removeAttribute('list');key.required=deepseek&&!(p.has_key&&p.kind===kind.value&&p.url===url.value);};
    kind.onchange=()=>{if(kind.value==='deepseek'){name.value='DeepSeek';url.value='https://api.deepseek.com';model.value='deepseek-flash';}else{if(name.value==='DeepSeek')name.value='';if(['https://api.deepseek.com','https://api.deepseek.com/v1'].includes(url.value.replace(/\/$/,'')))url.value='';if(model.value.startsWith('deepseek-'))model.value='';}sync();};url.oninput=sync;sync();
    finishEditor(dialog,form,async()=>{next.providers=next.providers.filter(v=>v.id!==p.id);next.providers.push({id:p.id,kind:kind.value,thinking:kind.value==='deepseek'&&thinking.checked,name:name.value,url:url.value,model:model.value,api_key:key.value});await saveConfig(next);},original?async()=>{if(next.agents.some(a=>a.provider_id===p.id))throw Error('Сначала переключите или удалите агентов этой модели');next.providers=next.providers.filter(v=>v.id!==p.id);await saveConfig(next);}:null);
  }
  section.querySelectorAll('[data-view]').forEach(b=>{b.onclick=()=>run(()=>show(b.dataset.view));});
  const previous=showApp;showApp=function(name){previous(name);if(name==='agents'){const title=document.getElementById('app-topbar-title');if(title){title.removeAttribute('data-i18n');title.textContent='Агенты';}run(load);}};button.onclick=()=>showApp('agents');
  window.addEventListener('tms-products-session-ended',reset);window.addEventListener('storage',event=>{if(event.key==='tayga.tokens'){reset();if(!section.classList.contains('hidden'))run(load);}});
})();
