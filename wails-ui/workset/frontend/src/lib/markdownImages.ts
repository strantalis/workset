import { readWorkspaceRepoImageBase64 } from './api/repo-files';

const IMAGE_EXTENSIONS = new Set([
	'.png',
	'.jpg',
	'.jpeg',
	'.gif',
	'.webp',
	'.svg',
	'.ico',
	'.bmp',
	'.avif',
]);

const MAX_CACHE_ENTRIES = 100;
const imageCache = new Map<string, string>();

export function clearImageCache(): void {
	imageCache.clear();
}

export type ImageResolveContext = {
	workspaceId: string;
	repoId: string;
	markdownFilePath: string;
};

/**
 * Post-process rendered markdown HTML to resolve relative image paths
 * into data: URIs fetched from the backend.
 *
 * Must be called AFTER DOMPurify sanitization since DOMPurify strips data: URIs.
 */
export async function resolveMarkdownImages(
	html: string,
	context: ImageResolveContext,
): Promise<string> {
	const doc = new DOMParser().parseFromString(html, 'text/html');
	const images = doc.querySelectorAll<HTMLImageElement>('img[src]');
	if (images.length === 0) return html;

	const mdDir = parentDir(context.markdownFilePath);

	const tasks: { img: HTMLImageElement; resolvedPath: string }[] = [];
	for (const img of images) {
		const src = img.getAttribute('src') ?? '';
		if (isAbsoluteUrl(src)) continue;
		if (!hasImageExtension(src)) continue;
		const resolved = resolvePath(mdDir, src);
		if (!resolved) continue;
		tasks.push({ img, resolvedPath: resolved });
	}

	if (tasks.length === 0) return html;

	await Promise.allSettled(
		tasks.map(async ({ img, resolvedPath }) => {
			const cacheKey = `${context.workspaceId}|${context.repoId}|${resolvedPath}`;
			const cached = imageCache.get(cacheKey);
			if (cached) {
				img.setAttribute('src', cached);
				return;
			}

			const response = await readWorkspaceRepoImageBase64(
				context.workspaceId,
				context.repoId,
				resolvedPath,
			);
			if (response.error || !response.base64) {
				img.setAttribute('alt', `[Image not found: ${resolvedPath}]`);
				return;
			}

			const dataUri = `data:${response.mimeType};base64,${response.base64}`;
			img.setAttribute('src', dataUri);

			if (imageCache.size >= MAX_CACHE_ENTRIES) {
				const oldest = imageCache.keys().next().value;
				if (oldest) imageCache.delete(oldest);
			}
			imageCache.set(cacheKey, dataUri);
		}),
	);

	return doc.body.innerHTML;
}

export function parentDir(filePath: string): string {
	const lastSlash = filePath.lastIndexOf('/');
	return lastSlash > 0 ? filePath.slice(0, lastSlash) : '';
}

export function isAbsoluteUrl(src: string): boolean {
	return /^[a-z][a-z0-9+.-]*:/i.test(src) || src.startsWith('//');
}

export function hasImageExtension(src: string): boolean {
	const path = src.split('?')[0].split('#')[0];
	const dot = path.lastIndexOf('.');
	if (dot < 0) return false;
	return IMAGE_EXTENSIONS.has(path.slice(dot).toLowerCase());
}

/**
 * Resolve a relative path against a base directory.
 * Returns a clean repo-relative path, or null if resolution escapes the root.
 */
export function resolvePath(baseDir: string, relativePath: string): string | null {
	const rel = relativePath.replace(/^\.\//, '');
	const parts = (baseDir ? baseDir + '/' + rel : rel).split('/');
	const resolved: string[] = [];
	for (const part of parts) {
		if (part === '' || part === '.') continue;
		if (part === '..') {
			if (resolved.length === 0) return null;
			resolved.pop();
		} else {
			resolved.push(part);
		}
	}
	return resolved.join('/') || null;
}
