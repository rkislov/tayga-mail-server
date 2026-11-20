// Welcome mail revisions are saved independently from publication.
async function openAdminWelcome(kind, target) {
  document.querySelector('dialog.welcome-dialog')?.close();
  const label = mailImageText;
  const dialog = directoryDialog(label('Приветственное письмо', 'Welcome mail') + ' — ' + target.name, 'welcome-dialog');
  const endpoint = '/api/v1/admin/welcome/' + kind + '/' + encodeURIComponent(target.id);
  const node = (tag, cls, text) => { const el = document.createElement(tag); if (cls) el.className = cls; if (text) el.textContent = text; return el; };
  const status = node('p', 'msg'); status.setAttribute('role', 'status');
  const info = node('p', 'meta');
  const deliveryInfo = node('p', 'meta'); deliveryInfo.setAttribute('aria-live','polite');
  const help = node('p', 'meta', label('Сохранение создаёт версию. Публикация включает выбранную сохранённую версию. Опубликованное письмо автоматически отправляется при создании новых ящиков; отключённые ящики получат его после включения. Публикация не рассылает письмо существующим пользователям.', 'Saving creates a version. Publishing activates the selected saved version. The published letter is sent automatically to new mailboxes; disabled mailboxes receive it after activation. Publishing does not mail existing users.'));
  const policy = node('div', 'welcome-policy');
  const mode = node('select', 'field-input'); mode.setAttribute('aria-label', label('Режим шаблона', 'Template mode'));
  for (const [value, text] of [['inherit', label('Наследовать от тенанта', 'Inherit from tenant')], ['own', label('Свой шаблон', 'Own template')], ['off', label('Не отправлять', 'Disabled')]]) {
    if (kind !== 'domains' && value === 'inherit') continue;
    mode.append(new Option(text, value));
  }
  const changeMode = directoryButton(label('Применить режим', 'Apply mode'), () => mutate({action:'mode', mode:mode.value}));
  const reload = directoryButton(label('Обновить историю', 'Refresh history'), () => load(true));
  policy.append(mode, changeMode, reload);
  const form = node('div', 'welcome-form');
  function field(title, tag = 'input') {
    const wrapper = node('label', 'field-label', title), input = node(tag, 'field-input');
    wrapper.append(input); form.append(wrapper); return input;
  }
  const subject = field(label('Тема письма', 'Subject')); subject.maxLength = 250;
  const variables = node('p', 'meta', label('Переменные в тексте: {{name}}, {{email}}, {{domain}}, {{tenant}}. Фотографии можно загрузить кнопкой на панели. Внешние ссылки — полные HTTPS-адреса.', 'Text variables: {{name}}, {{email}}, {{domain}}, {{tenant}}. Upload photos using the toolbar. External links require full HTTPS URLs.'));
  form.append(variables);
  const tools = node('div', 'welcome-toolbar');
  const visualTab = directoryButton(label('Визуальный редактор', 'Visual editor'), () => switchEditor(false));
  const sourceTab = directoryButton('HTML', () => switchEditor(true));
  tools.append(visualTab, sourceTab);
  const editor = node('div', 'welcome-rich-host');
  const editorSurface = node('div'); editor.append(editorSurface);
  const source = node('textarea', 'field-input welcome-source'); source.setAttribute('aria-label', label('HTML письма', 'Mail HTML')); source.spellcheck = false; source.hidden = true;
  const comment = field(label('Комментарий к версии', 'Version comment')); comment.maxLength = 1000;
  // Keep the editor between its toolbar and version comment.
  form.insertBefore(tools, comment.parentElement); form.insertBefore(editor, comment.parentElement); form.insertBefore(source, comment.parentElement);
  const actions = node('div', 'welcome-toolbar');
  const save = directoryButton(label('Сохранить новую версию', 'Save new version'), () => mutate({action:'save', content:content()}), true);
  const previewButton = directoryButton(label('Предпросмотр', 'Preview'), previewDraft);
  actions.append(save, previewButton); form.append(actions);
  const preview = node('iframe', 'welcome-preview'); preview.title = label('Предпросмотр письма', 'Mail preview'); preview.setAttribute('sandbox', ''); preview.hidden = true;
  const textPreview = node('pre', 'welcome-text'); textPreview.hidden = true;
  const historyTitle = node('h3', 'heading', label('История версий', 'Version history'));
  const historyTools = node('div', 'welcome-toolbar');
  const versions = node('select', 'field-input'); versions.setAttribute('aria-label', label('Версия письма', 'Mail version'));
  const publish = directoryButton(label('Опубликовать выбранную', 'Publish selected'), async () => {
    if (!await askConfirm(label('Опубликовать выбранную сохранённую версию? Изменения в редакторе не входят в публикацию.', 'Publish the selected saved version? Unsaved editor changes are not included.'))) return;
    await mutate({action:'publish', revision:Number(versions.value)}, false);
  });
  const restore = directoryButton(label('Восстановить как новую версию', 'Restore as new version'), async () => {
    if (!await askConfirm(label('Создать новую версию из выбранной? Текущий текст редактора будет заменён. Публикация выполняется отдельно.', 'Create a new version from the selected one? This replaces the editor content. Publish it separately.'))) return;
    await mutate({action:'restore', revision:Number(versions.value), content:{comment:comment.value}});
  });
  historyTools.append(versions, publish, restore);
  const historyContent = node('div', 'welcome-comparison');
  const selectedText = node('pre', 'welcome-text');
  const publishedText = node('pre', 'welcome-text');
  historyContent.append(selectedText, publishedText);
  const historyPreview = node('iframe', 'welcome-preview'); historyPreview.title = label('Просмотр выбранной версии', 'Selected version preview'); historyPreview.setAttribute('sandbox', '');
  const audit = node('details'); audit.append(node('summary', '', label('Журнал изменений', 'Change log'))); const auditText = node('pre', 'welcome-text'); audit.append(auditText);
  dialog.append(help, status, info, deliveryInfo, policy, form, preview, textPreview, historyTitle, historyTools, historyContent, historyPreview, audit);
  let data = null, busy = false, dirty = false, sourceMode = false, visualHTML = '', rich = null;
  const sample = value => String(value).replace(/\{\{(name|email|domain|tenant)\}\}/g, (_, key) => ({name:label('Анна Иванова', 'Anna Ivanova'), email:'anna@example.com', domain:'example.com', tenant:label('Организация', 'Organization')}[key]));
  function documentHTML(html) {
    return '<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src \'none\'; img-src https: data:; style-src \'unsafe-inline\'; base-uri \'none\'; form-action \'none\'"><style>body{font:16px/1.5 system-ui;padding:16px;overflow-wrap:anywhere;color:#18212b;background:white}img{max-width:100%;height:auto}table{max-width:100%}pre{white-space:pre-wrap}</style></head><body>' + html + '</body></html>';
  }
  function setVisual(html) {
    visualHTML = html;
    if (rich) rich.setData(html);
  }
  const bodyHTML = () => sourceMode ? source.value : (rich ? rich.getData() : visualHTML);
  const content = () => ({subject:subject.value, html:bodyHTML(), comment:comment.value});
  function fill(revision) {
    const v = revision || {subject:label('Добро пожаловать в {{tenant}}', 'Welcome to {{tenant}}'), html:label('<h1>Здравствуйте, {{name}}!</h1><p>Ваш почтовый адрес: {{email}}.</p>', '<h1>Hello, {{name}}!</h1><p>Your email address: {{email}}.</p>')};
    subject.value = v.subject; source.value = v.html; setVisual(v.html); comment.value = ''; dirty = false;
    preview.hidden = textPreview.hidden = true;
  }
  function lock(value) {
    busy = value;
    dialog.querySelectorAll('button,input,select,textarea').forEach(el => { if (!el.closest('.ck')) el.disabled = el.closest('header') ? false : value; });
    if (rich) { if (value) rich.enableReadOnlyMode('request'); else rich.disableReadOnlyMode('request'); }
    if (!value) {
      publish.disabled = restore.disabled = !data?.revisions.length;
      changeMode.disabled = !data;
      save.disabled = previewButton.disabled = !data || (rich && mailEditorPending(rich));
      for (const option of mode.options) if (option.value === 'own') option.disabled = !data?.published;
    }
  }
  function error(err) {
    const messages = {
      'use a subject, HTML body and supported variables; maximum text size is 256 KB; embedded images up to 2 MiB each and 8 MiB total': label('Проверьте тему, текст и переменные. Текст — до 256 КиБ, фотографии — до 2 МиБ каждая и 8 МиБ суммарно.', 'Check the subject, body and variables. Text: up to 256 KB; photos: 2 MiB each, 8 MiB total.'),
      'invalid action or mode; publish a revision before enabling own template': label('Сначала опубликуйте сохранённую версию, затем включайте свой шаблон.', 'Publish a saved version before enabling your own template.'),
      'welcome save failed': label('Не удалось сохранить письмо. Текст остаётся в редакторе.', 'Could not save the letter. Your text remains in the editor.'),
      'welcome lookup failed': label('Не удалось загрузить шаблон. Повторите загрузку истории.', 'Could not load the template. Refresh history to retry.'),
      'forbidden': label('Недостаточно прав для управления этим шаблоном.', 'You do not have permission to manage this template.'),
      'revision not found': label('Версия не найдена. Обновите историю.', 'Version not found. Refresh history.')
    };
    const conflict = String(err.message).includes('welcome template changed');
    setMsg(status, conflict ? label('Шаблон изменён другим администратором. Ваш текст сохранён в редакторе. Обновите историю, проверьте изменения и повторите сохранение.', 'Another administrator changed this template. Your editor text is retained. Refresh history, review the changes and save again.') : (messages[err.message] || err.message), 'err');
  }
  function renderHistory() {
    const chosen = Number(versions.value), selected = data.revisions.find(v => v.number === chosen), active = data.revisions.find(v => v.number === data.published);
    selectedText.textContent = selected ? label('Выбрана версия ', 'Selected version ') + selected.number + '\n' + selected.subject + '\n\n' + selected.html : label('Версий пока нет.', 'No versions yet.');
    publishedText.textContent = active ? label('Опубликована версия ', 'Published version ') + active.number + '\n' + active.subject + '\n\n' + active.html : label('Нет опубликованной версии.', 'No published version.');
    historyPreview.hidden = !selected;
    if (selected) historyPreview.srcdoc = documentHTML(sample(selected.html));
  }
  function render() {
    const previous = versions.value; versions.replaceChildren();
    for (const v of [...data.revisions].reverse()) versions.append(new Option('#' + v.number + (v.number === data.published ? ' ● ' + label('Опубликована', 'Published') : '') + ' · ' + new Date(v.saved_at).toLocaleString() + ' · ' + v.author + (v.restored_from ? ' ← #' + v.restored_from : '') + (v.comment ? ' · ' + v.comment : ''), String(v.number)));
    if ([...versions.options].some(o => o.value === previous)) versions.value = previous;
    mode.value = data.mode;
    auditText.textContent = [...data.events].reverse().map(e => new Date(e.at).toLocaleString() + ' · ' + e.actor + ' · ' + ({save:label('Сохранение', 'Save'), publish:label('Публикация', 'Publish'), restore:label('Восстановление', 'Restore'), mode:label('Смена режима', 'Mode change')}[e.action] || e.action) + (e.revision ? ' #' + e.revision : '') + ' · ' + e.mode).join('\n');
    renderHistory();
  }
  function showDeliveryStats(result) {
    const stats = result.delivery_stats;
    deliveryInfo.textContent = stats ? label('Доставлено: ', 'Delivered: ') + stats.delivered + label(' · Ожидают: ', ' · Pending: ') + stats.pending + label(' · Повтор после ошибки: ', ' · Retrying: ') + stats.retrying : '';
  }
  async function load(keepEditor = false) {
    if (busy) return; lock(true);
    try {
      const result = await api(endpoint); if (!dialog.isConnected) return;
      data = result.template; render(); showDeliveryStats(result);
      info.textContent = result.effective ? label('Действует: ', 'Effective: ') + (result.source === 'tenants' ? label('тенант', 'tenant') : label('домен', 'domain')) + ' · #' + result.effective.number + ' · ' + result.effective.subject : label('Действующее письмо отсутствует.', 'No effective letter.');
      if (!keepEditor) fill(data.revisions.at(-1) || result.effective);
      if (keepEditor) setMsg(status, label('История обновлена. Текст редактора сохранён; следующее сохранение создаст новую версию.', 'History refreshed. Editor text retained; saving will create a new version.'), 'ok');
    } catch (err) { error(err); } finally { lock(false); }
  }
  async function mutate(change, replaceEditor = true) {
    if (busy || !data || (rich && mailEditorPending(rich))) return; lock(true); setMsg(status, '');
    try {
      const result = await api(endpoint, {method:'POST', body:JSON.stringify({...change, expected:data.generation})});
      if (!dialog.isConnected) return;
      data = result; render();
      if (replaceEditor && (change.action === 'save' || change.action === 'restore')) { fill(data.revisions.at(-1)); versions.value = String(data.revisions.at(-1).number); renderHistory(); }
      setMsg(status, change.action === 'publish' ? label('Версия опубликована.', 'Version published.') : label('Сохранено.', 'Saved.'), 'ok');
      // Update effective inheritance without touching the editor or version choice.
      const effective = await api(endpoint);
      if (dialog.isConnected) showDeliveryStats(effective);
      if (dialog.isConnected) info.textContent = effective.effective ? label('Действует: ', 'Effective: ') + (effective.source === 'tenants' ? label('тенант', 'tenant') : label('домен', 'domain')) + ' · #' + effective.effective.number + ' · ' + effective.effective.subject : label('Действующее письмо отсутствует.', 'No effective letter.');
    } catch (err) { error(err); } finally { lock(false); }
  }
  async function switchEditor(toSource) {
    if (busy || toSource === sourceMode) return;
    if (rich && mailEditorPending(rich)) { setMsg(status, label('Дождитесь вставки фотографии.', 'Wait for the photo to finish inserting.'), 'err'); return; }
    if (toSource) { source.value = bodyHTML(); sourceMode = true; }
    else {
      lock(true);
      try {
        const clean = await api(endpoint, {method:'POST', body:JSON.stringify({action:'preview', content:content()})});
        setVisual(clean.html); sourceMode = false;
      } catch (err) { error(err); } finally { lock(false); }
    }
    source.hidden = !sourceMode; editor.hidden = sourceMode;
    visualTab.setAttribute('aria-pressed', String(!sourceMode)); sourceTab.setAttribute('aria-pressed', String(sourceMode));
  }
  async function previewDraft() {
    if (busy || (rich && mailEditorPending(rich))) return; lock(true);
    try {
      const clean = await api(endpoint, {method:'POST', body:JSON.stringify({action:'preview', content:content()})});
      preview.srcdoc = documentHTML(sample(clean.html)); preview.hidden = false;
      textPreview.textContent = sample(clean.subject) + '\n\n' + sample(clean.text); textPreview.hidden = false;
    } catch (err) { error(err); } finally { lock(false); }
  }
  subject.oninput = source.oninput = comment.oninput = () => { dirty = true; };
  versions.onchange = renderHistory;
  const close = dialog.querySelector('header button');
  const closeEditor = async () => { if (!dirty || await askConfirm(label('Закрыть редактор без сохранения изменений?', 'Close without saving your changes?'))) dialog.close(); };
  close.onclick = closeEditor; dialog.addEventListener('cancel', event => { event.preventDefault(); closeEditor(); });
  dialog.addEventListener('close', () => { rich?.destroy().catch(()=>{}); }, {once:true});
  await load();
  if (!dialog.isConnected || !data) return;
  lock(true);
  try {
    const instance = await createMailRichEditor(editorSurface, {html:visualHTML, onChange:() => { dirty = true; }});
    if (!dialog.isConnected) { await instance.destroy(); return; }
    rich = instance;
    rich.plugins.get('PendingActions').on('change:hasAny', () => { save.disabled = rich.plugins.get('PendingActions').hasAny || busy; });
  } catch (err) {
    sourceMode = true; source.hidden = false; editor.hidden = true;
    error(err);
  } finally { lock(false); }
}
$('tenant-welcome-button')?.addEventListener('click', () => openAdminWelcome('tenants', {id:tenantNav.tenantID, name:tenantNav.tenantName}));
