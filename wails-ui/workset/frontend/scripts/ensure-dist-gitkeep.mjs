import { mkdir, open } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const distDir = resolve(scriptDir, '..', 'dist');
const gitkeepPath = resolve(distDir, '.gitkeep');

await mkdir(distDir, { recursive: true });
const handle = await open(gitkeepPath, 'a');
await handle.close();
