/* Scoped Mattermost administration; all changes retain their selected scope. */
const mmAdmin = { scopes: [], selected: null, value: null, epoch: 0, account: "" };
function mmQuery(scope) { const query = new URLSearchParams(); if (scope.tenant_id) query.set('tenant_id', scope.tenant_id); if (scope.domain_id) query.set('domain_id', scope.domain_id); return query.size ? '?' + query : ''; }
function mmBool(value) { return value === undefined || value === null ? 'inherit' : value ? 'on' : 'off'; }
function mmCurrentGuard(epoch, account) { return () => { if (mmAdmin.epoch !== epoch || state.me?.id !== account) throw new Error('Область настроек изменилась'); }; }
async function refreshMattermostAdmin() {
  const epoch = ++mmAdmin.epoch, account = state.me?.id; const guard = mmCurrentGuard(epoch, account);
  setMsg($('mm-msg'), '');
  try {
    const result = await api('/api/v1/admin/mattermost/scopes', {_guard: guard});
    mmAdmin.scopes = result.scopes || [];
    const previous = JSON.stringify(mmAdmin.selected);
    $('mm-scope').innerHTML = mmAdmin.scopes.map((item, index) => `<option value="${index}" ${JSON.stringify(item.scope) === previous ? 'selected' : ''}>${escapeHtml(item.label)}</option>`).join('');
    if (!mmAdmin.scopes.length) { setMsg($('mm-msg'), 'Нет доступных областей настроек', 'err'); return; }
    mmAdmin.account = account;
    await loadMattermostScope();
  } catch (error) { if (mmAdmin.epoch === epoch) setMsg($('mm-msg'), error.message, 'err'); }
}
async function loadMattermostScope() {
  const item = mmAdmin.scopes[Number($('mm-scope').value)]; if (!item) return;
  mmFed.epoch++; mmFedHost()?.replaceChildren(); mmAdmin.selected = {...item.scope}; const epoch = ++mmAdmin.epoch, guard = mmCurrentGuard(epoch, state.me?.id);
  $('mm-plugin-editor')?.close();
  $('mm-policy-form').inert = true; $('mm-plugin-upload').disabled = true;
  try {
    const value = await api('/api/v1/admin/mattermost' + mmQuery(item.scope), {_guard: guard});
    mmAdmin.value = value; renderMattermostScope(value); void loadMattermostBots(); void loadMattermostFederation(); setMsg($('mm-msg'), '');
  } catch (error) { if (mmAdmin.epoch === epoch) setMsg($('mm-msg'), error.message, 'err'); }
  finally { if (mmAdmin.epoch === epoch) $('mm-policy-form').inert = false; }
}
function renderMattermostScope(value) {
  const own = value.overrides || {}, effective = value.effective || {};
  $('mm-enabled').value = mmBool(own.enabled); $('mm-plugins-enabled').value = mmBool(own.plugins_enabled);
  $('mm-calendar-reminders').value = mmBool(own.calendar_reminders);
  $('mm-plugin-installer').hidden = !value.global_admin;
  $('mm-name-inherit').checked = own.site_name === undefined;
  $('mm-name').value = own.site_name ?? effective.site_name ?? ''; $('mm-name').disabled = $('mm-name-inherit').checked;
  $('mm-plugin-config').value = JSON.stringify(own.plugins || {}, null, 2);
  $('mm-host-settings').classList.toggle('hidden', !value.global_admin);
  $('mm-plugin-upload').disabled = !value.global_admin || !value.plugin_host_enabled;
  $('mm-effective').textContent = `Итог: ${effective.site_name || 'Mattermost'} · доступ ${effective.enabled ? 'включён' : 'выключен'} · плагины ${effective.plugins_enabled ? 'включены' : 'выключены'}` + (!value.host_enabled ? '. Серверный API выключен — включите его в настройках запуска и перезапустите службу.' : !value.plugin_host_enabled ? '. Хост плагинов выключен — для запуска нужен перезапуск с включённым хостом.' : '');
  $('mm-push-enabled').value = mmBool(own.push?.enabled);
  $('mm-push-inherit').checked = own.push?.proxy_url === undefined;
  $('mm-push-url').value = own.push?.proxy_url ?? effective.push?.proxy_url ?? '';
  $('mm-push-url').disabled = $('mm-push-inherit').checked;
  $('mm-push-status').textContent = effective.push?.enabled ? 'Автоматическая отправка включена. Без текста сообщений.' : effective.push?.prepared ? 'TPNS подготовлен. Автоматическая отправка выключена.' : 'Тестовый шлюз не подготовлен.';
  const email = own.email || {};
  for (const [field,id] of [['incoming_address','mm-email-incoming'],['incoming_channel_id','mm-email-channel'],['outgoing_from','mm-email-from']]) {
    $(id).value = email[field] || ''; $(id).disabled = !value.scope?.domain_id;
  }
  $('mm-email-prefix-inherit').checked = email.subject_prefix === undefined;
  $('mm-email-prefix').value = email.subject_prefix ?? effective.email?.subject_prefix ?? '[TMS]';
  $('mm-email-prefix').disabled = $('mm-email-prefix-inherit').checked;
  const catalog = value.catalog || {}, manifests = [...(catalog.active || []), ...(catalog.inactive || [])];
  const seen = new Set(); const items = manifests.filter(item => { if (seen.has(item.id)) return false; seen.add(item.id); return true; });
  $('mm-plugin-list').innerHTML = items.map(item => {
    const enabled = effective.plugins?.[item.id]?.enabled === true;
    const domain = !!value.scope?.domain_id;
    const available = domain && value.plugin_host_enabled;
    return `<div class="config-collection-item"><strong>${escapeHtml(item.name || item.id)}</strong>
      <p class="meta">${escapeHtml(item.id)} · ${escapeHtml(item.version || '')}${domain ? (enabled ? ' · Разрешён' : ' · Выключен') : ''}</p>
      <div class="actions" ${domain ? '' : 'hidden'}>
        <button type="button" class="btn-secondary" data-mm-config="${escapeHtml(item.id)}" ${available ? '' : 'disabled'}>Настроить</button>
        <button type="button" class="btn-secondary" data-mm-plugin="${escapeHtml(item.id)}" data-mm-action="${enabled ? 'disable' : 'enable'}" ${available && effective.enabled && effective.plugins_enabled ? '' : 'disabled'}>${enabled ? 'Отключить в домене' : 'Запустить в домене'}</button>
      </div></div>`;
  }).join('') || '<p class="meta">Пакеты ещё не установлены. Наличие пакета не подтверждает совместимость всех его функций.</p>';
  $('mm-plugin-list').querySelectorAll('[data-mm-config]').forEach(button => {
    button.onclick = () => {
      const item = items.find(manifest => manifest.id === button.dataset.mmConfig);
      if (item) editMattermostPlugin(item);
    };
  });
  $('mm-plugin-list').querySelectorAll('[data-mm-plugin]').forEach(button => button.addEventListener('click', async () => {
    const scope = {...mmAdmin.selected}, epoch = mmAdmin.epoch, guard = mmCurrentGuard(epoch, state.me?.id); button.disabled = true;
    try { await api('/api/v4/plugins/' + encodeURIComponent(button.dataset.mmPlugin) + '/' + button.dataset.mmAction + mmQuery(scope), {method: 'POST', body: '{}', _guard: guard}); guard(); await loadMattermostScope(); }
    catch (error) { if (mmAdmin.epoch === epoch) { setMsg($('mm-msg'), error.data?.message || error.message, 'err'); button.disabled = false; } }
  }));
}
function editMattermostPlugin(manifest) {
  if (!mmAdmin.selected?.domain_id || $('mm-plugin-editor')?.open) return;
  const scope = {...mmAdmin.selected}, epoch = mmAdmin.epoch, guard = mmCurrentGuard(epoch, state.me?.id);
  const own = mmAdmin.value.overrides?.plugins?.[manifest.id]?.configuration || {};
  const effective = mmAdmin.value.effective?.plugins?.[manifest.id]?.configuration || {};
  const schema = manifest.settings_schema || {};
  const fields = [...(schema.settings || []), ...(schema.sections || []).flatMap(section => section?.settings || [])];
  const dialog = document.createElement('dialog'); dialog.id = 'mm-plugin-editor'; dialog.className = 'mm-chat-dialog';
  const form = document.createElement('form');
  const add = (parent, tag, text, className) => {
    const node = document.createElement(tag); node.textContent = text || '';
    if (className) node.className = className;
    parent.append(node); return node;
  };
  add(form, 'h3', manifest.name || manifest.id, 'heading');
  add(form, 'p', $('mm-scope').selectedOptions[0]?.textContent || '', 'meta');
  if (schema.header) add(form, 'p', schema.header, 'field-hint');
  const edits = new Map();
  const supported = new Set(['text', 'longtext', 'number', 'bool', 'dropdown', 'radio', 'username', 'generated']);
  for (const field of fields) {
    if (!field?.key || !supported.has(field.type) || field.hosting === 'cloud' || edits.has(field.key)) continue;
    const key = field.key, current = effective[key] ?? field.default, owned = Object.hasOwn(own, key);
    const secret = field.secret || field.type === 'generated' || current === '***';
    const row = add(form, 'div', '', 'config-collection-item');
    const label = add(row, 'label', field.display_name || key, 'field-label');
    let input;
    if (!secret && ['dropdown', 'radio', 'bool'].includes(field.type)) {
      input = add(label, 'select', '', 'field-input');
      const options = field.type === 'bool' ? [{display_name:'Да',value:'true'},{display_name:'Нет',value:'false'}] : field.options || [];
      for (const option of options) if (option) input.add(new Option(option.display_name, option.value));
      if (current != null && !Array.from(input.options).some(option => option.value === String(current))) {
        input.add(new Option(String(current), String(current)));
      }
    } else {
      input = add(label, field.type === 'longtext' && !secret ? 'textarea' : 'input', '', 'field-input');
      if (secret) { input.type = 'password'; input.autocomplete = 'new-password'; }
      else if (field.type === 'number') { input.type = 'number'; input.step = '1'; }
      input.placeholder = secret && current ? 'Значение задано' : field.placeholder || '';
    }
    if (!secret && current != null) input.value = String(current);
    const inheritLabel = add(row, 'label', ' Переопределить в этом домене', 'field-check');
    const override = document.createElement('input'); override.type = 'checkbox'; override.checked = owned;
    inheritLabel.prepend(override); input.disabled = !owned;
    override.onchange = () => { input.disabled = !override.checked; };
    if (field.help_text) add(row, 'p', field.help_text, 'field-hint');
    edits.set(key, {input, override, secret, type: field.type, owned, previous: input.value});
  }
  if (!edits.size) add(form, 'p', 'Пакет не описывает поля настройки. Используйте JSON параметров области по инструкции к плагину.', 'meta');
  const status = add(form, 'p', '', 'msg'); status.setAttribute('role', 'status');
  const actions = add(form, 'div', '', 'actions');
  const cancel = add(actions, 'button', 'Закрыть', 'btn-secondary'); cancel.type = 'button'; cancel.onclick = () => dialog.close();
  const submit = add(actions, 'button', 'Сохранить настройки', 'btn-primary'); submit.type = 'submit'; submit.hidden = !edits.size;
  dialog.append(form); document.body.append(dialog); dialog.showModal();
  const onLogout = new AbortController();
  window.addEventListener('tms-products-session-ended', () => dialog.close(), {signal:onLogout.signal});
  dialog.addEventListener('close', () => {onLogout.abort(); dialog.remove();}, {once:true});
  form.onsubmit = async event => {
    event.preventDefault();
    if (submit.disabled || submit.hidden) return;
    const set = Object.create(null), inherit = [];
    try {
      guard();
      for (const [key, field] of edits) {
        const {input, override, type, secret, owned, previous} = field;
        if (!override.checked) { if (owned) inherit.push(key); continue; }
        if ((secret && !input.value) || (owned && input.value === previous)) continue;
        if (type === 'number' && (!input.value || !Number.isSafeInteger(Number(input.value)))) throw new Error('Введите целое число');
        set[key] = type === 'bool' ? input.value === 'true' : type === 'number' ? Number(input.value) : input.value;
      }
      if (!Object.keys(set).length && !inherit.length) { dialog.close(); return; }
      submit.disabled = true;
      await api('/api/v4/plugins/'+encodeURIComponent(manifest.id)+'/configuration'+mmQuery(scope), {
        method:'PATCH', body:JSON.stringify({set, inherit}), _guard:guard,
      });
      dialog.close(); await loadMattermostScope();
    } catch (error) {
      if (dialog.open && mmAdmin.epoch === epoch) setMsg(status, error.data?.message || error.message, 'err');
    } finally { submit.disabled = false; }
  };
}
$('mm-push-inherit')?.addEventListener('change', () => { $('mm-push-url').disabled = $('mm-push-inherit').checked; });
$('mm-scope')?.addEventListener('change', loadMattermostScope);
$('mm-refresh')?.addEventListener('click', refreshMattermostAdmin);
$('mm-email-prefix-inherit')?.addEventListener('change', () => { $('mm-email-prefix').disabled = $('mm-email-prefix-inherit').checked; });
$('mm-name-inherit')?.addEventListener('change', () => { $('mm-name').disabled = $('mm-name-inherit').checked; });
$('mm-host-settings')?.addEventListener('click', async () => {
  try { settingsState.all = await api('/api/v1/admin/settings'); await openAdminConfig('mattermost'); }
  catch (error) { setMsg($('mm-msg'), error.message, 'err'); }
});
async function saveMattermostPolicy(value) {
  const scope = {...mmAdmin.selected}, epoch = mmAdmin.epoch, guard = mmCurrentGuard(epoch, state.me?.id);
  $('mm-policy-form').inert = true;
  try { const result = await api('/api/v1/admin/mattermost' + mmQuery(scope), {method: 'PUT', body: JSON.stringify(value), _guard: guard}); mmAdmin.value = result; renderMattermostScope(result); setMsg($('mm-msg'), 'Настройки выбранной области сохранены', 'ok'); }
  catch (error) { if (mmAdmin.epoch === epoch) setMsg($('mm-msg'), error.message, 'err'); }
  finally { if (mmAdmin.epoch === epoch) $('mm-policy-form').inert = false; }
}
$('mm-policy-form')?.addEventListener('submit', async event => {
  event.preventDefault(); const value = {};
  const email = {};
  if (mmAdmin.selected?.domain_id) {
    for (const [field,id] of [['incoming_address','mm-email-incoming'],['incoming_channel_id','mm-email-channel'],['outgoing_from','mm-email-from']]) email[field] = $(id).value.trim();
  }
  if (!$('mm-email-prefix-inherit').checked) email.subject_prefix = $('mm-email-prefix').value;
  value.email = email;
  value.push = {};
  if (!$('mm-push-inherit').checked) value.push.proxy_url = $('mm-push-url').value;
  if ($('mm-push-enabled').value !== 'inherit') value.push.enabled = $('mm-push-enabled').value === 'on';
  for (const [field, id] of [['enabled','mm-enabled'], ['plugins_enabled','mm-plugins-enabled'], ['calendar_reminders','mm-calendar-reminders']]) if ($(id).value !== 'inherit') value[field] = $(id).value === 'on';
  if (!$('mm-name-inherit').checked) { value.site_name = $('mm-name').value.trim(); if (!value.site_name) { setMsg($('mm-msg'), 'Укажите название или включите наследование', 'err'); return; } }
  try { value.plugins = JSON.parse($('mm-plugin-config').value || '{}'); if (!value.plugins || Array.isArray(value.plugins) || typeof value.plugins !== 'object') throw new Error(); }
  catch { setMsg($('mm-msg'), 'Параметры плагинов должны быть JSON-объектом', 'err'); return; }
  await saveMattermostPolicy(value);
});
$('mm-reset')?.addEventListener('click', async () => { if (await askConfirm('Удалить переопределения только выбранной области и наследовать настройки родителя?')) await saveMattermostPolicy({}); });
$('mm-plugin-upload')?.addEventListener('change', async event => {
  const input = event.target, file = input.files?.[0]; if (!file) return;
  const epoch = mmAdmin.epoch, guard = mmCurrentGuard(epoch, state.me?.id), scope = {...mmAdmin.selected};
  input.disabled = true;
  try {
    if (file.size > 100 * 1024 * 1024) throw new Error('Пакет превышает 100 МиБ');
    await api('/api/v1/me', {_guard: guard}); guard(); const form = new FormData(); form.append('plugin', file);
    const response = await fetch('/api/v4/plugins' + mmQuery(scope), {method: 'POST', headers: {Authorization: 'Bearer ' + state.tokens.access_token}, body: form});
    const result = await response.json(); guard(); if (!response.ok) throw new Error(result.message || 'Не удалось установить пакет');
    await loadMattermostScope(); setMsg($('mm-msg'), 'Пакет установлен. Настройте и запустите его в нужном домене.', 'ok');
  } catch (error) { if (mmAdmin.epoch === epoch) setMsg($('mm-msg'), error.message, 'err'); }
  finally { input.value = ''; if (mmAdmin.epoch === epoch) input.disabled = !mmAdmin.value?.global_admin; }
});

