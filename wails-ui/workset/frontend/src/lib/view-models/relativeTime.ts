type RelativeUnit = Intl.RelativeTimeFormatUnit;

const UNIT_SECONDS: Array<{ unit: RelativeUnit; seconds: number }> = [
	{ unit: 'week', seconds: 7 * 24 * 60 * 60 },
	{ unit: 'day', seconds: 24 * 60 * 60 },
	{ unit: 'hour', seconds: 60 * 60 },
	{ unit: 'minute', seconds: 60 },
];

const formatters = new Map<string, Intl.RelativeTimeFormat>();

const resolveLocale = (): string => {
	if (typeof navigator !== 'undefined' && navigator.language) {
		return navigator.language;
	}
	return 'en-US';
};

const formatterFor = (locale: string): Intl.RelativeTimeFormat => {
	const cached = formatters.get(locale);
	if (cached) return cached;
	const formatter = new Intl.RelativeTimeFormat(locale, { numeric: 'auto', style: 'short' });
	formatters.set(locale, formatter);
	return formatter;
};

export const formatRelativeTime = (iso: string, now = Date.now()): string => {
	const ts = new Date(iso).getTime();
	if (Number.isNaN(ts)) return 'unknown';

	const diffSeconds = Math.round((ts - now) / 1000);
	const absSeconds = Math.abs(diffSeconds);
	if (absSeconds < 60) return 'just now';

	const { unit, seconds } =
		UNIT_SECONDS.find((entry) => absSeconds >= entry.seconds) ??
		UNIT_SECONDS[UNIT_SECONDS.length - 1];
	const value = Math.round(diffSeconds / seconds);
	return formatterFor(resolveLocale()).format(value, unit);
};
