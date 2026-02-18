import { EVENT_HOOKS_PROGRESS } from './events';
import type { HookProgressEvent } from './types';
import { subscribeWailsEvent } from './wailsEventRegistry';

type EventHandler<T> = (payload: T) => void;

export const subscribeHookProgressEvent = (
	handler: EventHandler<HookProgressEvent>,
): (() => void) => subscribeWailsEvent(EVENT_HOOKS_PROGRESS, handler);
