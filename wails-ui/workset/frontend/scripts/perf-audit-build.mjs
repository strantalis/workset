/* eslint-disable no-console */
import { readdirSync, statSync } from 'node:fs';
import { relative } from 'node:path';

const rootDir = new URL('../', import.meta.url);
const distDir = new URL('../dist/', import.meta.url);
const assetsDir = new URL('../dist/assets/', import.meta.url);

const formatKiB = (bytes) => `${(bytes / 1024).toFixed(2)} kB`;
const formatMiB = (bytes) => `${(bytes / (1024 * 1024)).toFixed(2)} MB`;

const collectFiles = (dirUrl) => {
	const files = [];
	for (const entry of readdirSync(dirUrl, { withFileTypes: true })) {
		const entryUrl = new URL(`${entry.name}${entry.isDirectory() ? '/' : ''}`, dirUrl);
		if (entry.isDirectory()) {
			files.push(...collectFiles(entryUrl));
			continue;
		}
		const stats = statSync(entryUrl);
		files.push({
			path: relative(rootDir.pathname, entryUrl.pathname),
			bytes: stats.size,
		});
	}
	return files;
};

let files;
try {
	files = collectFiles(distDir);
} catch (error) {
	console.error('dist/ is missing. Run `npm run build` first.');
	if (error instanceof Error) {
		console.error(error.message);
	}
	process.exit(1);
}

const assetFiles = files.filter((file) => file.path.startsWith('dist/assets/'));
const sortedAssets = [...assetFiles].sort((left, right) => right.bytes - left.bytes);
const totalDistBytes = files.reduce((total, file) => total + file.bytes, 0);
const totalAssetBytes = assetFiles.reduce((total, file) => total + file.bytes, 0);
const jsAssetBytes = assetFiles
	.filter((file) => file.path.endsWith('.js'))
	.reduce((total, file) => total + file.bytes, 0);
const cssAssetBytes = assetFiles
	.filter((file) => file.path.endsWith('.css'))
	.reduce((total, file) => total + file.bytes, 0);

const mainEntryChunk =
	sortedAssets.find((file) => /dist\/assets\/index-.*\.js$/.test(file.path)) ?? null;
const mainCssChunk =
	sortedAssets.find((file) => /dist\/assets\/index-.*\.css$/.test(file.path)) ?? null;

console.log('Frontend build audit summary');
console.log('');
console.log(`dist files: ${files.length}`);
console.log(`dist asset files: ${assetFiles.length}`);
console.log(`dist total: ${formatMiB(totalDistBytes)}`);
console.log(`assets total: ${formatMiB(totalAssetBytes)}`);
console.log(`js assets total: ${formatMiB(jsAssetBytes)}`);
console.log(`css assets total: ${formatKiB(cssAssetBytes)}`);

if (mainEntryChunk) {
	console.log(`main entry chunk: ${mainEntryChunk.path} (${formatKiB(mainEntryChunk.bytes)})`);
}
if (mainCssChunk) {
	console.log(`main css chunk: ${mainCssChunk.path} (${formatKiB(mainCssChunk.bytes)})`);
}

console.log('');
console.log('largest 15 assets:');
for (const file of sortedAssets.slice(0, 15)) {
	console.log(`- ${file.path}: ${formatKiB(file.bytes)}`);
}

if (readdirSync(assetsDir).length === 0) {
	console.warn('');
	console.warn('Warning: dist/assets is empty.');
}
