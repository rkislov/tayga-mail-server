// SPDX-License-Identifier: GPL-3.0-only
// The generated secret exists only in its one-time dialog, never in storage.
let xmppDevicesRevision = 0;
const xmppDevicesAPI = '/api/v1/me/xmpp-devices';
function xmppDeviceNode(tag, className = '', text = '') {
  const node = document.createElement(tag); node.className = className; node.textContent = text; return node;
}
function xmppDeviceButton(text, action, primary = false) {
  const button = xmppDeviceNode('button', primary ? 'btn-spray' : 'btn-secondary', text);
  button.type = 'button'; button.onclick = action; return button;
}
function xmppDeviceError(error) {
  const messages = {
    device_connection_required: ['Сначала подключите хотя бы один клиент с новым паролем устройства.', 'Connect at least one client with a device password first.'],
    device_limit: ['Можно создать не более 50 действующих паролей.', 'Up to 50 active device passwords are allowed.'],
    invalid_device_name: ['Введите название от 1 до 80 символов.', 'Enter a name between 1 and 80 characters.'],
    device_not_found: ['Устройство или соединение уже удалено. Обновите список.', 'The device or connection is gone. Refresh the list.'],
    xmpp_device_operation_failed: ['Не удалось выполнить операцию. Обновите список и повторите.', 'Operation failed. Refresh the list and try again.']
  };
  return messages[error.message] ? mailImageText(...messages[error.message]) : error.message;
}
function resetXMPPDevices() {
  xmppDevicesRevision++;
  for (const dialog of document.querySelectorAll('.xmpp-device-dialog')) { dialog.querySelectorAll('input').forEach(input => { input.value = ''; }); dialog.close(); dialog.remove(); }
  $('xmpp-devices-card')?.replaceChildren();
}
async function refreshXMPPDevices() {
  const host = $('profile-security'); if (!host || !state.tokens?.access_token) return;
  let card = $('xmpp-devices-card');
  if (!card) { card = xmppDeviceNode('section', 'card stack-gap'); card.id = 'xmpp-devices-card'; host.prepend(card); }
  const query = card.querySelector('input[type=search]')?.value || '';
  const revision = ++xmppDevicesRevision, guard = omemoGuard();
  const active = () => { try { guard(); return revision === xmppDevicesRevision; } catch { return false; } };
  card.replaceChildren(xmppDeviceNode('p', 'meta', mailImageText('Загрузка устройств XMPP…', 'Loading XMPP devices…')));
  try {
    const data = await api(xmppDevicesAPI, { _guard: guard }); if (!active()) return;
    const header = xmppDeviceNode('div', 'xmpp-device-toolbar');
    header.append(xmppDeviceNode('h3', 'heading', mailImageText('Устройства XMPP', 'XMPP devices')),
      xmppDeviceButton(mailImageText('Обновить', 'Refresh'), refreshXMPPDevices),
      xmppDeviceButton(mailImageText('Создать пароль устройства', 'Create device password'), createXMPPDeviceDialog, true));
    const hint = xmppDeviceNode('p', 'meta', mailImageText(
      'Создайте отдельный пароль для каждого клиента: Monal, Conversations или другого приложения. Он работает только в XMPP. Отзыв отключает все соединения с этим паролем, восстановление сессий и связанные push-подписки.',
      'Create a separate password for each client: Monal, Conversations or another app. It works only with XMPP. Revoking it disconnects every connection using it, disables session resume and removes associated push subscriptions.'));
    const status = xmppDeviceNode('p', 'msg'); status.setAttribute('role', 'status');
    const policy = xmppDeviceNode('label', 'field-check');
    const checkbox = document.createElement('input'); checkbox.type = 'checkbox'; checkbox.checked = !!data.devices_only;
    checkbox.onchange = async () => {
      const only = checkbox.checked; checkbox.checked = !!data.devices_only;
      if (!confirm(mailImageText(only
        ? 'Разрешить XMPP только с паролями устройств? Клиенты с обычным паролем или общим токеном отключатся; их push-подписки будут удалены. Сначала замените пароль в каждом приложении. Почта и веб-вход продолжат работать.'
        : 'Снова разрешить обычный пароль и общий токен для входа в XMPP?', only
        ? 'Allow XMPP only with device passwords? Clients using an account password or general token will disconnect and their push registrations will be removed. Update each app first. Mail and web login will keep working.'
        : 'Allow account passwords and general tokens for XMPP again?'))) return;
      checkbox.disabled = true;
      try { await api(xmppDevicesAPI + '/policy', { method: 'PUT', body: JSON.stringify({ devices_only: only }), _guard: guard }); if (active()) await refreshXMPPDevices(); }
      catch (error) { if (active()) { status.textContent = xmppDeviceError(error); checkbox.disabled = false; } }
    };
    policy.append(checkbox, xmppDeviceNode('span', '', mailImageText('Только пароли устройств для XMPP', 'Device passwords only for XMPP')));
    const policyHint = xmppDeviceNode('p', 'meta', mailImageText(data.devices_only
      ? 'Включено. Обычный пароль и общие токены больше не подходят для XMPP.'
      : 'Переходный режим: прежние способы входа ещё разрешены. Подключите клиент с новым паролем, затем включите ограничение.', data.devices_only
      ? 'Enabled. Account passwords and general tokens are no longer accepted for XMPP.'
      : 'Migration mode: existing login methods are still allowed. Connect with a new device password, then enable the restriction.'));
    const search = document.createElement('input'); search.type = 'search'; search.value = query; search.className = 'field-input'; search.placeholder = mailImageText('Поиск по названию, ресурсу или IP', 'Search by name, resource or IP'); search.setAttribute('aria-label', search.placeholder);
    const lists = xmppDeviceNode('div', 'xmpp-device-lists');
    const perform = async (url, confirmation, button) => {
      if (!confirm(confirmation)) return; button.disabled = true;
      try { await api(url, { method: 'DELETE', _guard: guard }); if (active()) await refreshXMPPDevices(); }
      catch (error) { if (active()) { status.textContent = xmppDeviceError(error); button.disabled = false; } }
    };
    const date = value => value ? new Date(value).toLocaleString(state.lang === 'en' ? 'en-US' : 'ru-RU') : '—';
    const deviceName = id => data.devices.find(d => d.id === id)?.name || mailImageText('Обычный пароль / общий токен', 'Account password / general token');
    const paginatedTable = (title, columns, rows, render, textForSearch) => {
      const section = xmppDeviceNode('section', 'xmpp-device-list'), heading = xmppDeviceNode('h4', '', title);
      const wrap = xmppDeviceNode('div', 'xmpp-device-table-wrap'), table = xmppDeviceNode('table', 'admin-users-table');
      const thead = document.createElement('thead'), headRow = document.createElement('tr');
      columns.forEach(text => headRow.append(xmppDeviceNode('th', '', text))); thead.append(headRow);
      const tbody = document.createElement('tbody'); table.append(thead, tbody); wrap.append(table);
      const pager = xmppDeviceNode('div', 'xmpp-device-toolbar'), counter = xmppDeviceNode('span', 'meta'); let page = 0;
      const prev = xmppDeviceButton(mailImageText('Назад', 'Previous'), () => { page--; draw(); });
      const next = xmppDeviceButton(mailImageText('Далее', 'Next'), () => { page++; draw(); }); pager.append(counter, prev, next);
      const draw = () => {
        const filtered = rows.filter(row => textForSearch(row).toLocaleLowerCase().includes(search.value.trim().toLocaleLowerCase()));
        page = Math.min(Math.max(page, 0), Math.max(0, Math.ceil(filtered.length / 8) - 1)); tbody.replaceChildren();
        filtered.slice(page * 8, page * 8 + 8).forEach(item => {
          const tr = document.createElement('tr'); render(item).forEach(content => { const td = document.createElement('td'); if (typeof content === 'string') td.textContent = content; else td.append(content); tr.append(td); }); tbody.append(tr);
        });
        if (!filtered.length) { const tr = document.createElement('tr'), td = xmppDeviceNode('td', 'meta', mailImageText('Нет записей', 'No entries')); td.colSpan = columns.length; tr.append(td); tbody.append(tr); }
        counter.textContent = filtered.length ? `${page * 8 + 1}–${Math.min(page * 8 + 8, filtered.length)} / ${filtered.length}` : '0';
        prev.disabled = page === 0; next.disabled = (page + 1) * 8 >= filtered.length;
      };
      search.addEventListener('input', () => { page = 0; draw(); }); section.append(heading, wrap, pager); lists.append(section); draw();
    };
    paginatedTable(mailImageText('Пароли устройств', 'Device passwords'),
      [mailImageText('Устройство', 'Device'), mailImageText('Создан / последний вход', 'Created / last login'), mailImageText('Состояние', 'Status'), mailImageText('Действия', 'Actions')], data.devices, device => {
        const times = xmppDeviceNode('div', '', date(device.created_at)); times.append(xmppDeviceNode('p', 'meta', date(device.last_used_at)));
        const live = data.sessions.filter(s => s.credential_id === device.id).length;
        const label = device.revoked_at ? mailImageText('Отозван', 'Revoked') : live ? mailImageText(`Подключений: ${live}`, `Connections: ${live}`) : device.last_used_at ? mailImageText('Не подключено', 'Disconnected') : mailImageText('Ожидает первого входа', 'Waiting for first login');
        const action = device.revoked_at ? xmppDeviceNode('span', 'meta', date(device.revoked_at)) : xmppDeviceButton(mailImageText('Отозвать', 'Revoke'), event => perform(xmppDevicesAPI + '/' + encodeURIComponent(device.id), mailImageText(`Отозвать пароль «${device.name}»? Все использующие его соединения отключатся. Вход с ним больше невозможен. Локальная история на устройстве останется.`, `Revoke “${device.name}”? All connections using it will close and it cannot be used again. Local history remains on the device.`), event.currentTarget));
        return [device.name, times, label, action];
      }, device => device.name);
    paginatedTable(mailImageText('Сейчас подключены', 'Connected now'),
      [mailImageText('Клиент / ресурс', 'Client / resource'), mailImageText('Пароль устройства', 'Device password'), 'IP', mailImageText('Последняя активность', 'Last activity'), mailImageText('Действия', 'Actions')], data.sessions, session =>
      [session.agent, deviceName(session.credential_id), session.address, date(session.last_seen), xmppDeviceButton(mailImageText('Отключить', 'Disconnect'), event => perform(xmppDevicesAPI + '/sessions/' + encodeURIComponent(session.id), mailImageText('Закрыть это соединение? Клиент сможет подключиться снова. Чтобы запретить вход, отзовите его пароль.', 'Close this connection? The client can reconnect. Revoke its password to prevent login.'), event.currentTarget))], session => [session.agent, session.address, deviceName(session.credential_id)].join(' '));
    const note = xmppDeviceNode('p', 'meta', mailImageText('Показаны до 100 последних паролей, действующие — первыми. При потере устройства отдельно отзовите его ключ OMEMO в настройках шифрования чата. Пароль доступа и ключ шифрования — разные настройки.', 'Up to 100 recent passwords are shown, active ones first. If a device is lost, also retire its OMEMO key in the chat encryption settings. Access credentials and encryption keys are separate settings.'));
    card.replaceChildren(header, hint, policy, policyHint, search, status, lists, note);
  } catch (error) { if (active()) card.replaceChildren(xmppDeviceNode('p', 'msg', xmppDeviceError(error)), xmppDeviceButton(mailImageText('Повторить', 'Retry'), refreshXMPPDevices)); }
}
function createXMPPDeviceDialog() {
  if (document.querySelector('.xmpp-device-dialog')) return;
  const guard = omemoGuard(), dialog = xmppDeviceNode('dialog', 'mail-source-dialog xmpp-device-dialog');
  const form = document.createElement('form'), header = document.createElement('header'), title = xmppDeviceNode('h3', 'heading', mailImageText('Пароль для устройства XMPP', 'XMPP device password'));
  title.id = 'xmpp-device-title'; dialog.setAttribute('aria-labelledby', title.id);
  header.append(title, xmppDeviceButton(mailImageText('Закрыть', 'Close'), () => dialog.close()));
  const fields = xmppDeviceNode('div', 'xmpp-device-fields'), label = xmppDeviceNode('label', 'field-label', mailImageText('Название устройства', 'Device name'));
  const name = document.createElement('input'); name.className = 'field-input'; name.required = true; name.maxLength = 80; name.autocomplete = 'off'; name.placeholder = 'Monal · iPhone'; label.append(name); fields.append(label);
  const hint = xmppDeviceNode('p', 'meta', mailImageText('Пароль будет показан один раз. Введите его вместо обычного пароля в настройках приложения XMPP. Логин, сервер и порт оставьте прежними. Создание пароля не меняет ключи OMEMO.', 'The password is shown once. Use it instead of your account password in your XMPP app. Keep the username, server and port unchanged. Creating a password does not change OMEMO keys.'));
  const status = xmppDeviceNode('p', 'msg'); status.setAttribute('role', 'status');
  const footer = document.createElement('footer'), submit = xmppDeviceNode('button', 'btn-spray', mailImageText('Создать пароль', 'Create password')); submit.type = 'submit'; footer.append(submit);
  let secretInput;
  form.onsubmit = async event => {
    event.preventDefault(); if (!form.reportValidity() || submit.disabled) return; submit.disabled = true;
    try {
      const data = await api(xmppDevicesAPI, { method: 'POST', body: JSON.stringify({ name: name.value }), _guard: guard }); guard();
      if (!dialog.open) { refreshXMPPDevices(); return; }
      const secretLabel = xmppDeviceNode('label', 'field-label', mailImageText('Сохраните пароль сейчас', 'Save this password now'));
      secretInput = document.createElement('input'); secretInput.className = 'field-input xmpp-device-secret'; secretInput.readOnly = true; secretInput.autocomplete = 'off'; secretInput.spellcheck = false; secretInput.value = data.password; data.password = ''; secretLabel.append(secretInput); fields.replaceChildren(secretLabel);
      const copy = xmppDeviceButton(mailImageText('Копировать', 'Copy'), async () => {
        try { guard(); await navigator.clipboard.writeText(secretInput.value); guard(); if (dialog.open) status.textContent = mailImageText('Пароль скопирован.', 'Password copied.'); }
        catch { if (dialog.open) { secretInput.focus(); secretInput.select(); status.textContent = mailImageText('Выделите и скопируйте пароль вручную.', 'Select and copy the password manually.'); } }
      });
      footer.replaceChildren(copy, xmppDeviceButton(mailImageText('Готово', 'Done'), () => dialog.close(), true));
      status.textContent = mailImageText('Пароль создан. Подключите клиент и обновите список устройств, чтобы проверить вход.', 'Password created. Connect your client and refresh the device list to verify the login.');
      refreshXMPPDevices();
    } catch (error) { try { guard(); if (dialog.open) { status.textContent = xmppDeviceError(error); submit.disabled = false; } } catch {} }
  };
  dialog.onclose = () => { if (secretInput) secretInput.value = ''; name.value = ''; dialog.remove(); };
  form.append(header, fields, hint, status, footer); dialog.append(form); document.body.append(dialog); dialog.showModal(); name.focus();
}
