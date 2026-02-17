import type { DiffLineAnnotation } from './annotations';
import type { FileDiffRenderOptions } from './diffRenderController';

const MIN_SPLIT_DIFF_WIDTH = 1100;

export const resolveDiffStyle = (containerWidth: number | null | undefined): 'split' | 'unified' => {
	if (containerWidth == null) return 'split';
	return containerWidth < MIN_SPLIT_DIFF_WIDTH ? 'unified' : 'split';
};

export const buildDiffRenderOptions = <TAnnotation>(
	containerWidth: number | null | undefined,
	renderAnnotation: ((annotation: DiffLineAnnotation<TAnnotation>) => HTMLElement | undefined) | undefined,
): FileDiffRenderOptions<TAnnotation> => ({
	theme: 'pierre-dark',
	themeType: 'dark',
	diffStyle: resolveDiffStyle(containerWidth),
	diffIndicators: 'bars',
	overflow: 'wrap',
	renderAnnotation,
});
