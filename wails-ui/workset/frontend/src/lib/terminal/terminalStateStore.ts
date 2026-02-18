import { writable, type Writable } from 'svelte/store';

export class TerminalStateStore<T> {
	private readonly stores = new Map<string, Writable<T>>();

	ensure(id: string, build: (id: string) => T): Writable<T> {
		let store = this.stores.get(id);
		if (!store) {
			store = writable(build(id));
			this.stores.set(id, store);
		}
		return store;
	}

	emit(id: string, build: (id: string) => T): void {
		const store = this.ensure(id, build);
		store.set(build(id));
	}

	emitAll(build: (id: string) => T): void {
		for (const id of this.stores.keys()) {
			this.emit(id, build);
		}
	}

	delete(id: string): void {
		this.stores.delete(id);
	}
}