// External bot credentials are returned once and never stored in browser storage.
async function loadMattermostBots() {
 const host=$('mm-external-bots');if(!host)return;
 host.replaceChildren();$('mm-bot-secret')?.close();
 const epoch=mmAdmin.epoch,guard=mmCurrentGuard(epoch,state.me?.id),scope={...mmAdmin.selected};
 $('mm-bot-create').hidden=!scope.domain_id;
 if(!scope.domain_id){host.textContent='Выберите домен в списке областей, чтобы управлять его ботами.';return}
 try {
  const bots=await api('/api/v4/bots'+mmQuery(scope),{_guard:guard});
  for(const bot of bots){
   const row=document.createElement('div');row.className='mm-bot-row';
   const text=document.createElement('div'),title=document.createElement('strong'),meta=document.createElement('p');title.textContent=bot.display_name;meta.className='meta';meta.textContent='@'+bot.username+' · '+(bot.enabled?'Включён':'Выключен');text.append(title,meta);row.append(text);
   for(const [action,label]of [['token','Новый токен'],[bot.enabled?'disable':'enable',bot.enabled?'Отключить':'Включить']]){
    const button=document.createElement('button');button.type='button';button.className='btn-secondary';button.textContent=label;
    button.onclick=async()=>{if(action==='token'&&!await askConfirm('Выпустить новый токен? Старый токен сразу перестанет работать.'))return;button.disabled=true;try{guard();const result=await api('/api/v4/bots/'+bot.user_id+'/'+action+mmQuery(scope),{method:'POST',body:'{}',_guard:guard});await loadMattermostBots();guard();if(result.token)showBotCredential(result)}catch(e){if(mmAdmin.epoch===epoch)setMsg($('mm-msg'),e.data?.message||e.message,'err')}finally{button.disabled=false}};
    row.append(button);
   }
   host.append(row);
  }
  if(!bots.length)host.textContent='Ботов пока нет. Создайте отдельную учётную запись для вашей интеграции.';
 }catch(e){if(mmAdmin.epoch===epoch)host.textContent=e.data?.message||e.message}
}
function showBotCredential(bot){
 const dialog=$('mm-bot-secret');dialog.querySelector('input').value=bot.token;dialog.querySelector('[data-bot-id]').textContent=bot.user_id;dialog.showModal();
}
function setupMattermostBots(){
 const section=document.createElement('section');section.id='mm-bots-panel';section.className='mm-bots-panel';section.hidden=true;
 section.innerHTML='<h3>Боты Mattermost</h3><p class="field-hint">Подключите внешнего бота через API v4 и WebSocket. Создайте бота, скопируйте токен и добавьте его в нужный канал через меню участников чата.</p><form id="mm-bot-create" class="mm-bot-create"><label>Логин бота<input class="field-input" name="username" placeholder="notify-bot" pattern="[a-z0-9][a-z0-9._-]{1,63}" required maxlength="64" autocomplete="off"></label><label>Отображаемое имя<input class="field-input" name="display_name" placeholder="Уведомления" required maxlength="128"></label><button type="submit" class="btn-spray">Создать бота</button></form><p class="field-hint">Бот не получает права администратора. Он видит переписку каналов, в которые добавлен. Токен действует только для чата; его можно заменить или отключить бота.</p><div id="mm-external-bots" role="status"></div><details class="workspace-mm-help"><summary>Вебхуки и подключение</summary><p>Для односторонних уведомлений доступен API входящих вебхуков: POST /api/v4/hooks/incoming. Для двустороннего бота используйте выданный токен в заголовке Authorization: Bearer и подключение /api/v4/websocket.</p><p>Адрес сервера: <code id="mm-bot-origin"></code>. Бот запускается в вашей инфраструктуре; добавление учётной записи не запускает его программу.</p></details>';
 $('app-mattermost').querySelector('.card').append(section);$('mm-bot-origin').textContent=location.origin;
 const dialog=document.createElement('dialog');dialog.id='mm-bot-secret';dialog.className='mm-chat-dialog';dialog.innerHTML='<h3>Токен подключения</h3><p>Скопируйте токен сейчас: повторно он не показывается.</p><label>Токен<input class="field-input" readonly autocomplete="off" spellcheck="false"></label><p class="meta">ID бота: <code data-bot-id></code></p><div class="actions"><button type="button" class="btn-secondary" data-copy>Скопировать</button><form method="dialog"><button class="btn-spray">Готово</button></form></div>';document.body.append(dialog);
 dialog.addEventListener('close',()=>{dialog.querySelector('input').value='';dialog.querySelector('[data-bot-id]').textContent='';dialog.querySelector('[data-copy]').textContent='Скопировать'});
 dialog.querySelector('[data-copy]').onclick=async()=>{try{await navigator.clipboard.writeText(dialog.querySelector('input').value);dialog.querySelector('[data-copy]').textContent='Скопировано'}catch{dialog.querySelector('input').select()}};
 $('mm-bot-create').onsubmit=async event=>{event.preventDefault();const epoch=mmAdmin.epoch,guard=mmCurrentGuard(epoch,state.me?.id),scope={...mmAdmin.selected},form=event.currentTarget;const data=Object.fromEntries(new FormData(form));form.inert=true;try{const bot=await api('/api/v4/bots'+mmQuery(scope),{method:'POST',body:JSON.stringify(data),_guard:guard});form.reset();await loadMattermostBots();guard();showBotCredential(bot)}catch(e){if(mmAdmin.epoch===epoch)setMsg($('mm-msg'),e.data?.message||e.message,'err')}finally{form.inert=false}};
}
setupMattermostBots();

