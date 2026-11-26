/* Shared renderer for bot replies in Tayga's web chat and Agents. */
(() => {
  const markdown = window.markdownit?.({html: false, breaks: true});
  if (!markdown) return;

  const validateLink = markdown.validateLink.bind(markdown);
  markdown.validateLink = value => {
    if (!validateLink(value)) return false;
    try {
      const {protocol} = new URL(value, document.baseURI);
      return protocol === 'https:' || protocol === 'http:' || protocol === 'mailto:';
    } catch {
      return false;
    }
  };

  markdown.renderer.rules.link_open = (tokens, index, options, env, renderer) => {
    env.insideLink = true;
    const token = tokens[index];
    const url = new URL(token.attrGet('href'), document.baseURI);
    if (url.protocol !== 'mailto:' && url.origin !== location.origin) {
      token.attrSet('target', '_blank');
      token.attrSet('rel', 'noopener noreferrer');
    }
    return renderer.renderToken(tokens, index, options);
  };
  markdown.renderer.rules.link_close = (tokens, index, options, env, renderer) => {
    env.insideLink = false;
    return renderer.renderToken(tokens, index, options);
  };

  // Keep images behind a click so opening a conversation does not contact remote hosts.
  markdown.renderer.rules.image = (tokens, index, options, env, renderer) => {
    const token = tokens[index];
    const url = markdown.utils.escapeHtml(token.attrGet('src'));
    const label = markdown.utils.escapeHtml(renderer.renderInlineAsText(token.children, options, env) || 'Изображение');
    return env.insideLink ? label : `<a href="${url}" target="_blank" rel="noopener noreferrer">${label}</a>`;
  };

  markdown.renderer.rules.table_open = (tokens, index, options, _env, renderer) =>
    '<div class="chat-markdown-table">' + renderer.renderToken(tokens, index, options);
  markdown.renderer.rules.table_close = (tokens, index, options, _env, renderer) =>
    renderer.renderToken(tokens, index, options) + '</div>';

  // Older replies can contain escaped pairs of bold markers. Code stays literal.
  const escapedBold = '\\*\\*';
  markdown.inline.ruler.before('escape', 'tayga_escaped_bold', (state, silent) => {
    if (!state.src.startsWith(escapedBold, state.pos)) return false;
    const start = state.pos + escapedBold.length;
    const end = state.src.indexOf(escapedBold, start);
    if (end === -1 || end + escapedBold.length > state.posMax) return false;
    const content = state.src.slice(start, end);
    if (!content || /^\s|\s$|[\r\n`]/u.test(content)) return false;
    if (!silent) {
      state.push('strong_open', 'strong', 1);
      state.push('text', '', 0).content = content;
      state.push('strong_close', 'strong', -1);
    }
    state.pos = end + escapedBold.length;
    return true;
  });

  window.tmsChatMarkdown = (element, content) => {
    element.classList.add('chat-markdown');
    element.innerHTML = markdown.render(content, {});
  };
})();
