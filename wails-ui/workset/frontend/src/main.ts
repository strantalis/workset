import './style.css';
import App from './App.svelte';
import { mount } from 'svelte';
import { installWailsBindingDiagnostics } from './lib/wailsBindingDiagnostics';

const target = document.getElementById('app');

if (!target) {
	throw new Error('Missing #app mount point');
}

installWailsBindingDiagnostics();

const app = mount(App, {
	target,
});

export default app;
