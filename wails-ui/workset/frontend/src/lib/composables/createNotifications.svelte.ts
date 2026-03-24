export type NotificationLevel = 'error' | 'warn' | 'info';

export type Notification = {
	id: string;
	level: NotificationLevel;
	message: string;
	timestamp: number;
	actionLabel?: string;
	onAction?: () => void | Promise<void>;
};

type NotificationOptions = {
	duration?: number;
	actionLabel?: string;
	onAction?: () => void | Promise<void>;
};

export type NotificationManager = {
	readonly notifications: Notification[];
	error: (message: string, options?: NotificationOptions) => void;
	warn: (message: string, options?: NotificationOptions) => void;
	info: (message: string, options?: NotificationOptions) => void;
	dismiss: (id: string) => void;
	destroy: () => void;
};

const DEFAULT_DURATION_MS = 5000;

let nextId = 0;

export function createNotifications(): NotificationManager {
	let notifications = $state<Notification[]>([]);
	const timers = new Map<string, ReturnType<typeof setTimeout>>();

	const add = (level: NotificationLevel, message: string, options?: NotificationOptions): void => {
		const id = `notif-${++nextId}`;
		const duration = options?.duration ?? DEFAULT_DURATION_MS;
		notifications = [
			...notifications,
			{
				id,
				level,
				message,
				timestamp: Date.now(),
				actionLabel: options?.actionLabel,
				onAction: options?.onAction,
			},
		];
		if (duration > 0) {
			timers.set(
				id,
				setTimeout(() => {
					timers.delete(id);
					notifications = notifications.filter((n) => n.id !== id);
				}, duration),
			);
		}
	};

	const dismiss = (id: string): void => {
		const timer = timers.get(id);
		if (timer) {
			clearTimeout(timer);
			timers.delete(id);
		}
		notifications = notifications.filter((n) => n.id !== id);
	};

	const destroy = (): void => {
		for (const timer of timers.values()) {
			clearTimeout(timer);
		}
		timers.clear();
		notifications = [];
	};

	return {
		get notifications() {
			return notifications;
		},
		error: (message, options) => add('error', message, options),
		warn: (message, options) => add('warn', message, options),
		info: (message, options) => add('info', message, options),
		dismiss,
		destroy,
	};
}
