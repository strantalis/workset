import { Events } from '@wailsio/runtime';

type EventHandler<T> = (payload: T) => void;

type EventRegistryEntry = {
	handlers: Set<EventHandler<unknown>>;
	bound: boolean;
	unsubscribe?: () => void;
};

const WAILS_EVENT_REGISTRY_GLOBAL_KEY = '__worksetWailsEventRegistry';

const getGlobalEventRegistry = (): Map<string, EventRegistryEntry> => {
	const root = globalThis as typeof globalThis & {
		[WAILS_EVENT_REGISTRY_GLOBAL_KEY]?: Map<string, EventRegistryEntry>;
	};
	root[WAILS_EVENT_REGISTRY_GLOBAL_KEY] ??= new Map<string, EventRegistryEntry>();
	return root[WAILS_EVENT_REGISTRY_GLOBAL_KEY]!;
};

const eventRegistry = getGlobalEventRegistry();

export const subscribeWailsEvent = <T>(event: string, handler: EventHandler<T>): (() => void) => {
	let entry = eventRegistry.get(event);
	if (!entry) {
		entry = { handlers: new Set(), bound: false };
		eventRegistry.set(event, entry);
	}
	entry.handlers.add(handler as EventHandler<unknown>);
	if (!entry.bound) {
		const unsubscribe = Events.On(event, (payloadEvent) => {
			const payload = payloadEvent?.data as T;
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
