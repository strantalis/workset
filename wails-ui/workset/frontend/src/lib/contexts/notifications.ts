import { getContext, setContext } from 'svelte';
import type { NotificationManager } from '../composables/createNotifications.svelte';

const NOTIFICATION_CONTEXT_KEY = Symbol('notifications');

export function provideNotifications(manager: NotificationManager): void {
	setContext(NOTIFICATION_CONTEXT_KEY, manager);
}

export function useNotifications(): NotificationManager {
	return getContext<NotificationManager>(NOTIFICATION_CONTEXT_KEY);
}
