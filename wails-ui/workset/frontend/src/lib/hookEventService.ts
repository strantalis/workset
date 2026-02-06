import { EventsOn } from '../../wailsjs/runtime/runtime';
import type { HookProgressEvent } from './types';

type EventHandler<T> = (payload: T) => void;

type EventRegistryEntry = {
	handlers: Set<EventHandler<unknown>>;
	bound: boolean;
	unsubscribe?: () => void;
};

const eventRegistry = new Map<string, EventRegistryEntry>();

const subscribeEvent = <T>(event: string, handler: EventHandler<T>): (() => void) => {
	let entry = eventRegistry.get(event);
	if (!entry) {
		entry = { handlers: new Set(), bound: false };
		eventRegistry.set(event, entry);
	}
	entry.handlers.add(handler as EventHandler<unknown>);
	if (!entry.bound) {
		const unsubscribe = EventsOn(event, (payload: T) => {
			const current = eventRegistry.get(event);
			if (!current) return;
			for (const registered of current.handlers) {
				registered(payload as unknown);
			}
		});
		entry.unsubscribe = unsubscribe;
		entry.bound = true;
	}
	return () => {
		const current = eventRegistry.get(event);
		if (!current) return;
		current.handlers.delete(handler as EventHandler<unknown>);
		if (current.handlers.size !== 0) {
			return;
		}
		if (current.bound) {
			current.unsubscribe?.();
		}
		eventRegistry.delete(event);
	};
};

export const subscribeHookProgressEvent = (
	handler: EventHandler<HookProgressEvent>,
): (() => void) => subscribeEvent('hooks:progress', handler);
