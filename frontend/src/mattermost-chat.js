/* Shared TMS/Mattermost chat. No XMPP requests or tokens in WebSocket URLs. */
const mmChat = (() => {
  let generation = 0, owner = '', boot = null, socket = null, reconnect = null, heartbeat = null, poll = null;
  let me = null, team = null, channels = [], users = new Map(), memberships = new Map();
  let selected = '', posts = new Map(), page = 0, complete = false, historyBusy = false, historyRevision = 0;
  let sending = false, reply = '', pending = null, peopleMode = 'direct', peopleChannel = '', searchRevision = 0, searchTimer = null;
  let refreshing = null, viewBusy = false;
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
  const visible = () => !document.hidden && !el('thread').closest('#app-chat').classList.contains('hidden') && document.hasFocus();
  function connection(text) { el('connection').textContent = text; }
  function stop() {
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
        el('self').textContent = label(me);
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
        const name = document.createElement('span'); name.textContent = (channel.type === 'D' ? '' : channel.type === 'P' ? '🔒 ' : '# ') + channelName(channel); button.append(name);
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
    box.replaceChildren();
    if (!complete) { const older = document.createElement('button'); older.type = 'button'; older.className = 'btn-secondary btn-sm'; older.textContent = 'Загрузить более ранние'; older.onclick = () => loadHistory(); box.append(older); }
    const ordered = [...posts.values()].sort((a,b) => a.create_at - b.create_at || a.id.localeCompare(b.id));
    for (const post of ordered) {
      const article = document.createElement('article'); article.className = 'chat-bubble' + (post.user_id === me.id ? ' is-mine' : ''); article.dataset.postId = post.id;
      const author = document.createElement('strong'); author.textContent = label(users.get(post.user_id)); article.append(author);
      if (post.root_id) { const root = posts.get(post.root_id), quote = document.createElement('div'); quote.className = 'mm-chat-quote'; quote.textContent = 'Ответ: ' + (root?.message?.slice(0,160) || 'сообщение в ветке'); article.append(quote); }
      const text = document.createElement('p');
      const callURL = meetingURL(post.props?.tms_meeting_url);
      if (callURL) {
        const link = document.createElement('a');
        link.href = callURL;
        link.textContent = 'Присоединиться к звонку Jitsi';
        text.append(link);
      } else {
        text.textContent = post.message || (post.file_ids?.length ? 'Вложение' : '');
      }
      article.append(text);
      const time = document.createElement('time'); time.dateTime = new Date(post.create_at).toISOString(); time.textContent = new Date(post.create_at).toLocaleString(lang === 'en' ? 'en-GB' : 'ru-RU'); article.append(time);
      const answer = document.createElement('button'); answer.type = 'button'; answer.className = 'btn-secondary btn-sm mm-chat-answer'; answer.textContent = 'Ответить'; answer.onclick = () => { reply = post.root_id || post.id; el('reply-label').textContent = 'Ответ: ' + post.message.slice(0,120); el('reply').classList.remove('hidden'); el('body').focus(); }; article.append(answer);
      box.append(article);
    }
    if (!ordered.length) { const hint = document.createElement('p'); hint.className = 'meta'; hint.textContent = 'Сообщений пока нет'; box.append(hint); }
    if (keepScroll) box.scrollTop = previousTop + box.scrollHeight - previousHeight;
    else if (nearBottom || previousHeight <= box.clientHeight) box.scrollTop = box.scrollHeight;
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
    if (!body || !id || sending || !me) return;
    const version = generation; sending = true; const button = el('send').querySelector('[type=submit]'); button.disabled = true; setMsg(el('send-error'),'');
    if (!pending || pending.channel_id !== id || pending.message !== body || pending.root_id !== root) pending = {channel_id:id,message:body,root_id:root,pending_post_id:me.id + ':' + crypto.randomUUID()};
    try {
      const submission = pending;
      const post = await request('/posts',{method:'POST',body:JSON.stringify(submission)});
      if (selected === id) { posts.set(post.id,post); renderPosts(); el('posts').scrollTop = el('posts').scrollHeight; if (input.value.trim() === body) input.value = ''; reply = ''; el('reply').classList.add('hidden'); }
      if (pending === submission) pending = null; await syncChannels(); void markViewed();
    } catch (error) { if (generation === version) setMsg(el('send-error'),errorText(error),'err'); }
    finally { if (generation === version) { sending = false; button.disabled = false; } }
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
        const name = document.createElement('strong'); name.textContent = label(user); const username = document.createElement('span'); username.className = 'meta'; username.textContent = '@' + user.username; button.append(name,username);
        button.onclick = async () => {
          button.disabled = true;
          try {
            if (peopleMode === 'member') { await request('/channels/' + peopleChannel + '/members',{method:'POST',body:JSON.stringify({user_id:user.id})}); el('people').close(); }
            else { const channel = await request('/channels/direct',{method:'POST',body:JSON.stringify([me.id,user.id])}); el('people').close(); await syncChannels(); await open(channel.id); }
          } catch (error) { setMsg(el('people-error'),errorText(error),'err'); button.disabled = false; }
        };
        el('people-list').append(button);
      }
      if (!list.length) { const hint = document.createElement('p'); hint.className = 'meta'; hint.textContent = 'Пользователи не найдены'; el('people-list').append(hint); }
    } catch (error) { if (revision === searchRevision && current()) setMsg(el('people-error'),errorText(error),'err'); }
  }
  async function people(mode = 'direct') {
    if (!await ensure()) return;
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
      for (const member of list) { const row = document.createElement('p'); row.textContent = label(users.get(member.user_id)); el('people-list').append(row); }
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
  el('people').addEventListener('close',() => { searchRevision++; clearTimeout(searchTimer); el('search').hidden = false; });
  el('send').onsubmit = send;
  el('body').addEventListener('keydown',event => { if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) { event.preventDefault(); el('send').requestSubmit(); } });
  el('reply-cancel').onclick = () => { reply = ''; el('reply').classList.add('hidden'); };
  el('create').onclick = async () => { if (await ensure()) { el('create-name').value = ''; setMsg(el('create-error'),''); el('create-dialog').showModal(); } };
  el('create-cancel').onclick = () => el('create-dialog').close();
  el('create-form').onsubmit = async event => {
    event.preventDefault(); const button = event.submitter; button.disabled = true;
    try { const channel = await request('/channels',{method:'POST',body:JSON.stringify({team_id:team.id,name:'channel-' + crypto.randomUUID(),display_name:el('create-name').value.trim(),type:el('create-type').value})}); el('create-dialog').close(); await syncChannels(); await open(channel.id); }
    catch (error) { setMsg(el('create-error'),errorText(error),'err'); } finally { button.disabled = false; }
  };
  window.addEventListener('focus',() => { if (current() && me) { void syncChannels().then(() => selected ? loadHistory(true) : null).catch(report); } });
  document.addEventListener('visibilitychange',() => { if (!document.hidden && current() && me) { void syncChannels().then(() => selected ? loadHistory(true) : null).catch(report); } });
  return {start,stop,refresh,openPeer};
})();
