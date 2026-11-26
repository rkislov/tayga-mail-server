/* Native TMS Agents product. Tenant, domain and user are bound by the server. */
(function installAgents() {
  const workspace=window.tmsChatWorkspace;if(!workspace)return;
  const paths={spark:'M12 3l2.2 6.8L21 12l-6.8 2.2L12 21l-2.2-6.8L3 12l6.8-2.2L12 3z',catalog:'M10.5 18a7.5 7.5 0 1 1 0-15 7.5 7.5 0 0 1 0 15 M16 16l5 5',history:'M4 5h16v12H8l-4 4V5z',skills:'M12 3l-8 4 8 4 8-4-8-4z M4 12l8 4 8-4 M4 17l8 4 8-4',settings:'M5 4v16 M12 4v16 M19 4v16 M2 8h6 M9 15h6 M16 10h6',close:'M6 6l12 12 M18 6L6 18',plus:'M12 5v14 M5 12h14',arrow:'M5 12h14 M13 6l6 6-6 6',shield:'M12 3l8 3v6c0 5-8 9-8 9s-8-4-8-9V6l8-3z M8 12l3 3 5-6'};
  function icon(name){const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('fill','none');svg.setAttribute('stroke','currentColor');svg.setAttribute('stroke-width','1.6');svg.setAttribute('stroke-linecap','round');svg.setAttribute('stroke-linejoin','round');svg.setAttribute('aria-hidden','true');const path=document.createElementNS(svg.namespaceURI,'path');path.setAttribute('d',paths[name]||paths.spark);svg.append(path);return svg;}
  Object.assign(paths,{
    schedule:'M4 5h16v16H4zM8 3v4m8-4v4M4 9h16m-8 3v4l3 1',
    web:'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0ZM3 12h18M12 3c-5 5-5 13 0 18 5-5 5-13 0-18',
    attach:'m20 11-8.4 8.4a5 5 0 0 1-7-7L13 4a3.5 3.5 0 0 1 5 5l-8 8a2 2 0 0 1-2.8-2.8l7-7',
    mic:'M9 5a3 3 0 1 1 6 0v6a3 3 0 0 1-6 0V5ZM5 11a7 7 0 0 0 14 0M12 18v4m-3 0h6',
    stop:'M7 7h10v10H7z',send:'M12 20V5m-6 6 6-6 6 6',download:'M12 3v12m-5-5 5 5 5-5M4 17v4h16v-4',
  });
  const section=document.createElement('section');section.id='app-agents';section.className='app-page hidden';
  section.innerHTML=`<div class="tms-agents"><aside id="agents-sidebar" class="agents-sidebar"><div class="agents-brand"><span class="agents-brand-icon"></span><div><strong>Агенты</strong><span>Помощники для работы</span></div></div><nav class="agents-navigation" aria-label="Разделы агентов"><button type="button" data-view="catalog">Все агенты</button><button type="button" data-view="history">Мои диалоги</button><button type="button" data-view="schedules">Расписание</button><button type="button" data-view="skills">Навыки</button><button type="button" data-view="settings">Настройки</button></nav><div class="agents-side-history"></div><div class="agents-account"><span class="agents-account-icon"></span><div><small>Рабочее пространство</small><p class="agents-scope"></p></div></div></aside><div class="agents-divider" role="separator" tabindex="0" aria-orientation="vertical" aria-label="Ширина панели диалогов" aria-controls="agents-sidebar"></div><div class="agents-workspace"><div class="agents-error" role="alert"></div><main class="agents-main"></main></div></div>`;
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
  let cleanupChatMedia=()=>{};
  let owner='',ownerToken='',epoch=0,revision=0,loadRevision=0,boot=null,config=null,history=[],active='',view='catalog',sending=false,controller=new AbortController();
  const node=(tag,text,className)=>{const el=document.createElement(tag);if(text!==undefined)el.textContent=text;if(className)el.className=className;return el;};
  const action=(text,run,className)=>{const el=node('button',text,className);el.type='button';el.onclick=run;return el;};
  function iconAction(symbol,label,handler) {
    const button=action('',handler,'agents-icon-action');
    button.append(icon(symbol));button.setAttribute('aria-label',label);button.title=label;
    return button;
  }
  function guard(){const e=epoch,email=owner,token=ownerToken;return()=>{if(e!==epoch||email!==state.email||!token||token!==state.tokens?.access_token)throw Error('Учётная запись изменилась');};}
  async function request(path,options={}){const check=guard();try{check();const out=await api('/api/v4/tms/agents'+path,{...options,signal:controller.signal,_guard:check});check();return out;}catch(err){if(ownerToken!==state.tokens?.access_token){reset();if(state.tokens?.access_token&&!section.classList.contains('hidden'))queueMicrotask(()=>run(load));}throw err;}}
  function report(err){if(err.name!=='AbortError')error.textContent=err.data?.message||err.message;}
  async function run(fn){const version=epoch;error.textContent='';try{await fn();}catch(err){if(version===epoch)report(err);}}
  function reset(){clearInterval(pollTimer);cleanupChatMedia();epoch++;revision++;loadRevision++;controller.abort();controller=new AbortController();owner='';ownerToken='';boot=null;config=null;history=[];active='';sending=false;sidebar.replaceChildren();main.replaceChildren();error.textContent='';section.querySelector('.agents-scope').textContent='';document.querySelectorAll('dialog[data-agents-dialog]').forEach(el=>el.remove());}
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
  async function show(next){clearInterval(pollTimer);cleanupChatMedia();
    view=next;revision++;active='';main.classList.remove('is-conversation');main.replaceChildren();renderSidebar();
    if(!boot)return;
    if(next==='catalog'){renderCatalog();return;}
    if(next==='skills'){renderSkills();return;}
    if(next==='schedules'){await renderSchedules();return;}
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
  async function openChat(id){clearInterval(pollTimer);cleanupChatMedia();
    const version=++revision;const chat=await request('/conversations/'+encodeURIComponent(id));if(version!==revision)return;
    const historyEntry=history.find(entry=>entry.id===id);if(historyEntry)historyEntry.title=chat.title;
    active=id;view='chat';main.replaceChildren();main.classList.add('is-conversation');renderSidebar();
    const agent=boot.catalog.agents.find(a=>a.id===chat.agent_id);const header=title(agent?.name||'Агент','Личный диалог');
    header.append(iconAction('schedule','Запланировать задачу',()=>editSchedule({conversation_id:id})),action('Новый диалог',()=>run(()=>newChat(chat.agent_id))),action('Убрать из списка',()=>run(async()=>{if(!window.confirm('Убрать диалог из списка? История ветки останется в Mattermost.'))return;await request('/conversations/'+encodeURIComponent(id),{method:'DELETE'});history=history.filter(c=>c.id!==id);await show('catalog');})));
    const approvals=node('div',undefined,'agents-approvals');main.append(approvals);
    const drawActions=()=>{approvals.replaceChildren();for(const item of chat.actions||[]){if(item.state!=='pending'||item.expires_at<Date.now())continue;const card=node('article',undefined,'agents-card');card.append(node('strong',item.tool==='mail_draft'?'Подтвердить отправку письма':'Подтвердить отправку сообщения'),node('p',item.arguments.to||item.arguments.channel_id),node('p',item.arguments.subject||''),node('pre',item.arguments.text));card.append(action('Отправить',()=>run(async()=>{await request('/conversations/'+encodeURIComponent(id)+'/actions/'+encodeURIComponent(item.id),{method:'POST',body:'{}'});item.state='sent';drawActions();}),'agents-primary'));approvals.append(card);}};drawActions();
    const messages=node('div',undefined,'agents-messages');messages.setAttribute('role','log');messages.setAttribute('aria-label','Сообщения диалога');main.append(messages);
    const chatPath='/conversations/'+encodeURIComponent(id);
    const fileURLs=new Map();
    const chatIsCurrent=()=>active===id&&version===revision&&!section.classList.contains('hidden');
    const form=node('form',undefined,'agents-composer'),input=node('textarea');
    input.rows=2;input.maxLength=16000;input.placeholder='Сообщение для '+(agent?.name||'агента');input.setAttribute('aria-label','Сообщение агенту');
    const status=node('span','','agents-send-status');status.setAttribute('role','status');
    const chips=node('div',undefined,'agents-draft-files');chips.hidden=true;
    const tools=node('div',undefined,'agents-composer-tools'),left=node('div'),right=node('div');
    const fileInput=node('input');fileInput.type='file';fileInput.multiple=true;fileInput.hidden=true;
    const attach=iconAction('attach','Прикрепить файлы',()=>fileInput.click());
    const mic=iconAction('mic','Записать аудио',()=>void toggleRecording());
    const send=iconAction('send','Отправить');send.type='submit';send.classList.add('agents-primary','agents-send');
    let files=[],submission=null,recorder=null,recordingTimer=null,starting=false,webChanging=false,closed=false;
    const supportedAudio=!!(navigator.mediaDevices?.getUserMedia&&window.MediaRecorder);
    const allowSearch=!!agent?.tools?.includes('web_search')&&boot.search_available!==false;
    const allowDownload=!!agent?.tools?.includes('web_download');
    const allowWeb=allowSearch||allowDownload;
    const webLabel=allowDownload?'Доступ к интернету':'Веб-поиск';
    const web=iconAction('web',webLabel,()=>run(async()=>{
      webChanging=true;syncWeb();
      try {
        const updated=await request(chatPath,{method:'PATCH',body:JSON.stringify({web_enabled:!chat.web_enabled})});
        chat.web_enabled=updated.web_enabled;
      } finally {webChanging=false;syncWeb();}
    }));
    web.classList.add('agents-web-toggle');
    web.append(node('span',webLabel));
    function syncWeb() {
      const enabled=allowWeb&&!!chat.web_enabled;
      web.setAttribute('aria-pressed',String(enabled));
      web.setAttribute('aria-label',webLabel);
      web.title=webLabel+(allowWeb?(enabled?' · включено':' · выключено'):' · недоступно');
      web.disabled=!allowWeb||webChanging;
    }
    function syncComposer() {
      send.disabled=sending||starting||!!recorder||(!input.value.trim()&&!files.length);
      attach.disabled=sending;
      mic.disabled=sending||starting||!supportedAudio;
      mic.setAttribute('aria-pressed',String(!!recorder));
      const micLabel=!supportedAudio?'Запись аудио недоступна в этом браузере':recorder?'Остановить запись':'Записать аудио';
      mic.setAttribute('aria-label',micLabel);mic.title=micLabel;
      mic.replaceChildren(icon(recorder?'stop':'mic'));
    }
    function drawDraft() {
      chips.replaceChildren();
      chips.hidden=!files.length;
      for(const file of files){
        const pill=node('span',undefined,'agents-file-pill');
        const remove=iconAction('close','Убрать '+file.name,()=>{files=files.filter(f=>f!==file);submission=null;drawDraft();});
        remove.disabled=sending;
        pill.append(icon('attach'),node('span',file.name),remove);
        chips.append(pill);
      }
      syncComposer();
    }
    function addFiles(added) {
      if(sending||closed)return;
      const next=[...files,...added];
      if(next.length>5||next.reduce((total,file)=>total+file.size,0)>1024**3){
        status.textContent='Можно прикрепить до 5 файлов общим размером до 1 ГиБ.';
        return;
      }
      files=next;submission=null;status.textContent='';drawDraft();
    }
    function releaseRecording() {
      clearTimeout(recordingTimer);
      if(!recorder)return;
      const current=recorder;
      recorder=null;
      if(current.state!=='inactive')current.stop();
      current.stream.getTracks().forEach(track=>track.stop());
    }
    async function toggleRecording() {
      if(recorder){
        if(recorder.state!=='inactive')recorder.stop();
        return;
      }
      if(starting||sending||closed||!supportedAudio||files.length>=5){
        if(files.length>=5)status.textContent='Освободите место для аудио в списке вложений.';
        return;
      }
      starting=true;syncComposer();
      let stream;
      try {
        stream=await navigator.mediaDevices.getUserMedia({audio:true});
        if(closed||!chatIsCurrent()){stream.getTracks().forEach(track=>track.stop());return;}
        const current=new MediaRecorder(stream),chunks=[];
        recorder=current;
        current.ondataavailable=event=>{if(event.data.size)chunks.push(event.data);};
        current.onstop=()=>{
          if(recorder!==current)return;
          releaseRecording();
          if(closed||!chatIsCurrent())return;
          const type=current.mimeType||chunks[0]?.type||'audio/webm';
          const extension=type.includes('mp4')?'m4a':type.includes('ogg')?'ogg':'webm';
          if(chunks.length){
            const stamp=new Date().toISOString().slice(0,19).replaceAll(':','-');
            addFiles([new File(chunks,'voice-message-'+stamp+'.'+extension,{type})]);
          }else{status.textContent='Запись пуста.';syncComposer();}
        };
        current.onerror=()=>{releaseRecording();status.textContent='Не удалось записать аудио.';syncComposer();};
        current.start();
        recordingTimer=setTimeout(()=>{if(current.state!=='inactive')current.stop();},180000);
        status.textContent='Запись… Нажмите микрофон, чтобы остановить.';
      }catch(err){
        stream?.getTracks().forEach(track=>track.stop());
        releaseRecording();
        if(!closed)status.textContent=err.name==='NotAllowedError'?'Разрешите доступ к микрофону в браузере.':'Не удалось открыть микрофон.';
      }finally{starting=false;syncComposer();}
    }
    cleanupChatMedia=()=>{
      closed=true;
      releaseRecording();
      for(const url of fileURLs.values())URL.revokeObjectURL(url);
      fileURLs.clear();
    };
    async function fileBlob(file) {
      const check=guard();check();
      const res=await fetch('/api/v4/files/'+encodeURIComponent(file.id),{headers:{Authorization:'Bearer '+ownerToken},signal:controller.signal});
      if(!res.ok)throw Error('Не удалось открыть вложение');
      const blob=await res.blob();check();
      if(closed||!chatIsCurrent())throw new DOMException('Диалог закрыт','AbortError');
      return blob;
    }
    function drawArtifact(file) {
      const item=node('div',undefined,'agents-message-file');
      const expiry=new Date(file.expires_at);
      if(expiry.getTime()<=Date.now()){
        item.append(node('span',file.name+' · срок скачивания истёк'));
        return item;
      }
      const download=iconAction('download','Скачать '+file.name,()=>run(async()=>{
        download.disabled=true;
        try {
          const check=guard();check();
          const response=await fetch('/api/v4/tms/agents/artifacts/'+encodeURIComponent(file.id),{headers:{Authorization:'Bearer '+ownerToken},signal:controller.signal});
          if(response.status===404)throw Error('Временный файл больше недоступен');
          if(!response.ok)throw Error('Не удалось скачать файл');
          const blob=await response.blob();check();
          const url=URL.createObjectURL(blob);
          const link=node('a');link.href=url;link.download=file.name;link.click();
          setTimeout(()=>URL.revokeObjectURL(url),1000);
        }finally{download.disabled=false;}
      }));
      item.append(node('span',file.name),download);
      item.title='Скачать до '+expiry.toLocaleString();
      return item;
    }
    function drawFile(file) {
      const item=node('div',undefined,'agents-message-file');
      const download=iconAction('download','Скачать '+file.name,()=>run(async()=>{
        download.disabled=true;
        try{
          const url=URL.createObjectURL(await fileBlob(file));
          const link=node('a');link.href=url;link.download=file.name;link.click();
          setTimeout(()=>URL.revokeObjectURL(url),1000);
        }finally{download.disabled=false;}
      }));
      item.append(node('span',file.name),download);
      if(/^voice-message-.+\.(webm|m4a|ogg)$/.test(file.name)&&file.size<=24*1024**2){
        const player=node('audio');player.controls=true;player.preload='none';player.setAttribute('aria-label',file.name);
        const play=action('Прослушать',()=>run(async()=>{
          play.disabled=true;
          try{
            if(!fileURLs.has(file.id)){
              const blob=await fileBlob(file);
              const type=file.name.endsWith('.m4a')?'audio/mp4':file.name.endsWith('.ogg')?'audio/ogg':'audio/webm';
              fileURLs.set(file.id,URL.createObjectURL(new Blob([blob],{type})));
            }
            player.src=fileURLs.get(file.id);play.replaceWith(player);await player.play().catch(()=>{});
          }finally{play.disabled=false;}
        }));
        if(fileURLs.has(file.id)){player.src=fileURLs.get(file.id);item.append(player);}
        else item.append(play);
      }
      return item;
    }
    const draw=()=>{
      messages.replaceChildren();
      if(!chat.messages.length)messages.append(emptyState('spark','Чем могу помочь?',agent?.description||'Задайте вопрос или опишите задачу. История этого диалога доступна только вам.'));
      for(const m of chat.messages){
        const article=node('article',undefined,'agents-message '+m.role);
        article.append(node('strong',m.role==='user'?'Вы':agent?.name||'Агент'));
        if(m.content){
          const content=node('div',m.content);
          if(m.role==='assistant')window.tmsChatMarkdown?.(content,m.content);
          article.append(content);
        }
        for(const file of m.files||[])article.append(drawFile(file));
        for(const file of m.artifacts||[])article.append(drawArtifact(file));
        messages.append(article);
      }
      messages.scrollTop=messages.scrollHeight;
    };
    fileInput.onchange=()=>{addFiles(fileInput.files);fileInput.value='';};
    input.oninput=()=>{submission=null;syncComposer();};
    input.onpaste=event=>{if(event.clipboardData?.files.length){event.preventDefault();addFiles(event.clipboardData.files);}};
    left.append(web);right.append(attach,mic,send);tools.append(left,right);
    form.append(input,chips,fileInput,tools,status);
    main.append(form,node('small','Сообщения и текст поддерживаемых вложений передаются провайдеру.','agents-notice'));
    syncWeb();drawDraft();draw();
    form.onsubmit=event=>{
      event.preventDefault();
      if(sending||starting||recorder||closed||(!input.value.trim()&&!files.length))return;
      const content=input.value;
      if(!submission)submission={content,pending_post_id:crypto.randomUUID()};
      const draft=submission;
      run(async()=>{
        sending=true;input.disabled=true;drawDraft();const check=guard();
        try{
          if(files.length&&!draft.file_ids){
            status.textContent='Загрузка файлов…';
            const data=new FormData();data.append('channel_id',chat.channel_id);
            for(const file of files)data.append('files',file,file.name);
            const uploaded=await api('/api/v4/files',{method:'POST',body:data,signal:controller.signal,_guard:check});
            check();if(closed||!chatIsCurrent())return;
            draft.file_ids=uploaded.file_infos.map(file=>file.id);
          }
          status.textContent='Агент готовит ответ…';
          const updated=await request(chatPath+'/messages',{method:'POST',body:JSON.stringify(draft)});
          check();
          const i=history.findIndex(c=>c.id===id);if(i>=0)history[i]=updated;
          if(chatIsCurrent()){
            chat.messages=updated.messages;chat.actions=updated.actions;drawActions();
            input.value='';files=[];submission=null;draw();renderSidebar();
          }
          status.textContent='';
        }catch(err){
          if(chatIsCurrent()){
            const latest=await request(chatPath).catch(()=>null);
            if(latest&&chatIsCurrent()){
              if(latest.messages.some(m=>m.pending_post_id===draft.pending_post_id)){
                input.value='';files=[];submission=null;
              }
              chat.messages=latest.messages;
              if(historyEntry)historyEntry.title=latest.title;
              draw();renderSidebar();
            }
          }
          status.textContent=err.data?.message||err.message;
        }finally{
          check();sending=false;input.disabled=false;drawDraft();
        }
      });
    };
    pollTimer=setInterval(async()=>{
      if(sending||!chatIsCurrent())return;
      try{
        const updated=await request(chatPath);if(!chatIsCurrent())return;
        if(historyEntry&&historyEntry.title!==updated.title){historyEntry.title=updated.title;renderSidebar();}
        if(!webChanging){chat.web_enabled=updated.web_enabled;syncWeb();}
        if(JSON.stringify(chat.messages)!==JSON.stringify(updated.messages)){chat.messages=updated.messages;draw();}
        chat.actions=updated.actions;drawActions();
      }catch{ /* Keep the draft when temporarily offline. */ }
    },4000);
    input.onkeydown=event=>{if(event.key==='Enter'&&(event.ctrlKey||event.metaKey)){event.preventDefault();form.requestSubmit();}};
  }
  const scheduleStatus={
    completed:'Выполнено',waiting:'Ожидает свободного агента',conversation_busy:'Диалог занят или достигнут предел истории',
    error:'Модель не ответила',unavailable:'Агент или диалог недоступен',
  };
  function scheduleDate(value,zone){
    return new Intl.DateTimeFormat(document.documentElement.lang||'ru',{dateStyle:'short',timeStyle:'short',timeZone:zone}).format(value);
  }
  async function renderSchedules(){
    const version=revision;
    const header=title('Расписание');
    const create=action('Добавить задачу',()=>editSchedule(),'agents-primary');
    create.disabled=!history.length;
    header.append(create);
    const items=await request('/schedules');
    if(version!==revision)return;
    if(!items.length){
      main.append(emptyState('schedule','Заданий пока нет',history.length?
        'Выберите личный диалог и задайте время выполнения.':'Откройте диалог с агентом, чтобы запланировать первое задание.'));
      return;
    }
    const list=node('div',undefined,'agents-schedule-list');
    for(const item of items){
      const chat=history.find(value=>value.id===item.conversation_id);
      const card=node('article',undefined,'agents-card agents-schedule');
      const text=node('div',undefined,'agents-schedule-summary');
      const stateText=item.paused?'На паузе':item.next_at?'Следующий запуск '+scheduleDate(item.next_at,item.timezone):'Завершено';
      text.append(node('strong',item.prompt),node('p',(chat?.title||'Личный диалог')+' · '+stateText+' · '+item.timezone));
      if(item.status)text.append(node('p',(item.last_at?scheduleDate(item.last_at,item.timezone)+' · ':'')+(scheduleStatus[item.status]||item.status)));
      const controls=node('div',undefined,'agents-actions');
      if(chat)controls.append(action('Диалог',()=>run(()=>openChat(chat.id))));
      controls.append(action('Изменить',()=>editSchedule(item)));
      if(item.next_at || item.paused){
        controls.append(action(item.paused?'Возобновить':'Пауза',()=>run(async()=>{
          await request('/schedules/'+item.id,{method:'PUT',body:JSON.stringify({...item,paused:!item.paused})});
          await show('schedules');
        })));
      }
      card.append(text,controls);
      list.append(card);
    }
    main.append(list);
  }
  function editSchedule(item={}){
    const {dialog,form}=modal(item.id?'Изменить задание':'Запланировать задание');
    const conversation=select(form,'Личный диалог',history.map(chat=>{
      const agent=boot.catalog.agents.find(value=>value.id===chat.agent_id);
      return [chat.id,(agent?.name||'Агент')+' · '+chat.title];
    }),item.conversation_id);
    conversation.disabled=Boolean(item.id);
    const prompt=field(form,'Что агент должен сделать',item.prompt||'','textarea');
    prompt.required=true;prompt.maxLength=8000;
    const promptHint=node('small','Опишите задачу целиком. При запуске агент получает этот текст.','agents-settings-note');
    promptHint.id='agents-schedule-prompt-hint';
    prompt.setAttribute('aria-describedby',promptHint.id);form.append(promptHint);
    const interval=select(form,'Периодичность',[['once','Один раз'],['daily','Каждый день'],['weekly','По дням недели']],item.frequency);
    const date=field(form,'Дата и время первого запуска',item.local_time||'','datetime-local');
    date.required=true;
    const zone=field(form,'Часовой пояс',item.timezone||Intl.DateTimeFormat().resolvedOptions().timeZone||'UTC');
    zone.required=true;
    const days=node('fieldset',undefined,'agents-schedule-days');
    days.append(node('legend','Дни недели'));
    const dayFields=['Пн','Вт','Ср','Чт','Пт','Сб','Вс'].map((day,index)=>{
      const id=(index+1)%7;
      const checkbox=field(days,day,'','checkbox');
      checkbox.checked=item.weekdays?.includes(id)||false;
      return [id,checkbox];
    });
    const toggleDays=()=>{days.hidden=interval.value!=='weekly';};
    interval.onchange=toggleDays;toggleDays();form.append(days);
    const paused=field(form,'На паузе','','checkbox');
    paused.checked=Boolean(item.paused);
    const controls=node('div',undefined,'agents-actions');
    const status=node('p',undefined,'agents-error');
    status.setAttribute('role','alert');
    if(item.id)controls.append(action('Удалить',()=>run(async()=>{
      if(!window.confirm('Удалить задание из расписания?'))return;
      await request('/schedules/'+item.id,{method:'DELETE'});
      dialog.close();await show('schedules');
    })));
    const submit=node('button','Сохранить','agents-primary');
    submit.type='submit';controls.append(submit);form.append(controls,status);
    form.onsubmit=async event=>{
      event.preventDefault();submit.disabled=true;status.textContent='';
      try{
        await request(item.id?'/schedules/'+item.id:'/schedules',{method:item.id?'PUT':'POST',body:JSON.stringify({
          conversation_id:conversation.value,prompt:prompt.value,frequency:interval.value,local_time:date.value,timezone:zone.value.trim(),
          weekdays:dayFields.filter(([,checkbox])=>checkbox.checked).map(([id])=>id),paused:paused.checked,
        })});
        dialog.close();await show('schedules');
      }catch(err){status.textContent=err.data?.message||err.message;}finally{submit.disabled=false;}
    };
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
    form.append(node('p','Инструменты действуют от имени пользователя диалога. Выбранные разрешения задают доступ модели к данным.','agents-settings-note'));
    const tools=[];
    for(const [id,label] of [
      ['web_search','Поиск в интернете'],
      ['web_download','Загрузка публичных файлов по ссылке'],
      ['file_create','Создание временных документов, презентаций и диаграмм'],
      ['image_generate','Генерация изображений'],
      ['mail_read','Поиск и чтение моей почты'],
      ['mail_send','Подготовка писем с подтверждением'],
      ['calendar_read','Чтение моих календарей'],
      ['chat_read','Поиск и чтение доступных мне чатов'],
      ['chat_send','Подготовка сообщений с подтверждением'],
    ]){
      const checkbox=field(form,label,'','checkbox');
      checkbox.checked=a.tools?.includes(id);
      tools.push([id,checkbox]);
    }
    const imageToggle=tools.find(([id])=>id==='image_generate')[1];
    const imageConnections=next.providers.filter(p=>['openai','openai-compatible'].includes(p.kind));
    const imageSettings=node('div',undefined,'agents-image-settings');
    imageToggle.closest('label').after(imageSettings);
    const imageProvider=select(imageSettings,'Провайдер изображений',
      [['','Выберите подключение'],...imageConnections.map(p=>[p.id,p.name])],
      a.image_provider_id||imageConnections.find(p=>p.id===a.provider_id)?.id||'');
    const imageModel=field(imageSettings,'Модель изображений',a.image_model||'');
    imageModel.maxLength=200;imageModel.placeholder='gpt-image-2.6-flare';
    imageProvider.required=true;
    const showImageSettings=()=>{
      imageSettings.hidden=!imageToggle.checked;
      imageProvider.disabled=imageModel.disabled=!imageToggle.checked;
    };
    imageToggle.disabled=imageConnections.length===0;
    if(imageToggle.disabled)imageToggle.closest('label').title='Добавьте подключение с OpenAI Images API';
    imageToggle.onchange=showImageSettings;
    showImageSettings();
    const skills=[];for(const skill of next.skills){const checkbox=field(form,skill.name,'','checkbox');checkbox.checked=a.skill_ids?.includes(skill.id);skills.push([skill.id,checkbox]);}
    finishEditor(dialog,form,async()=>{
      const updated={...a,name:name.value,description:description.value,provider_id:provider.value,
        instructions:instructions.value,enabled:enabled.checked,
        image_provider_id:imageToggle.checked?imageProvider.value:'',
        image_model:imageToggle.checked?imageModel.value.trim():'',
        tools:tools.filter(([,el])=>el.checked).map(([key])=>key),
        skill_ids:skills.filter(([,el])=>el.checked).map(([key])=>key)};
      next.agents=next.agents.filter(v=>v.id!==a.id);next.agents.push(updated);await saveConfig(next);
    },original?async()=>{next.agents=next.agents.filter(v=>v.id!==a.id);await saveConfig(next);}:null);
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
    const thinking=field(form,'DeepSeek: режим рассуждений','','checkbox');thinking.checked=!!p.thinking;
    const hint=node('small');form.append(hint);
    const presets={deepseek:{name:'DeepSeek',url:'https://api.deepseek.com',model:'deepseek-flash',hint:'API-ключ из platform.deepseek.com. Подключение доступно только вашему домену.'},ollama:{name:'Ollama',url:'http://127.0.0.1:11434',model:'',hint:'Адрес Ollama со стороны сервера TMS. Ключ нужен только при защите API.'},openai:{name:'OpenAI',url:'https://api.openai.com/v1',model:'gpt-6.2-sol',hint:'Нужен ключ OpenAI API из platform.openai.com.'},'openai-compatible':{name:'',url:'',model:'',hint:'Базовый URL совместимого API, например https://model.example.com/v1.'}};
    const providerKind=()=>kind.value==='openai-compatible'&&original?.kind===''?'':kind.value;
    const result=node('small','');result.setAttribute('role','status');
    let probe=0;
    const test=action('Проверить подключение',async()=>{
      if(!url.reportValidity()||!key.reportValidity())return;
      const revision=++probe;
      test.disabled=true;
      picker.parentElement.classList.add('hidden');
      result.textContent='Получаем список моделей…';
      try {
        const {models}=await request('/config/models',{method:'POST',body:JSON.stringify({id:p.id,kind:providerKind(),url:url.value,api_key:key.value})});
        if(!dialog.open||!dialog.isConnected||revision!==probe)return;
        const options=document.createDocumentFragment();options.append(new Option('Выберите модель',''));
        for(const id of models)options.append(new Option(id,id));
        picker.replaceChildren(options);
        picker.value=models.includes(model.value)?model.value:'';
        picker.parentElement.classList.toggle('hidden',!models.length);
        result.textContent=models.length?'Моделей в каталоге: '+models.length:'Провайдер вернул пустой список моделей.';
      } catch(err) {
        if(dialog.open&&dialog.isConnected&&revision===probe)result.textContent=err.data?.message||err.message;
      } finally {
        if(revision===probe)test.disabled=false;
      }
    });
    const testRow=node('div',undefined,'agents-provider-test');testRow.append(test,result);form.append(testRow);
    const picker=select(form,'Модели провайдера',[],'');picker.parentElement.classList.add('hidden');
    picker.onchange=()=>{if(picker.value)model.value=picker.value;};
    model.oninput=()=>{picker.value=[...picker.options].some(option=>option.value===model.value)?model.value:'';};
    const resetProbe=()=>{probe++;test.disabled=false;result.textContent='';picker.parentElement.classList.add('hidden');picker.replaceChildren();};
    const sync=()=>{const selected=presets[kind.value],deepseek=kind.value==='deepseek';thinking.parentElement.classList.toggle('hidden',!deepseek);hint.textContent=selected.hint;url.readOnly=['deepseek','openai'].includes(kind.value);model.placeholder=kind.value==='ollama'?'Имя установленной модели':'Идентификатор модели';
      const retained=p.has_key&&p.kind===providerKind()&&p.url===url.value;key.required=['deepseek','openai'].includes(kind.value)&&!retained;key.parentElement.firstChild.textContent=retained?'Новый API-ключ (пусто — сохранить прежний)':kind.value==='ollama'?'API-ключ (если требуется)':'API-ключ';resetProbe();};
    kind.onchange=()=>{const selected=presets[kind.value];name.value=selected.name;url.value=selected.url;model.value=selected.model;key.value='';sync();};url.oninput=sync;key.oninput=resetProbe;sync();
    finishEditor(dialog,form,async()=>{next.providers=next.providers.filter(v=>v.id!==p.id);next.providers.push({id:p.id,kind:providerKind(),thinking:kind.value==='deepseek'&&thinking.checked,name:name.value,url:url.value,model:model.value,api_key:key.value});await saveConfig(next);},original?async()=>{if(next.agents.some(a=>a.provider_id===p.id))throw Error('Сначала переключите или удалите агентов этой модели');next.providers=next.providers.filter(v=>v.id!==p.id);await saveConfig(next);}:null);
  }
  section.querySelectorAll('[data-view]').forEach(b=>{b.onclick=()=>run(()=>show(b.dataset.view));});
  const previous=showApp;showApp=function(name){previous(name);if(name==='agents'){run(load);}else{cleanupChatMedia();}};
  window.addEventListener('pagehide',()=>cleanupChatMedia());
  window.addEventListener('tms-products-session-ended',reset);window.addEventListener('storage',event=>{if(event.key==='tayga.tokens'){reset();if(!section.classList.contains('hidden'))run(load);}});
})();