function resetMattermostBots(){
 mmAdmin.epoch++;
 $('mm-plugin-editor')?.close();
 const dialog=$('mm-bot-secret');
 if(dialog){dialog.querySelector('input').value='';dialog.querySelector('[data-bot-id]').textContent='';dialog.close();}
 $('mm-external-bots')?.replaceChildren();
 $('mm-bot-create')?.reset();
}


/* Federation shares only channels explicitly selected in the current domain. */
const mmFed = { peers: [], selected: '', epoch: 0 };
function mmFedHost() {
  let host = $('mm-federation');
  if (!host) { host = document.createElement('section'); host.id='mm-federation'; host.className='card mt-4'; $('app-mattermost')?.append(host); }
  return host;
}
async function loadMattermostFederation() {
  const host=mmFedHost(), scope={...mmAdmin.selected}, epoch=++mmFed.epoch, adminEpoch=mmAdmin.epoch, account=state.me?.id;
  const guard=()=>{ if(epoch!==mmFed.epoch)throw new Error('Область изменилась'); mmCurrentGuard(adminEpoch,account)(); };
  host.replaceChildren();
  if(!scope.domain_id){host.textContent='Межсерверная связь: выберите домен в области настроек.';return;}
  try {
    const data=await api('/api/v1/admin/mattermost/federation'+mmQuery(scope),{_guard:guard});mmFed.peers=data.peers||[];
    host.innerHTML=`<h3>Межсерверная связь Mattermost</h3><p>Каждая связь относится только к выбранному домену. В обмен входят только явно подключённые каналы. REST-связь уже двусторонняя: не создавайте встречный мост для той же пары каналов.</p>
      <div id="mm-fed-list"></div><details><summary>Добавить сервер</summary><form id="mm-fed-create" class="grid gap-2 mt-3">
      <label>Название связи<input name="name" required maxlength="64" autocomplete="off"></label>
      <label>Режим<select name="mode"><option value="native">Нативные общие каналы</option><option value="rest">Мост через бота API v4</option></select></label>
      <label>HTTPS-адрес сервера<input name="site_url" type="url" placeholder="https://chat.example.org"></label>
      <label>ID локальной учётной записи для доступа к каналам<input name="owner_id" autocomplete="off" placeholder="По умолчанию — текущий администратор"></label>
      <label>Токен бота (только для API v4)<input name="access_token" type="password" autocomplete="new-password"></label>
      <label>Пароль приглашения (для нативного режима)<input name="password" type="password" autocomplete="new-password"></label>
      <label>Код полученного приглашения (оставьте пустым, чтобы создать своё)<textarea name="invite_code" rows="3" autocomplete="off"></textarea></label>
      <button type="submit" class="btn">Добавить связь</button></form></details>
      <div id="mm-fed-result" role="status"></div><button id="mm-fed-refresh" class="btn btn-ghost mt-2">Обновить состояние</button><div id="mm-fed-links"></div>`;
    const list=$('mm-fed-list');
    for(const p of mmFed.peers){
      const row=document.createElement('div');row.className='flex gap-2 flex-wrap items-center mt-3';
      const text=document.createElement('span');text.textContent=`${p.name} · ${p.mode==='native'?'Нативная связь':'Мост API v4'} · ${!p.enabled?'Отключена':p.confirmed?'Подтверждена':'Ожидает подтверждения'}${p.last_error?' · Обмен отложен':''}`;row.append(text);
      const open=document.createElement('button');open.className='btn btn-ghost';open.textContent='Каналы';open.onclick=()=>loadMattermostFederationLinks(p,scope,guard);row.append(open);
      if(p.enabled){const off=document.createElement('button');off.className='btn btn-ghost';off.textContent='Отключить связь';off.onclick=async()=>{try{await api('/api/v1/admin/mattermost/federation/'+encodeURIComponent(p.id)+mmQuery(scope),{method:'DELETE',_guard:guard});await loadMattermostFederation();}catch(e){if(epoch===mmFed.epoch)setMsg($('mm-fed-result'),e.message,'err');}};row.append(off);}
      list.append(row);
    }
    $('mm-fed-refresh').onclick=loadMattermostFederation;
    $('mm-fed-create').onsubmit=async event=>{
      event.preventDefault();const form=event.currentTarget,values=Object.fromEntries(new FormData(form));const button=form.querySelector('button');button.disabled=true;
      try {const result=await api('/api/v1/admin/mattermost/federation'+mmQuery(scope),{method:'POST',body:JSON.stringify(values),_guard:guard});form.reset();await loadMattermostFederation();if(result.invite_code){const output=document.createElement('textarea');output.readOnly=true;output.rows=6;output.value=result.invite_code;const label=document.createElement('p');label.textContent='Передайте этот код администратору второго сервера. Пароль передайте отдельно. Код исчезнет при смене области или обновлении страницы.';$('mm-fed-result').replaceChildren(label,output);}}
      catch(e){if(epoch===mmFed.epoch)setMsg($('mm-fed-result'),e.message,'err');}finally{button.disabled=false;}
    };
  }catch(e){if(epoch===mmFed.epoch)host.textContent='Межсерверная связь недоступна: '+e.message;}
}
async function loadMattermostFederationLinks(peer,scope,guard) {
  try {
    const data=await api('/api/v1/admin/mattermost/federation/'+encodeURIComponent(peer.id)+'/links'+mmQuery(scope),{_guard:guard});
    const host=$('mm-fed-links');host.innerHTML=`<h4 class="mt-4">Каналы: ${escapeHtml(peer.name)}</h4><div id="mm-fed-link-list"></div><form id="mm-fed-link-form" class="grid gap-2 mt-3"><label>Локальный канал<select name="channel_id" required>${(data.channels||[]).map(c=>`<option value="${escapeHtml(c.id)}">${escapeHtml(c.display_name||c.name)}</option>`).join('')}</select></label><label>ID канала второго сервера<input name="remote_channel_id" placeholder="В нативном режиме: пусто для нового канала"></label><label><input name="readonly" type="checkbox"> Только отправка на второй сервер</label><label><input name="backfill" type="checkbox"> Включить предыдущую историю</label><button class="btn" type="submit">Разрешить обмен каналом</button></form><div id="mm-fed-link-msg" role="status"></div>`;
    for(const l of data.links||[]){const row=document.createElement('div');row.className='mt-2';row.textContent=`${l.channel_id} ↔ ${l.remote_channel_id} · ${l.enabled?'Включён':'Отключён'} `;if(l.enabled){const off=document.createElement('button');off.className='btn btn-ghost';off.textContent='Отключить';off.onclick=async()=>{try{await api('/api/v1/admin/mattermost/federation/'+encodeURIComponent(peer.id)+'/links/'+encodeURIComponent(l.id)+mmQuery(scope),{method:'DELETE',_guard:guard});await loadMattermostFederationLinks(peer,scope,guard);}catch(e){setMsg($('mm-fed-link-msg'),e.message,'err');}};row.append(off);}$('mm-fed-link-list').append(row);}
    for(const invite of data.pending||[]){if((data.links||[]).some(l=>l.enabled&&l.remote_channel_id===invite.remote_channel_id))continue;const row=document.createElement('button');row.className='btn btn-ghost mt-2';row.textContent='Входящее приглашение: '+invite.name;row.onclick=()=>{$('mm-fed-link-form').elements.remote_channel_id.value=invite.remote_channel_id;};host.append(row);}
    $('mm-fed-link-form').onsubmit=async event=>{event.preventDefault();const form=event.currentTarget;const body={channel_id:form.elements.channel_id.value,remote_channel_id:form.elements.remote_channel_id.value.trim(),readonly:form.elements.readonly.checked,backfill:form.elements.backfill.checked};try{await api('/api/v1/admin/mattermost/federation/'+encodeURIComponent(peer.id)+'/links'+mmQuery(scope),{method:'POST',body:JSON.stringify(body),_guard:guard});await loadMattermostFederationLinks(peer,scope,guard);}catch(e){setMsg($('mm-fed-link-msg'),e.message,'err');}};
  }catch(e){setMsg($('mm-fed-result'),e.message,'err');}
}

window.addEventListener('tms-session-ended',()=>{mmFed.epoch++;mmAdmin.epoch++;mmFed.peers=[];document.getElementById('mm-federation')?.replaceChildren();});
