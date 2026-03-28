type RuntimeTransportEnvelope = {
	object?: unknown;
	method?: unknown;
	args?: {
		methodID?: unknown;
		methodName?: unknown;
		args?: unknown[];
	};
};

export type WailsBindingFailureDetail = {
	status: number;
	bindingName: string;
	methodID: number | null;
	runtimeObject: number | null;
	runtimeMethod: number | null;
	responseText: string;
	argsPreview: unknown[];
};

const RUNTIME_URL_SUFFIX = '/wails/runtime';
const CALL_OBJECT_ID = 0;
const CALL_METHOD_ID = 0;

const KNOWN_BINDING_NAMES: Record<number, string> = {
	440589369: 'PreviewRepoHooks',
	1015230358: 'CreateWorkspace',
	2319042906: 'ListRegisteredRepos',
	2678866830: 'SearchGitHubRepositories',
	3260789714: 'GetGitHubAuthInfo',
	927671124: 'GetGitHubAuthStatus',
	3496751781: 'SetGitHubAuthMode',
	2156208737: 'SetGitHubCLIPath',
	3969957477: 'SetGitHubToken',
};

let diagnosticsInstalled = false;
const FAILURE_BUFFER_LIMIT = 20;

const isRecord = (value: unknown): value is Record<string, unknown> =>
	value !== null && typeof value === 'object' && !Array.isArray(value);

const summarizeValue = (value: unknown, depth = 0): unknown => {
	if (value === null || value === undefined) return value;
	if (typeof value === 'string') {
		return value.length > 160 ? `${value.slice(0, 157)}...` : value;
	}
	if (typeof value === 'number' || typeof value === 'boolean') {
		return value;
	}
	if (Array.isArray(value)) {
		if (depth >= 1) {
			return `[array(${value.length})]`;
		}
		return value.slice(0, 5).map((entry) => summarizeValue(entry, depth + 1));
	}
	if (!isRecord(value)) {
		return String(value);
	}
	if (depth >= 1) {
		return '[object]';
	}
	const preview: Record<string, unknown> = {};
	for (const [key, entry] of Object.entries(value).slice(0, 8)) {
		preview[key] = summarizeValue(entry, depth + 1);
	}
	return preview;
};

export const decodeWailsBindingFailure = ({
	url,
	body,
	status,
	responseText,
}: {
	url: string;
	body: string | null | undefined;
	status: number;
	responseText: string;
}): WailsBindingFailureDetail | null => {
	if (!url.endsWith(RUNTIME_URL_SUFFIX) || !body) {
		return null;
	}
	let payload: RuntimeTransportEnvelope;
	try {
		payload = JSON.parse(body) as RuntimeTransportEnvelope;
	} catch {
		return null;
	}

	const runtimeObject =
		typeof payload.object === 'number' ? payload.object : Number(payload.object ?? NaN);
	const runtimeMethod =
		typeof payload.method === 'number' ? payload.method : Number(payload.method ?? NaN);
	const methodIDValue = payload.args?.methodID;
	const methodID = typeof methodIDValue === 'number' ? methodIDValue : Number(methodIDValue ?? NaN);
	const methodName =
		typeof payload.args?.methodName === 'string' ? payload.args.methodName.trim() : '';
	const bindingName =
		methodName ||
		(Number.isFinite(methodID) ? KNOWN_BINDING_NAMES[methodID] : null) ||
		(Number.isFinite(runtimeObject) && Number.isFinite(runtimeMethod)
			? `object:${runtimeObject}/method:${runtimeMethod}`
			: 'unknown');
	const argsPreview = Array.isArray(payload.args?.args)
		? payload.args.args.slice(0, 3).map((entry) => summarizeValue(entry))
		: [];

	if (
		Number.isFinite(runtimeObject) &&
		Number.isFinite(runtimeMethod) &&
		runtimeObject === CALL_OBJECT_ID &&
		runtimeMethod === CALL_METHOD_ID
	) {
		return {
			status,
			bindingName,
			methodID: Number.isFinite(methodID) ? methodID : null,
			runtimeObject,
			runtimeMethod,
			responseText,
			argsPreview,
		};
	}

	return {
		status,
		bindingName,
		methodID: Number.isFinite(methodID) ? methodID : null,
		runtimeObject: Number.isFinite(runtimeObject) ? runtimeObject : null,
		runtimeMethod: Number.isFinite(runtimeMethod) ? runtimeMethod : null,
		responseText,
		argsPreview,
	};
};

export const installWailsBindingDiagnostics = (): void => {
	if (diagnosticsInstalled || typeof window === 'undefined' || typeof fetch !== 'function') {
		return;
	}
	diagnosticsInstalled = true;

	const originalFetch = window.fetch.bind(window);
	window.fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
		const response = await originalFetch(input, init);
		if (response.ok) {
			return response;
		}

		const url =
			typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
		const requestBody = typeof init?.body === 'string' ? init.body : null;
		const responseText = await response
			.clone()
			.text()
			.catch(() => '');
		const detail = decodeWailsBindingFailure({
			url,
			body: requestBody,
			status: response.status,
			responseText,
		});
		if (detail) {
			const diagnosticsWindow = window as typeof window & {
				__WORKSET_WAILS_BINDING_FAILURES__?: WailsBindingFailureDetail[];
			};
			const existing = diagnosticsWindow.__WORKSET_WAILS_BINDING_FAILURES__ ?? [];
			diagnosticsWindow.__WORKSET_WAILS_BINDING_FAILURES__ = [...existing, detail].slice(
				-FAILURE_BUFFER_LIMIT,
			);
			window.dispatchEvent(
				new CustomEvent<WailsBindingFailureDetail>('workset:wails-binding-failure', {
					detail,
				}),
			);
		}
		return response;
	};
};
