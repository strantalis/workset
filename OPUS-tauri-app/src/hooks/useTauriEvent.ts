import { useEffect } from 'react';
import { onEvent } from '@/api/events';

/**
 * Subscribe to a Tauri event, auto-unsubscribing on unmount.
 */
export function useTauriEvent<T>(event: string, handler: (payload: T) => void) {
  useEffect(() => {
    let unlisten: (() => void) | undefined;

    onEvent<T>(event, handler).then((fn) => {
      unlisten = fn;
    });

    return () => {
      unlisten?.();
    };
  }, [event, handler]);
}
