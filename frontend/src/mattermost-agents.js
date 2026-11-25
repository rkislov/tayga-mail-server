/* Native TMS Agents product. Tenant, domain and user are bound by the server. */
(function installAgents() {
  const workspace=window.tmsChatWorkspace;if(!workspace)return;
  const paths={spark:'M12 3l2.2 6.8L21 12l-6.8 2.2L12 21l-2.2-6.8L3 12l6.8-2.2L12 3z',catalog:'M10.5 18a7.5 7.5 0 1 1 0-15 7.5 7.5 0 0 1 0 15 M16 16l5 5',history:'M4 5h16v12H8l-4 4V5z',skills:'M12 3l-8 4 8 4 8-4-8-4z M4 12l8 4 8-4 M4 17l8 4 8-4',settings:'M5 4v16 M12 4v16 M19 4v16 M2 8h6 M9 15h6 M16 10h6',close:'M6 6l12 12 M18 6L6 18',plus:'M12 5v14 M5 12h14',arrow:'M5 12h14 M13 6l6 6-6 6',shield:'M12 3l8 3v6c0 5-8 9-8 9s-8-4-8-9V6l8-3z M8 12l3 3 5-6'};
  function icon(name){const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('fill','none');svg.setAttribute('stroke','currentColor');svg.setAttribute('stroke-width','1.6');svg.setAttribute('stroke-linecap','round');svg.setAttribute('stroke-linejoin','round');svg.setAttribute('aria-hidden','true');const path=document.createElementNS(svg.namespaceURI,'path');path.setAttribute('d',paths[name]||paths.spark);svg.append(path);return svg;}
  const section=document.createElement('section');section.id='app-agents';section.className='app-page hidden';
  section.innerHTML=`<div class="tms-agents"><aside id="agents-sidebar" class="agents-sidebar"><div class="agents-brand"><span class="agents-brand-icon"></span><div><strong>Агенты</strong><span>Помощники для работы</span></div></div><nav class="agents-navigation" aria-label="Разделы агентов"><button type="button" data-view="catalog">Все агенты</button><button type="button" data-view="history">Мои диалоги</button><button type="button" data-view="skills">Навыки</button><button type="button" data-view="settings">Настройки</button></nav><div class="agents-side-history"></div><div class="agents-account"><span class="agents-account-icon"></span><div><small>Рабочее пространство</small><p class="agents-scope"></p></div></div></aside><div class="agents-divider" role="separator" tabindex="0" aria-orientation="vertical" aria-label="Ширина панели диалогов" aria-controls="agents-sidebar"></div><div class="agents-workspace"><div class="agents-error" role="alert"></div><main class="agents-main"></main></div></div>`;
  const layout=section.querySelector('.tms-agents');
  const divider=section.querySelector('.agents-divider');
  const leftPanel=section.querySelector('.agents-sidebar');
  const widthPreference='tayga.agentsSidebarWidth';
  let preferredWidth=Number(localStorage.getItem(widthPreference))||240;
  function resizePanels(remember=false) {
    if(!layout.clientWidth)return;
    const minimum=180;
    const maximum=Math.max(minimum,Math.min(560,layout.clientWidth-340));
    const width=Math.round(Math.max(minimum,Math.min(maximum,preferredWidth)));
    layout.style.setProperty('--agents-sidebar-width',width+'px');
    divider.setAttribute('aria-valuemin',minimum);
    divider.setAttribute('aria-valuemax',maximum);
    divider.setAttribute('aria-valuenow',width);
    if(remember){preferredWidth=width;localStorage.setItem(widthPreference,String(width));}
  }
  divider.addEventListener('pointerdown',event=>{
    if(event.button!==0)return;
    event.preventDefault();
    divider.focus();
    divider.setPointerCapture(event.pointerId);
  });
  divider.addEventListener('pointermove',event=>{
    if(!divider.hasPointerCapture(event.pointerId))return;
    preferredWidth=event.clientX-layout.getBoundingClientRect().left;
    resizePanels(true);
  });
  divider.addEventListener('keydown',event=>{
    const current=leftPanel.getBoundingClientRect().width;
    const widths={ArrowLeft:current-16,ArrowRight:current+16,Home:180,End:560};
    if(!(event.key in widths))return;
    event.preventDefault();
    preferredWidth=widths[event.key];
    resizePanels(true);
  });
  if(typeof ResizeObserver!=='undefined')new ResizeObserver(()=>resizePanels()).observe(layout);
  section.querySelector('.agents-brand-icon').append(icon('spark'));section.querySelector('.agents-account-icon').append(icon('shield'));
  section.querySelectorAll('[data-view]').forEach(b=>b.prepend(icon(b.dataset.view)));

  workspace.register('agents',{label:'Агенты',section});
  const main=section.querySelector('.agents-main'),error=section.querySelector('.agents-error'),sidebar=section.querySelector('.agents-side-history');
  let pollTimer=null;
  let owner='',ownerToken='',epoch=0,revision=0,loadRevision=0,boot=null,config=null,history=[],active='',view='catalog',sending=false,controller=new AbortController();
  const node=(tag,text,className)=>{const el=document.createElement(tag);if(text!==undefined)el.textContent=text;if(className)el.className=className;return el;};
  const action=(text,run,className)=>{const el=node('button',text,className);el.type='button';el.onclick=run;return el;};
  function guard(){const e=epoch,email=owner,token=ownerToken;return()=>{if(e!==epoch||email!==state.email||!token||token!==state.tokens?.access_token)throw Error('Учётная запись изменилась');};}
  async function request(path,options={}){const check=guard();try{check();const out=await api('/api/v4/tms/agents'+path,{...options,signal:controller.signal,_guard:check});check();return out;}catch(err){if(ownerToken!==state.tokens?.access_token){reset();if(state.tokens?.access_token&&!section.classList.contains('hidden'))queueMicrotask(()=>run(load));}throw err;}}
  function report(err){if(err.name!=='AbortError')error.textContent=err.data?.message||err.message;}
  async function run(fn){const version=epoch;error.textContent='';try{await fn();}catch(err){if(version===epoch)report(err);}}
  function reset(){clearInterval(pollTimer);epoch++;revision++;loadRevision++;controller.abort();controller=new AbortController();owner='';ownerToken='';boot=null;config=null;history=[];active='';sending=false;sidebar.replaceChildren();main.replaceChildren();error.textContent='';section.querySelector('.agents-scope').textContent='';document.querySelectorAll('dialog[data-agents-dialog]').forEach(el=>el.remove());}
  async function load(){
    if(!state.tokens?.access_token){reset();return;}
    if(owner!==state.email||ownerToken!==state.tokens.access_token){reset();owner=state.email;ownerToken=state.tokens.access_token;}
    const check=guard(),version=++loadRevision;const nextBoot=await request('');check();
    const nextHistory=await request('/conversations');check();if(version!==loadRevision)return;boot=nextBoot;history=nextHistory;
    section.querySelector('.agents-scope').textContent=state.email||'Мой домен';renderSidebar();if(active)await openChat(active);else await show(view==='chat'?'catalog':view);
  }
  function renderSidebar(){
    section.querySelectorAll('[data-view]').forEach(b=>{const selected=b.dataset.view===view;b.classList.toggle('is-selected',selected);if(selected)b.setAttribute('aria-current','page');else b.removeAttribute('aria-current');});
    sidebar.replaceChildren();
    if(!boot.catalog.agents.length)return;
    sidebar.append(node('p','Личные диалоги','agents-sidebar-label'));
    for(const agent of boot.catalog.agents){
      const heading=node('div',undefined,'agents-history-heading');heading.append(node('span',agent.name),action('+',()=>run(()=>newChat(agent.id))));heading.lastChild.setAttribute('aria-label','Новый диалог: '+agent.name);heading.lastChild.title='Новый диалог';sidebar.append(heading);
      for(const chat of history.filter(c=>c.agent_id===agent.id).sort((a,b)=>b.updated_at-a.updated_at)){
        const b=action(chat.title,()=>run(()=>openChat(chat.id)),'agents-history-link');b.classList.toggle('is-selected',active===chat.id);b.title=chat.title;sidebar.append(b);
      }
    }
  }
  function title(text,subtitle){const header=node('header',undefined,'agents-header');const block=node('div');block.append(node('h1',text));if(subtitle)block.append(node('p',subtitle));header.append(block);main.append(header);return header;}
  async function show(next){clearInterval(pollTimer);
    view=next;revision++;active='';main.classList.remove('is-conversation');main.replaceChildren();renderSidebar();
    if(!boot)return;
    if(next==='catalog'){renderCatalog();return;}
    if(next==='skills'){renderSkills();return;}
    if(next==='history'){title('Мои диалоги','Только ваша история');if(!history.length)main.append(emptyState('history','Ваша история начинается здесь','Выберите помощника в каталоге и задайте первый вопрос.',[action('Выбрать агента',()=>run(()=>show('catalog')),'agents-primary')]));for(const chat of [...history].sort((a,b)=>b.updated_at-a.updated_at)){const row=node('article',undefined,'agents-provider');row.append(node('strong',chat.title),action('Открыть',()=>run(()=>openChat(chat.id))));main.append(row);}return;}
    title('Настройки агентов','Модели и агенты только для текущего домена');
    if(!boot.can_manage){main.append(emptyState('settings','Всё настроит администратор','Настройки моделей доступны администратору домена. Вы можете выбрать доступного помощника в каталоге.'));return;}
    const version=revision;config=await request('/config');if(version!==revision)return;
    renderSettings();
  }
  function emptyState(symbol,heading,description,buttons=[]){
    const wrap=node('div',undefined,'agents-empty'),visual=node('div',undefined,'agents-empty-visual');visual.append(icon(symbol));wrap.append(visual,node('h2',heading),node('p',description));
    if(buttons.length){const controls=node('div',undefined,'agents-empty-actions');controls.append(...buttons);wrap.append(controls);}
    return wrap;
  }
  function renderCatalog(){
    const header=title('Все агенты','Помощники вашего рабочего пространства');
    if(boot.can_manage)header.append(action('+ Создать агента',()=>run(()=>editAgent()),'agents-primary'));
    if(!boot.catalog.agents.length){
      main.append(emptyState('spark','С чего начнём?',boot.can_manage?'Подключите DeepSeek, Ollama или OpenAI и создайте помощника для вашей команды.':'Здесь появятся помощники, настроенные администратором для вашего домена.',boot.can_manage?[action('Подключить модель',()=>run(()=>editProvider()),'agents-primary'),action('Создать первого агента',()=>run(()=>editAgent()))]:[]));
      if(boot.can_manage){const steps=node('div',undefined,'agents-setup-steps');for(const [number,heading,text]of [['1','Подключите модель','DeepSeek, Ollama или OpenAI'],['2','Настройте помощника','Имя, инструкции и навыки'],['3','Начните диалог','Личная история для каждого']]){const step=node('div');step.append(node('span',number),node('strong',heading),node('small',text));steps.append(step);}main.append(steps);}
      return;
    }
    const toolbar=node('div',undefined,'agents-toolbar'),searchWrap=node('div',undefined,'agents-search-wrap'),search=node('input');search.type='search';search.placeholder='Поиск агентов';search.setAttribute('aria-label','Поиск агентов');search.className='agents-search';searchWrap.append(icon('catalog'),search);toolbar.append(searchWrap,node('span','Агентов: '+boot.catalog.agents.length,'agents-count'));main.append(toolbar);
    const grid=node('div',undefined,'agents-catalog');main.append(grid);
    const fill=()=>{grid.replaceChildren();const term=search.value.toLocaleLowerCase();const list=boot.catalog.agents.filter(a=>(a.name+' '+a.description).toLocaleLowerCase().includes(term));
      if(!list.length){grid.append(emptyState('catalog','Ничего не найдено','Попробуйте другое имя или описание.'));return;}
      for(const agent of list){const card=node('article',undefined,'agents-card'),avatar=node('div',undefined,'agents-avatar');avatar.append(icon('spark'));const top=node('div',undefined,'agents-card-top');top.append(avatar,node('span','Агент','agents-badge'));card.append(top,node('h2',agent.name),node('p',agent.description||'Помощник для ваших рабочих задач'));const controls=node('div',undefined,'agents-card-actions');controls.append(action('Начать диалог',()=>run(()=>newChat(agent.id)),'agents-primary'));if(boot.can_manage)controls.append(action('Настроить',()=>run(()=>editAgent(agent.id))));card.append(controls);grid.append(card);}
    };search.oninput=fill;fill();
  }
  function renderSkills(){
    const header=title('Навыки','Дополнительные инструкции для ваших помощников');
    if(boot.can_manage)header.append(action('+ Добавить навык',()=>run(()=>editSkill()),'agents-primary'));
    if(!boot.catalog.skills.length){main.append(emptyState('skills','Помогите агенту работать точнее','Добавьте инструкции: например, отвечать кратко, писать деловым языком или проверять структуру документа.',boot.can_manage?[action('Добавить навык',()=>run(()=>editSkill()),'agents-primary')]:[]));return;}
    const grid=node('div',undefined,'agents-catalog');main.append(grid);
    for(const skill of boot.catalog.skills){const card=node('article',undefined,'agents-card'),avatar=node('div',undefined,'agents-avatar');avatar.append(icon('skills'));card.append(avatar,node('h2',skill.name),node('p','Дополнительная инструкция для агента'));if(boot.can_manage)card.append(action('Изменить',()=>run(()=>editSkill(skill.id))));grid.append(card);}
  }
  async function newChat(id){const check=guard();const chat=await request('/conversations',{method:'POST',body:JSON.stringify({agent_id:id})});check();history.push(chat);await openChat(chat.id);}
  async function openChat(id){clearInterval(pollTimer);
    const version=++revision;const chat=await request('/conversations/'+encodeURIComponent(id));if(version!==revision)return;
    const historyEntry=history.find(entry=>entry.id===id);if(historyEntry)historyEntry.title=chat.title;
    active=id;view='chat';main.replaceChildren();main.classList.add('is-conversation');renderSidebar();
    const agent=boot.catalog.agents.find(a=>a.id===chat.agent_id);const header=title(agent?.name||'Агент','Личный диалог');
    header.append(action('Новый диалог',()=>run(()=>newChat(chat.agent_id))),action('Убрать из списка',()=>run(async()=>{if(!window.confirm('Убрать диалог из списка? История ветки останется в Mattermost.'))return;await request('/conversations/'+encodeURIComponent(id),{method:'DELETE'});history=history.filter(c=>c.id!==id);await show('catalog');})));
    const web=action(chat.web_enabled?'Веб-поиск включён':'Веб-поиск выключен',()=>run(async()=>{const updated=await request('/conversations/'+encodeURIComponent(id),{method:'PATCH',body:JSON.stringify({web_enabled:!chat.web_enabled})});chat.web_enabled=updated.web_enabled;web.textContent=chat.web_enabled?'Веб-поиск включён':'Веб-поиск выключен';}));web.setAttribute('aria-pressed',String(!!chat.web_enabled));if(agent?.tools?.includes('web_search')){if(boot.search_available===false){web.disabled=true;web.textContent='Поиск не настроен';}header.append(web);}
    const approvals=node('div',undefined,'agents-approvals');main.append(approvals);
    const drawActions=()=>{approvals.replaceChildren();for(const item of chat.actions||[]){if(item.state!=='pending'||item.expires_at<Date.now())continue;const card=node('article',undefined,'agents-card');card.append(node('strong',item.tool==='mail_draft'?'Подтвердить отправку письма':'Подтвердить отправку сообщения'),node('p',item.arguments.to||item.arguments.channel_id),node('p',item.arguments.subject||''),node('pre',item.arguments.text));card.append(action('Отправить',()=>run(async()=>{await request('/conversations/'+encodeURIComponent(id)+'/actions/'+encodeURIComponent(item.id),{method:'POST',body:'{}'});item.state='sent';drawActions();}),'agents-primary'));approvals.append(card);}};drawActions();
    const messages=node('div',undefined,'agents-messages');messages.setAttribute('role','log');messages.setAttribute('aria-label','Сообщения диалога');main.append(messages);
    const draw=()=>{messages.replaceChildren();if(!chat.messages.length)messages.append(emptyState('spark','Чем могу помочь?',agent?.description||'Задайте вопрос или опишите задачу. История этого диалога доступна только вам.'));for(const m of chat.messages){const article=node('article',undefined,'agents-message '+m.role);article.append(node('strong',m.role==='user'?'Вы':agent?.name||'Агент'),node('div',m.content));messages.append(article);}messages.scrollTop=messages.scrollHeight;};draw();
    const form=node('form',undefined,'agents-composer'),input=node('textarea');input.rows=3;input.maxLength=16000;input.placeholder='Напишите сообщение…';input.setAttribute('aria-label','Сообщение агенту');
    const send=node('button','Отправить','agents-primary');send.type='submit';const status=node('span','', 'agents-send-status');status.setAttribute('role','status');form.append(input,send,status);main.append(form,node('small','Ответы ИИ могут содержать ошибки. Сообщения передаются настроенному администратором провайдеру.','agents-notice'));
    form.onsubmit=event=>{event.preventDefault();if(sending||!input.value.trim())return;const content=input.value;run(async()=>{
      sending=true;send.disabled=true;input.disabled=true;status.textContent='Агент готовит ответ…';const check=guard();
      try{const updated=await request('/conversations/'+encodeURIComponent(id)+'/messages',{method:'POST',body:JSON.stringify({content})});check();const i=history.findIndex(c=>c.id===id);if(i>=0)history[i]=updated;if(active===id&&version===revision){chat.messages=updated.messages;chat.actions=updated.actions;drawActions();input.value='';draw();renderSidebar();}}
      catch(err){
        if(active===id&&version===revision){
          const latest=await request('/conversations/'+encodeURIComponent(id)).catch(()=>null);
          if(latest&&active===id&&version===revision){
            chat.messages=latest.messages;
            if(historyEntry)historyEntry.title=latest.title;
            draw();
            renderSidebar();
          }
        }
        throw err;
      }
      finally{check();sending=false;send.disabled=false;input.disabled=false;status.textContent='';}
    });};
    pollTimer=setInterval(async()=>{if(sending||section.classList.contains('hidden')||active!==id||version!==revision)return;try{const updated=await request('/conversations/'+encodeURIComponent(id));if(active!==id||version!==revision)return;if(historyEntry&&historyEntry.title!==updated.title){historyEntry.title=updated.title;renderSidebar();}if(JSON.stringify(chat.messages)!==JSON.stringify(updated.messages)){chat.messages=updated.messages;draw();}chat.actions=updated.actions;drawActions();}catch{ /* Keep the draft when temporarily offline. */ }},4000);
    input.onkeydown=event=>{if(event.key==='Enter'&&(event.ctrlKey||event.metaKey)){event.preventDefault();form.requestSubmit();}};
  }
  function modal(titleText){
    const dialog=node('dialog');dialog.dataset.agentsDialog='';dialog.className='agents-dialog';const form=node('form'),header=node('div',undefined,'agents-dialog-header'),close=action('',()=>dialog.close(),'agents-dialog-close');close.append(icon('close'));close.setAttribute('aria-label','Закрыть');header.append(node('h2',titleText),close);form.append(header);dialog.append(form);section.append(dialog);dialog.addEventListener('close',()=>dialog.remove());dialog.showModal();return {dialog,form};
  }
  function field(form,label,value='',type='text'){
    const wrap=node('label',label),input=node(type==='textarea'?'textarea':'input');if(type!=='textarea')input.type=type;else input.rows=5;input.value=value;if(type==='checkbox')wrap.className='agents-checkbox';wrap.append(input);form.append(wrap);return input;
  }
  function select(form,label,items,value){const wrap=node('label',label),input=node('select');input.setAttribute('aria-label',label);for(const [id,text]of items)input.add(new Option(text,id));input.value=value||items[0]?.[0]||'';wrap.append(input);form.append(wrap);return input;}
  function finishEditor(dialog,form,save,remove){const controls=node('div',undefined,'agents-actions');controls.append(action('Отмена',()=>dialog.close()));if(remove)controls.append(action('Удалить',()=>run(async()=>{await remove();dialog.close();await reload();})));const submit=node('button','Сохранить','agents-primary');submit.type='submit';controls.append(submit);form.append(controls);const localError=node('p','','agents-error');localError.setAttribute('role','alert');form.append(localError);form.onsubmit=async event=>{event.preventDefault();submit.disabled=true;try{await save();dialog.close();await reload();}catch(err){localError.textContent=err.data?.message||err.message;}finally{submit.disabled=false;}};}
  async function adminConfig(){if(!boot.can_manage)throw Error('Нужны права администратора домена');return request('/config');}
  async function saveConfig(next){await request('/config',{method:'PUT',body:JSON.stringify(next)});}
  async function reload(){const check=guard();boot=await request('');check();renderSidebar();await show(view==='chat'?'catalog':view);}
  async function editAgent(id){
    const next=await adminConfig();if(!next.providers.length){await editProvider();return;}
    const original=next.agents.find(a=>a.id===id),a=original||{id:crypto.randomUUID(),name:'',description:'',instructions:'',provider_id:next.providers[0].id,enabled:true,skill_ids:[]};
    const {dialog,form}=modal(original?'Настройки агента':'Новый агент');
    const name=field(form,'Имя',a.name);name.required=true;name.maxLength=100;const description=field(form,'Описание',a.description);description.maxLength=1000;
    const provider=select(form,'Модель',next.providers.map(p=>[p.id,p.name+' · '+p.model]),a.provider_id),instructions=field(form,'Инструкции агента',a.instructions,'textarea');instructions.maxLength=32000;
    const enabled=field(form,'Доступен пользователям домена','','checkbox');enabled.checked=a.enabled;
    const tools=[];for(const [id,label] of [['web_search','Поиск в интернете'],['mail_read','Поиск и чтение моей почты'],['mail_send','Подготовка писем с подтверждением'],['chat_read','Чтение доступных мне каналов'],['chat_send','Подготовка сообщений с подтверждением']]){const checkbox=field(form,label,'','checkbox');checkbox.checked=a.tools?.includes(id);tools.push([id,checkbox]);}
    const skills=[];for(const skill of next.skills){const checkbox=field(form,skill.name,'','checkbox');checkbox.checked=a.skill_ids?.includes(skill.id);skills.push([skill.id,checkbox]);}
    finishEditor(dialog,form,async()=>{const updated={...a,name:name.value,description:description.value,provider_id:provider.value,instructions:instructions.value,enabled:enabled.checked,tools:tools.filter(([,el])=>el.checked).map(([key])=>key),skill_ids:skills.filter(([,el])=>el.checked).map(([key])=>key)};next.agents=next.agents.filter(v=>v.id!==a.id);next.agents.push(updated);await saveConfig(next);},original?async()=>{next.agents=next.agents.filter(v=>v.id!==a.id);await saveConfig(next);}:null);
  }
  async function editSkill(id){const next=await adminConfig(),original=next.skills.find(s=>s.id===id),skill=original||{id:crypto.randomUUID(),name:'',instructions:''};const {dialog,form}=modal('Навык');const name=field(form,'Название',skill.name),instructions=field(form,'Инструкции',skill.instructions,'textarea');name.required=true;name.maxLength=100;instructions.maxLength=16000;
    finishEditor(dialog,form,async()=>{next.skills=next.skills.filter(s=>s.id!==skill.id);next.skills.push({...skill,name:name.value,instructions:instructions.value});await saveConfig(next);},original?async()=>{next.skills=next.skills.filter(s=>s.id!==skill.id);next.agents.forEach(a=>{a.skill_ids=a.skill_ids.filter(k=>k!==skill.id);});await saveConfig(next);}:null);
  }
  function renderSettings(){const header=main.querySelector('header');header.append(action('Веб-поиск',()=>run(editSearch)));header.append(action('+ Подключить модель',()=>run(()=>editProvider()),'agents-primary'));main.append(node('p','Подключения доступны агентам только в вашем домене.','agents-settings-note'));if(!config.providers.length)main.append(emptyState('settings','Подключите первую модель','Выберите DeepSeek, Ollama, OpenAI или совместимый провайдер.',[action('Подключить модель',()=>run(()=>editProvider()),'agents-primary')]));
    for(const p of config.providers){const card=node('article',undefined,'agents-provider');const mark=node('div',undefined,'agents-avatar');mark.append(icon('settings'));const details=node('div',undefined,'agents-provider-details');details.append(node('h2',p.name),node('p',p.model),node('small',p.url));card.append(mark,details,node('span',p.has_key?'Ключ настроен':'Без API-ключа','agents-badge'),action('Изменить',()=>run(()=>editProvider(p.id))));main.append(card);}
    const header2=node('div',undefined,'agents-actions');header2.append(node('h2','Агенты'),action('+ Создать',()=>run(()=>editAgent()),'agents-primary'));main.append(header2);
    for(const a of config.agents){const row=node('article',undefined,'agents-provider');row.append(node('strong',a.name+(a.enabled?'':' · отключён')),action('Настроить',()=>run(()=>editAgent(a.id))));main.append(row);}
  }
  async function editSearch(){const next=await adminConfig(),current=next.search||{}, {dialog,form}=modal('Веб-поиск домена');const kind=select(form,'Сервис',[['','Выключен'],['brave','Brave Search API'],['searxng','SearXNG']],current.kind),url=field(form,'Адрес SearXNG',current.kind==='searxng'?current.url:''),key=field(form,current.has_key?'Новый ключ (пусто — сохранить)':'API-ключ','','password');form.append(node('p','Поисковые запросы передаются выбранному сервису. Для SearXNG включите формат JSON. Агент и пользователь отдельно разрешают веб-поиск.'));finishEditor(dialog,form,async()=>{next.search={kind:kind.value,url:kind.value==='searxng'?url.value:'',api_key:key.value};await saveConfig(next);});}
  async function editProvider(id){
    const next=await adminConfig(),original=next.providers.find(p=>p.id===id),p=original||{id:crypto.randomUUID(),kind:'deepseek',thinking:false,name:'DeepSeek',url:'https://api.deepseek.com',model:'deepseek-flash',api_key:''};const {dialog,form}=modal('Подключение к модели');
    const kind=select(form,'Провайдер',[['deepseek','DeepSeek'],['ollama','Ollama'],['openai','OpenAI (ChatGPT)'],['openai-compatible','Другой OpenAI-совместимый API']],p.kind||'openai-compatible');
    const name=field(form,'Название',p.name),url=field(form,'Базовый URL API',p.url,'url'),model=field(form,'Идентификатор модели',p.model),key=field(form,p.has_key?'Новый API-ключ (пусто — сохранить прежний)':'API-ключ', '', 'password');name.required=url.required=model.required=true;key.autocomplete='new-password';name.maxLength=100;model.maxLength=200;
    const suggestions=node('datalist');suggestions.id='agents-model-suggestions';form.append(suggestions);
    const thinking=field(form,'DeepSeek: режим рассуждений','','checkbox');thinking.checked=!!p.thinking;
    const hint=node('small');form.append(hint);
    const presets={deepseek:{name:'DeepSeek',url:'https://api.deepseek.com',model:'deepseek-flash',models:['deepseek-flash','deepseek-v4-pro'],hint:'API-ключ из platform.deepseek.com. Подключение доступно только вашему домену.'},ollama:{name:'Ollama',url:'http://127.0.0.1:11434',model:'',models:[],hint:'Адрес Ollama со стороны сервера TMS. Укажите установленную модель из ollama list. Ключ нужен только при защите API; HTTP разрешён для локального сервера и частных IP-адресов.'},openai:{name:'OpenAI',url:'https://api.openai.com/v1',model:'gpt-6.2-sol',models:['gpt-6.2-sol','gpt-6-luna'],hint:'Нужен ключ OpenAI API из platform.openai.com и доступ к выбранной модели. Ключ хранится на сервере в настройках вашего домена.'},'openai-compatible':{name:'',url:'',model:'',models:[],hint:'Базовый URL совместимого API, например https://model.example.com/v1. Запросы отправляет сервер TMS.'}};
    const sync=()=>{const selected=presets[kind.value],deepseek=kind.value==='deepseek';thinking.parentElement.hidden=!deepseek;hint.textContent=selected.hint;url.readOnly=['deepseek','openai'].includes(kind.value);model.placeholder=kind.value==='ollama'?'Имя установленной модели':'Идентификатор модели';suggestions.replaceChildren();for(const id of selected.models){const option=node('option');option.value=id;suggestions.append(option);}model.setAttribute('list',suggestions.id);
      const retained=p.has_key&&p.kind===kind.value&&p.url===url.value;key.required=['deepseek','openai'].includes(kind.value)&&!retained;key.parentElement.firstChild.textContent=retained?'Новый API-ключ (пусто — сохранить прежний)':kind.value==='ollama'?'API-ключ (если требуется)':'API-ключ';};
    kind.onchange=()=>{const selected=presets[kind.value];name.value=selected.name;url.value=selected.url;model.value=selected.model;key.value='';sync();};url.oninput=sync;sync();
    finishEditor(dialog,form,async()=>{next.providers=next.providers.filter(v=>v.id!==p.id);next.providers.push({id:p.id,kind:kind.value,thinking:kind.value==='deepseek'&&thinking.checked,name:name.value,url:url.value,model:model.value,api_key:key.value});await saveConfig(next);},original?async()=>{if(next.agents.some(a=>a.provider_id===p.id))throw Error('Сначала переключите или удалите агентов этой модели');next.providers=next.providers.filter(v=>v.id!==p.id);await saveConfig(next);}:null);
  }
  section.querySelectorAll('[data-view]').forEach(b=>{b.onclick=()=>run(()=>show(b.dataset.view));});
  const previous=showApp;showApp=function(name){previous(name);if(name==='agents'){run(load);}};
  window.addEventListener('tms-products-session-ended',reset);window.addEventListener('storage',event=>{if(event.key==='tayga.tokens'){reset();if(!section.classList.contains('hidden'))run(load);}});
})();
