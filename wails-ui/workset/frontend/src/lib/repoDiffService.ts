import { subscribeWailsEvent } from './wailsEventRegistry';

type EventHandler<T> = (payload: T) => void;

export const subscribeRepoDiffEvent = <T>(event: string, handler: EventHandler<T>): (() => void) =>
	subscribeWailsEvent(event, handler);
