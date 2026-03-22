export type MermaidOverlayFit = {
	intrinsicWidth: number;
	intrinsicHeight: number;
	fitScale: number;
};

const readViewBoxDimensions = (viewBox: string): { width: number; height: number } | null => {
	const parts = viewBox.split(/[\s,]+/).map(Number);
	if (parts.length !== 4 || !parts.every((value) => Number.isFinite(value))) return null;
	return {
		width: parts[2] ?? 0,
		height: parts[3] ?? 0,
	};
};

const resolveSvgDimensions = (svg: SVGElement): { width: number; height: number } | null => {
	const explicitWidth = Number(svg.getAttribute('width') ?? '');
	const explicitHeight = Number(svg.getAttribute('height') ?? '');
	if (
		Number.isFinite(explicitWidth) &&
		explicitWidth > 0 &&
		Number.isFinite(explicitHeight) &&
		explicitHeight > 0
	) {
		return { width: explicitWidth, height: explicitHeight };
	}

	const viewBox = svg.getAttribute('viewBox')?.trim() ?? '';
	if (!viewBox) return null;
	return readViewBoxDimensions(viewBox);
};

export const calculateMermaidOverlayFit = (
	svg: SVGElement,
	viewportWidth: number,
	viewportHeight: number,
): MermaidOverlayFit | null => {
	const dimensions = resolveSvgDimensions(svg);
	if (!dimensions) return null;

	return {
		intrinsicWidth: dimensions.width,
		intrinsicHeight: dimensions.height,
		fitScale: Math.min(
			Math.max(1, viewportWidth) / dimensions.width,
			Math.max(1, viewportHeight) / dimensions.height,
		),
	};
};
