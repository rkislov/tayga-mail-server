/* Shared TMS/Mattermost chat. No XMPP requests or tokens in WebSocket URLs. */
const mmChat = (() => {
  let generation = 0, owner = '', boot = null, socket = null, reconnect = null, heartbeat = null, poll = null;
  let me = null, team = null, channels = [], users = new Map(), memberships = new Map();
  let selected = '', posts = new Map(), page = 0, complete = false, historyBusy = false, historyRevision = 0;
  let sending = false, reply = '', pending = null, peopleMode = 'direct', peopleChannel = '', searchRevision = 0, searchTimer = null;
  let refreshing = null, viewBusy = false;
  let attachments = [], mediaEpoch = 0, mediaBytes = 0, mediaURLs = [], mediaController = new AbortController(), avatarCache = new Map();
  let browseRevision = 0, browsePage = 0, browseTerm = '';
  const el = id => $('mm-chat-' + id);
  const label = user => user?.nickname?.trim() || [user?.first_name, user?.last_name].filter(Boolean).join(' ').trim() || user?.email || user?.username || 'Пользователь';
  const current = () => owner && owner === state.email && !!state.tokens?.access_token;
  const guard = () => { const version = generation; return () => { if (!current() || generation !== version) throw Error('Учётная запись изменилась'); }; };
  async function request(path, options = {}) {
    const check = guard(); check();
    const result = await api('/api/v4' + path, {...options, _guard: check}); check(); return result;
  }
  const errorText = error => error.data?.message || error.message;
  const visible = () => !document.hidden && !el('thread').closest('#app-chat').classList.contains('hidden') && (!isMobilePane() || $('view-account').classList.contains('pane-mode-read')) && document.hasFocus();
  function connection(text) { el('connection').textContent = text; }
  const iconPaths = {back:'M15 5l-7 7 7 7',refresh:'M20 7v5h-5 M4 17v-5h5 M6 7a7 7 0 0 1 12-1l2 3 M4 15l2 3a7 7 0 0 0 12-1',new:'M12 20H4V4h12 M15 5l4-4 4 4-4 4 M10 14l9-9',channel:'M8 3 6 21 M16 3l-2 18 M3 8h18 M2 16h18',browse:'M21 11a9 9 0 1 1-9-8 M16 3h5v5 M21 3l-9 9',members:'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2 M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8 M20 21v-2a4 4 0 0 0-3-3.87 M16 3.13a4 4 0 0 1 0 7.75',call:'M15 8l6-4v16l-6-4 M3 5h12v14H3z',reply:'M9 10 4 5l5-4 M4 5h9a7 7 0 0 1 7 7v7',attach:'M21 11 12 20a6 6 0 0 1-9-9L13 1a4 4 0 0 1 6 6L9 17a2 2 0 0 1-3-3l9-9',close:'M6 6l12 12 M18 6 6 18',download:'M12 3v12 M7 10l5 5 5-5 M4 16v5h16v-5'};
  function iconButton(button, name, title) {
    button.classList.add('mm-icon-button'); button.title = title; button.setAttribute('aria-label',title);
    button.innerHTML = '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="'+iconPaths[name]+'"/></svg>';
    return button;
  }
  async function binary(path, options = {}, retry = false) {
    const check = guard(); check();
    const response = await fetch('/api/v4'+path, {...options, headers:{Authorization:'Bearer '+state.tokens.access_token}}); check();
    if (response.status === 401 && !retry && state.tokens?.refresh_token) {
      if (await refreshAccessToken()) { check(); return binary(path,options,true); }
    }
    if (!response.ok) throw Error('Не удалось загрузить файл ('+response.status+')');
    return response;
  }
  function avatar(user) {
    const node = document.createElement('span'); node.className = 'mm-avatar'; node.setAttribute('aria-hidden','true');
    node.textContent = label(user).split(/\s+/).slice(0,2).map(part=>part[0]).join('').toLocaleUpperCase();
    if (!user?.id) return node;
    if (!avatarCache.has(user.id) && avatarCache.size < 128) {
      const check = guard();
      avatarCache.set(user.id,(async()=>{try{const response=await binary('/users/'+encodeURIComponent(user.id)+'/image',{signal:mediaController.signal});const blob=await boundedPreviewBlob(response,2*1024*1024);check();const url=URL.createObjectURL(blob);return url}catch{return null}})());
    }
    avatarCache.get(user.id)?.then(url=>{if(url&&node.isConnected){const image=document.createElement('img');image.alt='';image.src=url;node.replaceChildren(image)}});
    return node;
  }
  function clearMedia() { mediaEpoch++; mediaURLs.forEach(url=>URL.revokeObjectURL(url)); mediaURLs=[]; mediaBytes=0; }
  function clearAttachments() { attachments.forEach(item=>{if(item.url)URL.revokeObjectURL(item.url)});attachments=[];pending=null;renderAttachments(); }
  function renderAttachments() {
    const box=el('attachments'); if(!box)return;box.replaceChildren();
    attachments.forEach(item=>{const row=document.createElement('div');row.className='mm-pending-file';if(item.url){const image=document.createElement('img');image.alt='';image.src=item.url;row.append(image)}const name=document.createElement('span');name.textContent=item.file.name;row.append(name);const remove=iconButton(document.createElement('button'),'close','Убрать '+item.file.name);remove.type='button';remove.disabled=sending;remove.onclick=()=>{if(item.url)URL.revokeObjectURL(item.url);attachments=attachments.filter(other=>other!==item);pending=null;renderAttachments()};row.append(remove);box.append(row)});
    box.hidden=!attachments.length;
  }
  function addFiles(files) {
    if(sending||!selected)return;
    if(attachments.length+files.length>5 || attachments.reduce((n,item)=>n+item.file.size,0)+[...files].reduce((n,file)=>n+file.size,0)>1024**3){setMsg(el('send-error'),'Можно прикрепить до 5 файлов общим размером до 1 ГиБ','err');return}
    for(const file of files)attachments.push({file,url:/^image\/(png|jpeg|gif|webp)$/.test(file.type)&&file.size<16*1024*1024?URL.createObjectURL(file):null});
    pending=null;setMsg(el('send-error'),'');renderAttachments();
  }
  async function renderFiles(post, host, epoch) {
    for(const id of post.file_ids||[]) {
      if(epoch!==mediaEpoch||!current())return;
      const card=document.createElement('div');card.className='mm-file';const format=document.createElement('span');format.className='mm-file-format';format.textContent='FILE';format.setAttribute('aria-hidden','true');card.append(format);const name=document.createElement('span');name.textContent='Вложение';card.append(name);host.append(card);
      const download=iconButton(document.createElement('button'),'download','Скачать вложение');download.type='button';card.append(download);
      download.onclick=async()=>{download.disabled=true;try{const check=guard(),response=await binary('/files/'+encodeURIComponent(id));const blob=await response.blob();check();if(epoch!==mediaEpoch)return;const url=URL.createObjectURL(blob);const link=document.createElement('a');link.href=url;link.download=name.textContent;link.click();setTimeout(()=>URL.revokeObjectURL(url),1000)}catch(error){report(error)}finally{download.disabled=false}};
      try {
        const info=await request('/files/'+encodeURIComponent(id)+'/info');if(epoch!==mediaEpoch)return;
        name.textContent=info.name;const extension=String(info.extension||info.name.split('.').pop()||'FILE').replace(/[^a-z0-9]/gi,'').slice(0,5).toUpperCase();format.textContent=extension||'FILE';format.dataset.format=extension;download.setAttribute('aria-label','Скачать '+info.name);download.title=download.getAttribute('aria-label')?.replace('вложение',info.name)||info.name;
        const size=document.createElement('small');size.textContent=fmtBytes(info.size);card.append(size);
        if(info.has_preview_image && info.size<=12*1024*1024 && mediaBytes+info.size<=32*1024*1024) {
          mediaBytes+=info.size;
          const response=await binary('/files/'+encodeURIComponent(id)+'/preview',{signal:mediaController.signal});const blob=await boundedPreviewBlob(response,12*1024*1024);
          if(epoch!==mediaEpoch||!current())return;
          const url=URL.createObjectURL(blob);mediaURLs.push(url);const image=document.createElement('img');image.alt=info.name;image.src=url;image.className='mm-file-image';card.classList.add('has-image');card.prepend(image);
          image.onload=()=>{const box=el('posts');if(box.scrollHeight-box.scrollTop-box.clientHeight<image.height+100)box.scrollTop=box.scrollHeight};
        }
      } catch(error) { if(epoch===mediaEpoch){card.classList.add('has-error');name.textContent='Вложение недоступно';card.title=errorText(error)} }
    }
  }
  async function refreshStatus() {try{const status=await request('/users/me/status');el('status').value=status.status;el('status').dataset.status=status.status}catch{} }
  function setupControls() {
    iconButton(el('thread').querySelector('[data-pane-back]'),'back','К списку чатов');
    for(const [id,name,title] of [['refresh','refresh','Обновить'],['new','new','Новый диалог'],['create','channel','Создать канал'],['browse','browse','Открытые каналы'],['members','members','Участники'],['call','call','Видеозвонок'],['reply-cancel','close','Отменить ответ']])iconButton(el(id),name,title);
    const identity=el('self').parentElement, profile=document.createElement('div');profile.className='mm-profile-text';profile.append(el('self'),el('connection'));identity.append(profile);
    const status=document.createElement('select');status.id='mm-chat-status';status.className='mm-status';status.setAttribute('aria-label','Мой статус');
    for(const [value,title] of [['online','В сети'],['away','Отошёл'],['dnd','Не беспокоить'],['offline','Не в сети']])status.add(new Option(title,value));profile.append(status);
    status.onchange=async()=>{status.disabled=true;try{await request('/users/me/status',{method:'PUT',body:JSON.stringify({status:status.value})});status.dataset.status=status.value}catch(error){report(error);await refreshStatus()}finally{status.disabled=false}};
    const input=document.createElement('input');input.type='file';input.multiple=true;input.hidden=true;input.id='mm-chat-files';input.onchange=()=>{addFiles(input.files);input.value=''};
    const attach=iconButton(document.createElement('button'),'attach','Прикрепить файлы');attach.id='mm-chat-attach';attach.type='button';attach.onclick=()=>input.click();
    const box=document.createElement('div');box.id='mm-chat-attachments';box.hidden=true;el('send').before(box);el('send').prepend(attach,input);
    el('body').addEventListener('paste',event=>{if(event.clipboardData.files.length){event.preventDefault();addFiles(event.clipboardData.files)}});
    el('send').addEventListener('dragover',event=>{if(event.dataTransfer.types.includes('Files')){event.preventDefault();el('send').classList.add('is-dragging')}});
    el('send').addEventListener('dragleave',()=>el('send').classList.remove('is-dragging'));
    el('send').addEventListener('drop',event=>{event.preventDefault();el('send').classList.remove('is-dragging');addFiles(event.dataTransfer.files)});
  }
  setupControls();
  function fitMobileViewport() {
    const root=$('view-account'), viewport=window.visualViewport;
    if(viewport && isMobilePane() && Math.abs(viewport.scale-1)<0.05) root.style.height=viewport.height+'px';
    else root.style.removeProperty('height');
  }
  window.visualViewport?.addEventListener('resize',fitMobileViewport);
  window.addEventListener('resize',fitMobileViewport);
  fitMobileViewport();
  function stop() {
    clearMedia(); clearAttachments(); mediaController.abort(); mediaController=new AbortController();
    avatarCache.forEach(promise=>promise.then(url=>{if(url)URL.revokeObjectURL(url)}));avatarCache.clear();
    el('self').closest('.mm-chat-identity').querySelector('.mm-avatar')?.remove();
    generation++; historyRevision++; searchRevision++; browseRevision++;
    el('browse-dialog')?.close(); el('browse-results')?.replaceChildren();
    clearTimeout(reconnect); clearTimeout(searchTimer); clearInterval(heartbeat); clearInterval(poll);
    reconnect = heartbeat = poll = null;
    const old = socket; socket = null; if (old) { old.onclose = null; old.close(); }
    owner = ''; boot = refreshing = null; me = team = null; channels = []; users = new Map(); memberships = new Map();
    selected = reply = ''; posts = new Map(); pending = null; sending = historyBusy = viewBusy = false;
    el('posts').replaceChildren(); el('channels').replaceChildren(); el('people-list').replaceChildren(); el('body').value = ''; el('filter').value = ''; el('search').value = '';
    el('people').close(); el('create-dialog').close(); el('thread').classList.add('hidden'); el('empty').classList.remove('hidden');
    el('reply').classList.add('hidden'); el('self').textContent = ''; el('send').querySelector('[type=submit]').disabled = false;
  }
  async function ensure() {
    if (!state.tokens?.access_token || !state.email) return false;
    if (owner && owner !== state.email) stop();
    if (boot) return boot;
    owner = state.email; const version = generation;
    const pendingBoot = (async () => {
      connection('Подключение…');
      try {
        me = await request('/users/me'); users.set(me.id, me);
        const teams = await request('/users/me/teams'); team = teams[0];
        if (!team) throw Error('Нет доступной команды');
        el('self').textContent = label(me);el('self').closest('.mm-chat-identity').prepend(avatar(me));void refreshStatus();
        await syncChannels(); connect();
        poll = setInterval(() => { if (current()) syncChannels().catch(report); }, 30000);
        return true;
      } catch (error) {
        if (generation === version && current()) { connection('Нет соединения'); report(error); boot = null; }
        return false;
      }
    })();
    boot = pendingBoot;
    return pendingBoot;
  }
  function report(error) { if (current()) setMsg(el('error'), errorText(error), 'err'); }
  function start() { void ensure(); }
  async function getUsers(ids) {
    const missing = [...new Set(ids.filter(id => id && !users.has(id)))];
    for (let offset = 0; offset < missing.length; offset += 200) {
      const list = await request('/users/ids', {method:'POST', body:JSON.stringify(missing.slice(offset, offset + 200))});
      list.forEach(user => users.set(user.id, user));
    }
  }
  async function syncChannels() {
    if (refreshing) return refreshing;
    const version = generation, check = guard();
    const task = (async () => {
      const [list, members] = await Promise.all([
        request('/users/me/teams/' + team.id + '/channels'),
        request('/users/me/teams/' + team.id + '/channels/members')
      ]);
      await getUsers(list.filter(channel => channel.type === 'D').flatMap(channel => channel.name.split('__')));
      check(); channels = list; memberships = new Map(members.map(member => [member.channel_id, member]));
      if (selected && !channels.some(channel => channel.id === selected)) { selected = ''; posts.clear(); el('posts').replaceChildren(); el('thread').classList.add('hidden'); el('empty').classList.remove('hidden'); }
      renderChannels(); setMsg(el('error'), '');
    })().finally(() => { if (generation === version) refreshing = null; });
    refreshing = task; return task;
  }
  function channelName(channel) {
    if (channel.type !== 'D') return channel.display_name || channel.name;
    const other = channel.name.split('__').find(id => id !== me?.id) || me?.id;
    return other === me?.id ? 'Избранное (сообщения себе)' : label(users.get(other));
  }
  function renderChannels() {
    const query = el('filter').value.trim().toLocaleLowerCase(), container = el('channels');
    container.replaceChildren();
    for (const [type, title] of [['channels','Каналы'],['direct','Личные сообщения']]) {
      const list = channels.filter(channel => (channel.type === 'D') === (type === 'direct') && channelName(channel).toLocaleLowerCase().includes(query));
      const heading = document.createElement('h3'); heading.className = 'meta mm-chat-group'; heading.textContent = title; container.append(heading);
      list.sort((a,b) => type === 'direct' ? b.last_post_at - a.last_post_at || channelName(a).localeCompare(channelName(b)) : channelName(a).localeCompare(channelName(b)));
      for (const channel of list) {
        const member = memberships.get(channel.id), unread = Math.max(0, channel.total_msg_count - (member?.msg_count || 0));
        const button = document.createElement('button'); button.type = 'button'; button.className = 'msg-item mm-chat-channel'; button.classList.toggle('is-active', channel.id === selected);
        if (channel.id === selected) button.setAttribute('aria-current','true');
        const name = document.createElement('span'); name.textContent = (channel.type === 'D' ? '' : channel.type === 'P' ? '🔒 ' : '# ') + channelName(channel); if(channel.type==='D')button.append(avatar(users.get(channel.name.split('__').find(id=>id!==me?.id)||me?.id)));button.append(name);
        if (unread) { const badge = document.createElement('span'); badge.className = 'mm-chat-unread'; badge.textContent = String(unread); badge.setAttribute('aria-label',unread + ' непрочитанных'); button.append(badge); }
        button.onclick = () => open(channel.id).catch(report); container.append(button);
      }
      if (!list.length) { const hint = document.createElement('p'); hint.className = 'meta'; hint.textContent = type === 'direct' ? 'Начните новый диалог' : 'Нет каналов'; container.append(hint); }
    }
    const channel = channels.find(item => item.id === selected);
    if (channel) el('title').textContent = channelName(channel);
  }
  async function refresh() {
    if (!await ensure()) return;
    try { await syncChannels(); if (selected) await loadHistory(true); } catch (error) { report(error); }
  }
  async function open(id) {
    if (!await ensure()) return;
    clearMedia(); clearAttachments();
    selected = id; historyRevision++; posts = new Map(); page = 0; complete = historyBusy = false; reply = ''; pending = null;
    el('reply').classList.add('hidden'); el('body').value = ''; el('thread').classList.remove('hidden'); el('empty').classList.add('hidden');
    el('posts').replaceChildren(); setMsg(el('send-error'),''); renderChannels(); setMobilePane('read');
    await loadHistory();
  }
  async function loadHistory(resync = false) {
    if (!selected || historyBusy) return;
    const id = selected, revision = historyRevision, version = generation;
    historyBusy = true;
    try {
      // Reconnect fetches the latest page while preserving older loaded entries.
      const since = resync && posts.size ? Math.max(...[...posts.values()].map(post => post.create_at)) - 1 : 0;
      const list = []; let cursor = resync ? 0 : page, batch;
      do {
        const data = await request('/channels/' + id + '/posts?per_page=80&page=' + cursor + (since ? '&since=' + Math.max(0,since) : ''));
        batch = data.order.map(key => data.posts[key]); list.push(...batch); cursor++;
        if (revision !== historyRevision || version !== generation || id !== selected) return;
      } while (since && batch.length === 80);
      await getUsers(list.map(post => post.user_id));
      if (revision !== historyRevision || version !== generation || id !== selected) return;
      list.forEach(post => posts.set(post.id, post));
      if (!resync) { page++; complete = list.length < 80; }
      renderPosts(!resync && page > 1); void markViewed();
    } catch (error) { if (revision === historyRevision && version === generation) setMsg(el('send-error'),errorText(error),'err'); }
    finally { if (revision === historyRevision && version === generation) historyBusy = false; }
  }
  function renderPosts(keepScroll = false) {
    const box = el('posts'), previousHeight = box.scrollHeight, previousTop = box.scrollTop;
    const nearBottom = previousHeight - previousTop - box.clientHeight < 100;
    clearMedia(); const epoch=mediaEpoch;
    box.replaceChildren();
    if (!complete) { const older = document.createElement('button'); older.type = 'button'; older.className = 'btn-secondary btn-sm'; older.textContent = 'Загрузить более ранние'; older.onclick = () => loadHistory(); box.append(older); }
    const ordered = [...posts.values()].sort((a,b) => a.create_at - b.create_at || a.id.localeCompare(b.id));
    for (const post of ordered) {
      const article = document.createElement('article'); article.className = 'chat-bubble' + (post.user_id === me.id ? ' is-mine' : ''); article.dataset.postId = post.id;
      const author = document.createElement('strong'); author.textContent = label(users.get(post.user_id)); article.append(avatar(users.get(post.user_id)),author);
      if (post.root_id) { const root = posts.get(post.root_id), quote = document.createElement('div'); quote.className = 'mm-chat-quote'; quote.textContent = 'Ответ: ' + (root?.message?.slice(0,160) || 'сообщение в ветке'); article.append(quote); }
      const text = document.createElement('p');
      const callURL = meetingURL(post.props?.tms_meeting_url);
      if (callURL) {
        const link = document.createElement('a');
        link.href = callURL;
        link.textContent = 'Присоединиться к видеозвонку';
        text.append(link);
      } else {
        text.textContent = post.message || '';text.hidden=!post.message;
      }
      article.append(text);
      if (users.get(post.user_id)?.is_bot && Array.isArray(post.props?.attachments)) {
        const actions=document.createElement('div');actions.className='mm-bot-actions';
        for (const attachment of post.props.attachments.slice(0,10)) {
          if (!Array.isArray(attachment.actions)) continue;
          for (const action of attachment.actions.slice(0,10)) {
            if (action.type!=='button' || !/^[a-zA-Z0-9_-]{1,64}$/.test(action.id||'') || typeof action.name!=='string') continue;
            const button=document.createElement('button');button.type='button';button.className='btn-secondary btn-sm';button.textContent=action.name.slice(0,100);
            button.onclick=async()=>{button.disabled=true;try{await request('/posts/'+encodeURIComponent(post.id)+'/actions/'+encodeURIComponent(action.id),{method:'POST',body:'{}'});}catch(error){setMsg(el('send-error'),errorText(error),'err')}finally{button.disabled=false}};
            actions.append(button);
          }
        }
        if(actions.childNodes.length)article.append(actions);
      }
      if(post.file_ids?.length){const files=document.createElement('div');files.className='mm-post-files';article.append(files);void renderFiles(post,files,epoch)}
      const time = document.createElement('time'); time.dateTime = new Date(post.create_at).toISOString(); time.textContent = new Date(post.create_at).toLocaleString(lang === 'en' ? 'en-GB' : 'ru-RU'); article.append(time);
      const actions=document.createElement('div');actions.className='mm-message-actions';
      const discuss=()=>beginDiscussion(post);
      const answer=document.createElement('button');answer.type='button';answer.className='btn-secondary btn-sm mm-chat-answer';iconButton(answer,'reply',lang==='en'?'Reply':'Ответить');answer.onclick=discuss;
      const more=window.tmsProductActions?.chatActions({title:post.message||'',discuss});
      actions.append(answer);if(more)actions.append(more);article.append(actions);
      box.append(article);
    }
    if (!ordered.length) { const hint = document.createElement('p'); hint.className = 'meta'; hint.textContent = 'Сообщений пока нет'; box.append(hint); }
    if (keepScroll) box.scrollTop = previousTop + box.scrollHeight - previousHeight;
    else if (nearBottom || previousHeight <= box.clientHeight) box.scrollTop = box.scrollHeight;
  }
  function beginDiscussion(post) {
    reply=post.root_id||post.id;
    el('reply-label').textContent=(lang==='en'?'Reply: ':'Ответ: ')+(post.message||'').slice(0,120);
    el('reply').classList.remove('hidden');el('body').focus();
  }
  async function discussMail(subject) {
    const english=lang==='en';
    const text=(ru,en)=>english?en:ru;
    if(!await ensure()){
      showApp('chat');
      setMsg(el('error'),text('Чат сейчас недоступен','Chat is currently unavailable'),'err');
      return;
    }
    const check=guard();
    const dialog=document.createElement('dialog');dialog.className='mm-chat-dialog product-create-dialog';
    const form=document.createElement('form');
    const heading=document.createElement('h2');heading.className='heading';heading.textContent=text('Начать обсуждение в чате','Start a chat discussion');
    const label=document.createElement('label');label.className='field-label';label.textContent=text('Диалог или канал','Conversation or channel');
    const select=document.createElement('select');select.className='field-input';select.required=true;
    for(const channel of channels)select.add(new Option(channelName(channel),channel.id));
    select.value=channels.some(channel=>channel.id===selected)?selected:select.options[0]?.value||'';
    label.append(select);
    const controls=document.createElement('div');controls.className='actions';
    const cancel=document.createElement('button');cancel.type='button';cancel.className='btn-secondary';cancel.textContent=text('Отмена','Cancel');cancel.onclick=()=>dialog.close();
    const submit=document.createElement('button');submit.type='submit';submit.className='btn-spray';submit.textContent=text('Открыть черновик','Open draft');submit.disabled=!select.value;
    const direct=document.createElement('button');direct.type='button';direct.className='btn-secondary';direct.textContent=text('Новый личный чат','New direct message');
    const draft=subject.trim()?text('Обсудим письмо «','Let’s discuss the email “')+subject.trim().slice(0,240)+text('».','”.'):'';
    const putDraft=()=>{
      check();
      if(reply){
        reply='';el('reply').classList.add('hidden');
      }
      setMobilePane('read');
      if(draft)el('body').value=[el('body').value.trimEnd(),draft].filter(Boolean).join('\n\n');
      el('body').focus();
    };
    direct.onclick=()=>{check();dialog.close();showApp('chat');void people('direct',putDraft);};
    controls.append(cancel,direct,submit);form.append(heading,label,controls);dialog.append(form);document.body.append(dialog);
    const controller=new AbortController();
    dialog.addEventListener('close',()=>{controller.abort();dialog.remove();},{once:true});
    window.addEventListener('tms-products-session-ended',()=>dialog.close(),{once:true,signal:controller.signal});
    form.onsubmit=async event=>{
      event.preventDefault();
      try {
        check();const channel=select.value;dialog.close();showApp('chat');
        if(channel!==selected)await open(channel);
        check();if(selected===channel)putDraft();
      } catch(error) {if(current())report(error);}
    };
    dialog.showModal();
  }
  async function markViewed() {
    if (!selected || !visible() || viewBusy) return;
    const id = selected, version = generation; viewBusy = true;
    try {
      await request('/channels/members/me/view',{method:'POST',body:JSON.stringify({channel_id:id})});
      const member = memberships.get(id), channel = channels.find(item => item.id === id);
      if (member && channel) { member.msg_count = channel.total_msg_count; renderChannels(); }
    } catch {} finally { if (version === generation) viewBusy = false; }
  }
  function connect() {
    if (!current() || socket) return;
    const check = guard(), version = generation;
    const url = new URL('/api/v4/websocket', location.href); url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(url); socket = ws; let seq = 1;
    ws.onopen = () => { try { check(); ws.send(JSON.stringify({seq:seq++,action:'authentication_challenge',data:{token:state.tokens.access_token}})); } catch { ws.close(); } };
    ws.onmessage = async event => {
      if (socket !== ws) return;
      try {
        check(); const message = JSON.parse(event.data);
        if (message.event === 'hello') {
          connection('Подключено');
          clearInterval(heartbeat); heartbeat = setInterval(() => { if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({seq:seq++,action:'ping',data:{}})); },25000);
          await syncChannels(); if (selected) await loadHistory(true); return;
        }
        if (message.event !== 'posted') { if (['channel_created','direct_added','added_to_team','user_added','channel_viewed'].includes(message.event)) await syncChannels(); return; }
        const post = JSON.parse(message.data.post);
        if (!channels.some(channel => channel.id === post.channel_id)) await syncChannels();
        const channel = channels.find(item => item.id === post.channel_id);
        if (channel) { channel.last_post_at = Math.max(channel.last_post_at,post.create_at); }
        if (post.channel_id === selected) {
          await getUsers([post.user_id]); check();
          if (post.channel_id === selected) { posts.set(post.id,post); renderPosts(); void markViewed(); }
        }
        renderChannels();
        // Counts are authoritative on the server; never infer them from duplicate events.
        await syncChannels();
      } catch (error) { if (version === generation && current()) report(error); }
    };
    ws.onclose = () => {
      if (socket !== ws || generation !== version) return;
      socket = null; clearInterval(heartbeat); connection('Соединение потеряно · переподключение…');
      reconnect = setTimeout(async () => {
        if (!current() || generation !== version) return;
        try { await request('/users/me'); connect(); } catch (error) { if (generation === version && current()) { report(error); reconnect = setTimeout(connect,5000); } }
      },2000);
    };
    ws.onerror = () => { if (socket === ws) connection('Переподключение…'); };
  }
  async function send(event) {
    event.preventDefault();
    const input = el('body'), body = input.value.trim(), id = selected, root = reply;
    if ((!body && !attachments.length) || !id || sending || !me) return;
    const version = generation, selectedFiles=[...attachments]; sending = true;renderAttachments(); const button = el('send').querySelector('[type=submit]'); button.disabled = true; setMsg(el('send-error'),'');
    if (!pending || pending.channel_id !== id || pending.message !== body || pending.root_id !== root) pending = {channel_id:id,message:body,root_id:root,pending_post_id:me.id + ':' + crypto.randomUUID()};
    try {
      const submission = pending;
      if(selectedFiles.length && !submission.file_ids){setMsg(el('send-error'),'Загрузка файлов…');const data=new FormData();data.append('channel_id',id);selectedFiles.forEach(item=>data.append('files',item.file,item.file.name));const response=await binary('/files?channel_id='+encodeURIComponent(id),{method:'POST',body:data});const uploaded=await response.json();if(version!==generation||!current())return;submission.file_ids=uploaded.file_infos.map(info=>info.id)}
      setMsg(el('send-error'),'');
      const post = await request('/posts',{method:'POST',body:JSON.stringify(submission)});
      if (selected === id) { posts.set(post.id,post); renderPosts(); el('posts').scrollTop = el('posts').scrollHeight; if (input.value.trim() === body) input.value = ''; reply = ''; el('reply').classList.add('hidden'); clearAttachments(); }
      if (pending === submission) pending = null; await syncChannels(); void markViewed();
    } catch (error) { if (generation === version) setMsg(el('send-error'),errorText(error),'err'); }
    finally { if (generation === version) { sending = false; button.disabled = false;renderAttachments(); } }
  }
  async function searchPeople() {
    const revision = ++searchRevision;
    try {
      const list = await request('/users/search',{method:'POST',body:JSON.stringify({term:el('search').value.trim(),team_id:team.id,limit:100})});
      if (revision !== searchRevision || !el('people').open) return;
      el('people-list').replaceChildren(); setMsg(el('people-error'),'');
      for (const user of list) {
        users.set(user.id,user);
        const button = document.createElement('button'); button.type = 'button'; button.className = 'msg-item mm-chat-person';
        const name = document.createElement('strong'); name.textContent = label(user); const username = document.createElement('span'); username.className = 'meta'; username.textContent = '@' + user.username; button.append(avatar(user),name,username);
        button.onclick = async () => {
          button.disabled = true;
          try {
            if (peopleMode === 'member') { await request('/channels/' + peopleChannel + '/members',{method:'POST',body:JSON.stringify({user_id:user.id})}); el('people').close(); }
            else { const channel = await request('/channels/direct',{method:'POST',body:JSON.stringify([me.id,user.id])}); const after=peopleAfterOpen;el('people').close(); await syncChannels(); await open(channel.id);if(selected===channel.id)after?.(); }
          } catch (error) { setMsg(el('people-error'),errorText(error),'err'); button.disabled = false; }
        };
        el('people-list').append(button);
      }
      if (!list.length) { const hint = document.createElement('p'); hint.className = 'meta'; hint.textContent = 'Пользователи не найдены'; el('people-list').append(hint); }
    } catch (error) { if (revision === searchRevision && current()) setMsg(el('people-error'),errorText(error),'err'); }
  }
  let peopleAfterOpen=null;
  async function people(mode = 'direct',afterOpen=null) {
    if (!await ensure()) return;
    peopleAfterOpen=afterOpen;
    peopleMode = mode; peopleChannel = selected; el('search').hidden = false; el('people-list').replaceChildren(); el('search').value = ''; setMsg(el('people-error'),'');
    el('people-title').textContent = mode === 'member' ? 'Добавить участника' : 'Новый диалог'; el('people').showModal(); el('search').focus(); await searchPeople();
  }
  async function openPeer(email) {
    if (!await ensure()) return;
    try { const user = await request('/users/email/' + encodeURIComponent(email)); users.set(user.id,user); const channel = await request('/channels/direct',{method:'POST',body:JSON.stringify([me.id,user.id])}); await syncChannels(); await open(channel.id); } catch (error) { report(error); }
  }
  async function members() {
    if (!selected) return;
    const id = selected;
    try {
      const list = await request('/channels/' + id + '/members'); await getUsers(list.map(member => member.user_id));
      if (id !== selected) return;
      el('people-list').replaceChildren(); el('search').value = ''; el('search').hidden = true; el('people-title').textContent = 'Участники'; setMsg(el('people-error'),'');
      for (const member of list) { const row = document.createElement('p'); row.className='mm-member';row.append(avatar(users.get(member.user_id)),document.createTextNode(label(users.get(member.user_id)))); el('people-list').append(row); }
      if (channels.find(channel => channel.id === id)?.type !== 'D') { const add = document.createElement('button'); add.type = 'button'; add.className = 'btn-spray'; add.textContent = 'Добавить участника'; add.onclick = () => { peopleMode = 'member'; peopleChannel = id; el('search').hidden = false; el('people-title').textContent = 'Добавить участника'; el('search').focus(); void searchPeople(); }; el('people-list').append(add); }
      el('people').showModal();
    } catch (error) { report(error); }
  }
  function meetingURL(value) {
    if (!value) return '';
    try {
      const url = new URL(value);
      if (url.origin === location.origin && url.pathname === '/' &&
          url.searchParams.get('app') === 'vks' && /^tms-[0-9a-f]{32}$/.test(url.searchParams.get('room'))) {
        return url.href;
      }
    } catch {}
    return '';
  }

  async function startCall() {
    const channel = selected, check = guard(), button = el('call');
    if (!channel) return;
    button.disabled = true;
    try {
      const result = await request('/commands/execute', {
        method:'POST', body:JSON.stringify({command:'/meet',channel_id:channel,team_id:team.id}),
      });
      check();
      const link = meetingURL(result.goto_location);
      if (!link) throw new Error('Не удалось открыть звонок');
      if (selected === channel) location.assign(link);
    } catch (error) { report(error); }
    finally { button.disabled = false; }
  }

  async function browseChannels(append = false) {
    if (!await ensure() || !el('browse-dialog').open) return;
    const results = el('browse-results'), more = el('browse-more');
    if (!append) {
      browseRevision++;
      browsePage = 0;
      browseTerm = el('browse-query').value.trim();
      results.replaceChildren();
    }
    const revision = browseRevision;
    more.classList.add('hidden');
    setMsg(el('browse-status'), 'Поиск…');
    try {
      const list = await request('/teams/' + team.id + '/channels/search?page=' + browsePage + '&per_page=30', {
        method:'POST', body:JSON.stringify({term:browseTerm}),
      });
      if (revision !== browseRevision || !el('browse-dialog').open) return;
      for (const channel of list) {
        const row = document.createElement('div');
        row.className = 'mm-person-row';
        const title = document.createElement('span');
        title.textContent = channelName(channel);
        const action = document.createElement('button');
        action.type = 'button';
        action.className = 'btn-secondary btn-sm';
        action.textContent = memberships.has(channel.id) ? 'Открыть' : 'Вступить';
        action.onclick = async () => {
          if (revision !== browseRevision || !el('browse-dialog').open) return;
          action.disabled = true;
          try {
            if (!memberships.has(channel.id)) {
              await request('/channels/' + channel.id + '/members', {
                method:'POST', body:JSON.stringify({user_id:me.id}),
              });
              await syncChannels();
              if (!memberships.has(channel.id)) await syncChannels();
            }
            if (revision !== browseRevision) return;
            el('browse-dialog').close();
            await open(channel.id);
          } catch (error) {
            if (revision === browseRevision) setMsg(el('browse-status'), errorText(error), 'err');
          } finally { action.disabled = false; }
        };
        row.append(title, action);
        results.append(row);
      }
      browsePage++;
      more.classList.toggle('hidden', list.length < 30);
      setMsg(el('browse-status'), results.childElementCount ? '' : 'Открытые каналы не найдены');
    } catch (error) {
      if (revision === browseRevision) setMsg(el('browse-status'), errorText(error), 'err');
    }
  }

  el('call')?.addEventListener('click', startCall);
  el('browse')?.addEventListener('click', () => {
    el('browse-query').value = '';
    el('browse-dialog').showModal();
    void browseChannels();
  });
  el('browse-dialog')?.addEventListener('close', () => { browseRevision++; });
  el('browse-form')?.addEventListener('submit', event => {
    event.preventDefault();
    void browseChannels();
  });
  el('browse-more')?.addEventListener('click', () => browseChannels(true));
  el('filter').addEventListener('input',renderChannels);
  el('refresh').onclick = refresh; el('new').onclick = () => people(); el('members').onclick = members;
  el('search').oninput = () => { clearTimeout(searchTimer); searchRevision++; searchTimer = setTimeout(searchPeople,200); };
  el('people').addEventListener('close',() => { searchRevision++; clearTimeout(searchTimer); peopleAfterOpen=null;el('search').hidden = false; });
  el('send').onsubmit = send;
  el('body').addEventListener('keydown',event => { if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) { event.preventDefault(); el('send').requestSubmit(); } });
  el('reply-cancel').onclick = () => { reply = ''; el('reply').classList.add('hidden'); clearAttachments(); };
  el('create').onclick = async () => { if (await ensure()) { el('create-name').value = ''; setMsg(el('create-error'),''); el('create-dialog').showModal(); } };
  el('create-cancel').onclick = () => el('create-dialog').close();
  el('create-form').onsubmit = async event => {
    event.preventDefault(); const button = event.submitter; button.disabled = true;
    try { const channel = await request('/channels',{method:'POST',body:JSON.stringify({team_id:team.id,name:'channel-' + crypto.randomUUID(),display_name:el('create-name').value.trim(),type:el('create-type').value})}); el('create-dialog').close(); await syncChannels(); await open(channel.id); }
    catch (error) { setMsg(el('create-error'),errorText(error),'err'); } finally { button.disabled = false; }
  };
  el('reminders')?.addEventListener('click', async () => {
    const dialog = el('reminders-dialog'), status = el('reminders-status'), toggle = el('reminders-toggle');
    if (dialog.open) return;
    const check = guard();
    let enabled = false;
    function display(info) {
      enabled = info.enabled;
      toggle.hidden = !info.available && !enabled;
      toggle.textContent = enabled ? 'Выключить' : 'Включить';
      setMsg(status, info.available ? (enabled ? 'Личные напоминания включены.' : 'Личные напоминания выключены.') : 'Администратор домена выключил эту функцию.');
    }
    toggle.hidden = true;
    setMsg(status, 'Загрузка…');
    dialog.showModal();
    const onLogout = new AbortController();
    window.addEventListener('tms-products-session-ended', () => dialog.close(), {signal:onLogout.signal});
    dialog.addEventListener('close', () => { onLogout.abort(); toggle.onclick = null; }, {once:true});
    try {
      const info = await request('/tms/calendar-reminders');
      check(); if (!dialog.open) return;
      display(info);
      toggle.onclick = async () => {
        toggle.disabled = true;
        try {
          const result = await request('/tms/calendar-reminders', {method:'PUT', body:JSON.stringify({enabled:!enabled})});
          check(); if (dialog.open) display(result);
        } catch(error) { if (dialog.open && current()) setMsg(status,errorText(error),'err'); }
        finally { toggle.disabled = false; }
      };
    } catch(error) { if (dialog.open && current()) setMsg(status,errorText(error),'err'); }
  });
  window.addEventListener('focus',() => { if (current() && me) { void syncChannels().then(() => selected ? loadHistory(true) : null).catch(report); } });
  document.addEventListener('visibilitychange',() => { if (!document.hidden && current() && me) { void syncChannels().then(() => selected ? loadHistory(true) : null).catch(report); } });
  return {start,stop,refresh,openPeer,discussMail};
})();
