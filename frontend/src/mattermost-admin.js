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
  mmAdmin.selected = {...item.scope}; const epoch = ++mmAdmin.epoch, guard = mmCurrentGuard(epoch, state.me?.id);
  $('mm-policy-form').inert = true; $('mm-plugin-upload').disabled = true;
  try {
    const value = await api('/api/v1/admin/mattermost' + mmQuery(item.scope), {_guard: guard});
    mmAdmin.value = value; renderMattermostScope(value); setMsg($('mm-msg'), '');
  } catch (error) { if (mmAdmin.epoch === epoch) setMsg($('mm-msg'), error.message, 'err'); }
  finally { if (mmAdmin.epoch === epoch) $('mm-policy-form').inert = false; }
}
function renderMattermostScope(value) {
  const own = value.overrides || {}, effective = value.effective || {};
  $('mm-enabled').value = mmBool(own.enabled); $('mm-plugins-enabled').value = mmBool(own.plugins_enabled);
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
    return `<div class="config-collection-item"><strong>${escapeHtml(item.name || item.id)}</strong><p class="meta">${escapeHtml(item.id)} · ${escapeHtml(item.version || '')}</p><div class="actions"><button type="button" class="btn-secondary" data-mm-plugin="${escapeHtml(item.id)}" data-mm-action="${enabled ? 'disable' : 'enable'}" ${!value.scope?.domain_id || !value.plugin_host_enabled || !effective.plugins_enabled ? 'disabled' : ''}>${enabled ? 'Отключить в домене' : 'Запустить в домене'}</button></div></div>`;
  }).join('') || '<p class="meta">Пакеты ещё не установлены. Наличие пакета не подтверждает совместимость всех его функций.</p>';
  $('mm-plugin-list').querySelectorAll('[data-mm-plugin]').forEach(button => button.addEventListener('click', async () => {
    const scope = {...mmAdmin.selected}, epoch = mmAdmin.epoch, guard = mmCurrentGuard(epoch, state.me?.id); button.disabled = true;
    try { await api('/api/v4/plugins/' + encodeURIComponent(button.dataset.mmPlugin) + '/' + button.dataset.mmAction + mmQuery(scope), {method: 'POST', body: '{}', _guard: guard}); guard(); await loadMattermostScope(); }
    catch (error) { if (mmAdmin.epoch === epoch) { setMsg($('mm-msg'), error.data?.message || error.message, 'err'); button.disabled = false; } }
  }));
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
  for (const [field, id] of [['enabled','mm-enabled'], ['plugins_enabled','mm-plugins-enabled']]) if ($(id).value !== 'inherit') value[field] = $(id).value === 'on';
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
