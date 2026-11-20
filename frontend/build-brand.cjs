const fs = require('node:fs');
const path = require('node:path');
const source = path.join(__dirname, 'assets/tayga-tree.svg');
const target = path.join(__dirname, '../internal/frontend/dist/assets');
for (const name of ['tayga-tree.svg', 'favicon.svg']) fs.copyFileSync(source, path.join(target, name));
