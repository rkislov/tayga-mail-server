/* Tayga host adaptations for the original Boards and Playbooks bundles. */
(() => {
  'use strict';
  const scope = location.pathname.match(/^\/mm\/[a-z0-9]{26}/)?.[0];
  if (!scope) return;
  try {
    const color = localStorage.getItem('tayga.colorMode');
    const dark = color === 'dark' || (color === 'auto' && matchMedia('(prefers-color-scheme: dark)').matches);
    document.documentElement.dataset.taygaColor = dark ? 'dark' : 'light';
  } catch { /* Use the default palette when storage is unavailable. */ }
  const queryLanguage = new URLSearchParams(location.search).get('lang');
  let locale = queryLanguage || document.documentElement.lang;
  if (!queryLanguage) {
    try { locale = localStorage.getItem('tayga.lang') || locale; } catch { /* Use the server language. */ }
  }
  locale = locale === 'en' ? 'en' : 'ru';
  document.documentElement.lang = locale;
  const root = document.getElementById('root');
  if (root) root.textContent = locale === 'en' ? 'Loading…' : 'Загрузка…';
  if (document.documentElement.dataset.taygaProduct === 'boards') {
    // The original Boards app has its own locale store, independent of
    // Mattermost's Redux preferences and the host IntlProvider.
    try { localStorage.setItem('language', locale); } catch { /* Use the browser locale. */ }
  }
  const boardStrings = locale === 'ru' ? window.taygaBoardsTemplatesRu : null;
  const base = scope + '/plugins/focalboard/api/v2';
  const fetchProduct = window.fetch.bind(window);
  const templates = new Map();
  const profiles = new Map();
  // Keep React's image node intact; a failed image must not leave a broken icon.
  document.addEventListener('error', event => {
    const image = event.target;
    if (!(image instanceof HTMLImageElement)) return;
    let url;
    try { url = new URL(image.src, location.href); } catch { return; }
    if (url.origin !== location.origin) return;
    const match = url.pathname.match(new RegExp('^' + scope + '/api/v4/users/([a-z0-9]{26})/image$'));
    if (!match) return;
    const profile = profiles.get(match[1]);
    const name = [profile?.first_name, profile?.last_name].filter(Boolean).join(' ').trim()
      || profile?.username || image.closest('.PlaybookRunProfile')?.querySelector('.name')?.textContent || '';
    const words = name.split('@')[0].match(/[\p{L}\p{N}]+/gu) || [];
    const initials = words.slice(0, 2).map(word => Array.from(word)[0]).join('').toLocaleUpperCase() || '?';
    const escaped = initials.replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&apos;'}[char]));
    image.alt = name;
    image.src = 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128"><circle cx="64" cy="64" r="64" fill="#dfe7d6"/><text x="64" y="66" text-anchor="middle" dominant-baseline="middle" font-family="Arial,sans-serif" font-size="47" font-weight="600" fill="#3f5a30">' + escaped + '</text></svg>');
  }, true);

  let statusEditorInstalled = false;

  // The shipped Playbooks host uses a bare textarea for its Textbox bridge.
  // Give the status editor a labelled input and use the host's own Markdown
  // renderer in preview mode. Leave other Textbox consumers on the host bridge.
  function installStatusEditor() {
    const {React, Components, PostUtils} = window;
    if (statusEditorInstalled || document.documentElement.dataset.taygaProduct !== 'playbooks' ||
        !React?.createElement || !Components?.Textbox ||
        !PostUtils?.formatText || !PostUtils?.messageHtmlToComponent) return;
    const HostTextbox = Components.Textbox;
    Components.Textbox = function StatusTextbox(props) {
      if (props.id !== 'update_run_status_textbox') {
        return React.createElement(HostTextbox, props);
      }
      const label = locale === 'en' ? 'Change since last update' : 'Изменения с последнего обновления';
      const {value, id, onChange, disabled, rows} = props;
      if (props.preview) {
        const rendered = PostUtils.messageHtmlToComponent(PostUtils.formatText(value || ''));
        return React.createElement('div', {
          id, className: 'tayga-update-preview', role: 'region', tabIndex: 0,
          'aria-label': locale === 'en' ? 'Update preview' : 'Предпросмотр обновления',
        }, rendered);
      }
      return React.createElement('textarea', {
        id, value, onChange, disabled, rows, className: 'form-control',
        'aria-label': label,
      });
    };
    statusEditorInstalled = true;
  }

  function translateLabel(value) {
    return typeof value === 'string' && Object.hasOwn(boardStrings.labels, value)
      ? boardStrings.labels[value] : value;
  }
  function localizedBoard(board, labels) {
    if (!labels) return board;
    return {
      ...board, title: labels[0], description: labels[1],
      cardProperties: board.cardProperties?.map(property => ({
        ...property, name: translateLabel(property.name),
        options: property.options?.map(option => ({...option, value: translateLabel(option.value)})),
      })),
    };
  }
  function localizedBlocks(blocks) {
    return blocks.map(block => {
      const properties = block.fields?.properties;
      if (!properties || block.type !== 'card') return {...block, title: translateLabel(block.title)};
      const localized = Object.fromEntries(Object.entries(properties).map(([key, value]) => [key, translateLabel(value)]));
      return {...block, title: translateLabel(block.title), fields: {...block.fields, properties: localized}};
    });
  }
  function withJSON(response, data) {
    const headers = new Headers(response.headers);
    headers.delete('Content-Encoding');
    headers.delete('Content-Length');
    headers.delete('ETag');
    return new Response(JSON.stringify(data), {status: response.status, headers});
  }
  function cacheTemplate(board) {
    const title = board?.teamId === '0' && board.isTemplate ? String(board.title || '').trim() : '';
    const labels = Object.hasOwn(boardStrings.boards, title) ? boardStrings.boards[title] : null;
    if (board?.id) templates.set(board.id, labels);
    return labels;
  }
  async function templateLabels(request, id) {
    if (templates.has(id)) return templates.get(id);
    // Official Boards assigns fresh IDs to its default templates on import.
    // Verify metadata through the same scoped, authenticated API before using
    // the translations on blocks or a newly duplicated board.
    const board = await fetchProduct(new URL(base + '/boards/' + id, location.origin), {
      headers: request.headers, credentials: request.credentials, signal: request.signal,
    });
    return board.ok ? cacheTemplate(await board.json()) : null;
  }

  async function boardResponse(request, suffix) {
    if (request.method === 'GET' && /^\/teams\/[^/]+\/categories$/.test(suffix)) {
      const response = await fetchProduct(request);
      if (!response.ok) return response;
      const categories = await response.clone().json();
      return withJSON(response, categories.map(category => category.type === 'system' && category.name === 'Boards'
        ? {...category, name: 'Доски'} : category));
    }
    if (request.method === 'GET' && /^\/teams\/[^/]+\/templates$/.test(suffix)) {
      const response = await fetchProduct(request);
      if (!response.ok) return response;
      const list = await response.clone().json();
      return withJSON(response, list.map(board => localizedBoard(board, cacheTemplate(board))));
    }
    const target = suffix.match(/^\/boards\/([a-z0-9]{26,27})(?:\/(blocks|duplicate))?$/);
    if (!target) return fetchProduct(request);
    const [, id, action] = target;
    if (request.method === 'GET' && action === undefined) {
      const response = await fetchProduct(request);
      if (!response.ok) return response;
      const board = await response.clone().json();
      const labels = cacheTemplate(board);
      return labels ? withJSON(response, localizedBoard(board, labels)) : response;
    }
    if (request.method === 'GET' && action === 'blocks') {
      const response = await fetchProduct(request);
      if (!response.ok || !await templateLabels(request, id)) return response;
      const blocks = await response.clone().json();
      return withJSON(response, localizedBlocks(blocks));
    }
    if (request.method !== 'POST' || action !== 'duplicate') return fetchProduct(request);
    const labels = await templateLabels(request, id);
    const response = await fetchProduct(request);
    if (!response.ok || !labels) return response;
    const item = await response.clone().json();
    const original = item.boards?.[0];
    if (!original || item.boards.length !== 1 || original.teamId === '0' || !Array.isArray(item.blocks)) return response;
    const board = localizedBoard(original, labels);
    const blocks = localizedBlocks(item.blocks);
    const changed = [];
    for (const [index, block] of blocks.entries()) {
      const previous = item.blocks[index];
      const properties = block.fields?.properties;
      const updated = properties && Object.entries(properties).some(([key,value]) => previous.fields.properties[key] !== value);
      if (block.title !== previous.title || updated) {
        changed.push({id: block.id, patch: {title: block.title, updatedFields: updated ? {properties} : undefined}});
      }
    }
    const headers = new Headers(request.headers);
    headers.set('Content-Type', 'application/json');
    const patch = await fetchProduct(new URL(base + '/boards-and-blocks', location.origin), {
      method: 'PATCH', headers, credentials: request.credentials,
      body: JSON.stringify({
        boardIDs: [board.id],
        boardPatches: [{title: board.title, description: board.description, updatedCardProperties: board.cardProperties}],
        blockIDs: changed.map(block => block.id),
        blockPatches: changed.map(block => block.patch),
      }),
    });
    if (!patch.ok) return response;
    return withJSON(response, {...item, boards: [board], blocks});
  }

  window.fetch = (input, options) => {
    const request = new Request(input, options);
    const url = new URL(request.url);
    if (url.origin !== location.origin) return fetchProduct(request);
    if (url.pathname === scope + '/products/bootstrap') {
      url.searchParams.set('lang', locale);
      return fetchProduct(new Request(url, request)).then(async response => {
        if (response.ok) {
          installStatusEditor();
          try {
            const data = await response.clone().json();
            for (const profile of data.users || []) profiles.set(profile.id, profile);
          } catch { /* The host handles an invalid bootstrap response. */ }
        }
        return response;
      });
    }
    if (boardStrings && document.documentElement.dataset.taygaProduct === 'boards' && url.pathname.startsWith(base + '/')) {
      return boardResponse(request, url.pathname.slice(base.length));
    }
    return fetchProduct(request);
  };
})();
