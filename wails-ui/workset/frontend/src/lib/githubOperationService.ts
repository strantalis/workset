import type { GitHubOperationStatus } from './api/github';
import { EVENT_GITHUB_OPERATION } from './events';
import { subscribeWailsEvent } from './wailsEventRegistry';

type EventHandler<T> = (payload: T) => void;

export const subscribeGitHubOperationEvent = (
	handler: EventHandler<GitHubOperationStatus>,
): (() => void) => subscribeWailsEvent(EVENT_GITHUB_OPERATION, handler);
