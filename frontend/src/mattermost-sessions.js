const mmProfileSessions = (() => {
  const list = $('mm-sessions-list');
  const status = $('mm-sessions-status');
  const reload = $('mm-sessions-refresh');
  let generation = 0;

  function reset() {
    generation++;
    list?.replaceChildren();
    if (status) setMsg(status, '');
    if (reload) reload.disabled = false;
  }

  function formatDate(value) {
    return new Date(value).toLocaleString(lang === 'en' ? 'en' : 'ru', {
      dateStyle: 'medium', timeStyle: 'short',
    });
  }

  async function refresh() {
    if (!list || !state.tokens?.access_token) return;
    const revision = ++generation;
    const guard = () => {
      if (revision !== generation || !state.tokens?.access_token) {
        throw new Error('Session changed');
      }
    };
    reload.disabled = true;
    list.replaceChildren();
    setMsg(status, mailImageText('Загрузка…', 'Loading…'));
    try {
      const sessions = await api('/api/v4/users/me/sessions', {_guard: guard});
      guard();
      for (const session of sessions) {
        // The profiled web login also appears in the Mattermost session list.
        // Do not offer to revoke it or any entry from an older API without the flag.
        if (session.is_current === true) continue;
        const row = document.createElement('li');
        row.className = 'mm-session-row';
        const details = document.createElement('span');
        details.textContent = mailImageText('Последняя активность: ', 'Last active: ') +
          formatDate(session.last_activity_at) + ' · ' +
          mailImageText('Действует до: ', 'Expires: ') + formatDate(session.expires_at);
        row.append(details);
        if (session.is_current === false) {
          const revoke = document.createElement('button');
          revoke.type = 'button';
          revoke.className = 'btn-secondary';
          revoke.textContent = mailImageText('Завершить', 'End session');
          revoke.addEventListener('click', async () => {
            if (generation !== revision) return;
            if (!window.confirm(mailImageText('Завершить эту сессию Mattermost? На устройстве потребуется повторный вход.',
              'End this Mattermost session? The device will need to sign in again.'))) return;
            revoke.disabled = true;
            try {
              await api('/api/v4/users/sessions/revoke', {
                method: 'POST', body: JSON.stringify({session_id: session.id}), _guard: guard,
              });
              guard();
              await refresh();
            } catch (error) {
              if (revision !== generation) return;
              revoke.disabled = false;
              setMsg(status, error.data?.message || error.message, 'err');
            }
          });
          row.append(revoke);
        }
        list.append(row);
      }
      setMsg(status, list.childElementCount ? '' : mailImageText('Других активных сессий нет.', 'No other active sessions.'));
    } catch (error) {
      if (revision !== generation) return;
      const unavailable = [403, 404, 501].includes(error.status);
      setMsg(status, unavailable ? mailImageText('Mattermost недоступен для этой учётной записи.',
        'Mattermost is unavailable for this account.') : error.data?.message || error.message, unavailable ? '' : 'err');
    } finally {
      if (revision === generation) reload.disabled = false;
    }
  }

  reload?.addEventListener('click', refresh);
  return {refresh, reset};
})();
