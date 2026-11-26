const fs = require('node:fs');
const path = require('node:path');

const source = 'node_modules/markdown-it';
const dest = '../internal/frontend/dist/assets';
const browser = fs.readFileSync(path.join(source, 'dist/browser/markdown-it.umd.min.js'), 'utf8');
fs.writeFileSync(path.join(dest, 'markdown-it.js'), browser.replace(/^\/\/# sourceMappingURL=.*\n?$/m, ''));
fs.copyFileSync(path.join(source, 'LICENSE'), path.join(dest, 'markdown-it-LICENSE.txt'));
