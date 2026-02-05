const DEFAULT_TERMINAL_KEYBINDINGS = {
	'terminal.close_tab': ['CmdOrCtrl+W'],
	'terminal.new_tab': ['CmdOrCtrl+T'],
	'terminal.split_vertical': ['CmdOrCtrl+code:Backslash'],
	'terminal.split_horizontal': ['CmdOrCtrl+Shift+code:Backslash'],
	'terminal.focus_tab_1': ['CmdOrCtrl+1'],
	'terminal.focus_tab_2': ['CmdOrCtrl+2'],
	'terminal.focus_tab_3': ['CmdOrCtrl+3'],
	'terminal.focus_tab_4': ['CmdOrCtrl+4'],
	'terminal.focus_tab_5': ['CmdOrCtrl+5'],
	'terminal.focus_tab_6': ['CmdOrCtrl+6'],
	'terminal.focus_tab_7': ['CmdOrCtrl+7'],
	'terminal.focus_tab_8': ['CmdOrCtrl+8'],
	'terminal.focus_tab_9': ['CmdOrCtrl+9'],
	'terminal.font_increase': ['CmdOrCtrl+='],
	'terminal.font_decrease': ['CmdOrCtrl+-'],
	'terminal.font_reset': ['CmdOrCtrl+0'],
	'terminal.focus_pane_up': ['CmdOrCtrl+Alt+ArrowUp'],
	'terminal.focus_pane_down': ['CmdOrCtrl+Alt+ArrowDown'],
	'terminal.focus_pane_left': ['CmdOrCtrl+Alt+ArrowLeft'],
	'terminal.focus_pane_right': ['CmdOrCtrl+Alt+ArrowRight'],
	'terminal.next_tab': ['Ctrl+Tab'],
	'terminal.prev_tab': ['Ctrl+Shift+Tab'],
};

type TerminalKeybindingAction = keyof typeof DEFAULT_TERMINAL_KEYBINDINGS;

type TerminalKeybindings = Record<string, string[]>;

type ParsedChord = {
	key?: string;
	code?: string;
	cmd: boolean;
	cmdOrCtrl: boolean;
	ctrl: boolean;
	alt: boolean;
	shift: boolean;
};

type ResolvedTerminalKeybindings = {
	actions: Array<{ action: string; chords: ParsedChord[] }>;
};

const normalizeToken = (token: string): string => token.trim().toLowerCase();

const normalizeEventKey = (key: string): string => key.toLowerCase();

const normalizeCode = (code: string): string => code.toLowerCase();

const parseChord = (raw: string): ParsedChord | null => {
	const parts = raw
		.split('+')
		.map((part) => part.trim())
		.filter(Boolean);

	let key: string | undefined;
	let code: string | undefined;
	let cmd = false;
	let cmdOrCtrl = false;
	let ctrl = false;
	let alt = false;
	let shift = false;

	for (const part of parts) {
		const token = normalizeToken(part);
		if (token === 'cmdorctrl' || token === 'commandorcontrol') {
			cmdOrCtrl = true;
			continue;
		}
		if (token === 'cmd' || token === 'command' || token === 'meta') {
			cmd = true;
			continue;
		}
		if (token === 'ctrl' || token === 'control') {
			ctrl = true;
			continue;
		}
		if (token === 'alt' || token === 'option') {
			alt = true;
			continue;
		}
		if (token === 'shift') {
			shift = true;
			continue;
		}
		if (key || code) {
			return null;
		}
		if (token.startsWith('code:')) {
			code = normalizeCode(token.slice('code:'.length));
		} else {
			key = token;
		}
	}

	if (!key && !code) return null;
	return {
		key,
		code,
		cmd,
		cmdOrCtrl,
		ctrl,
		alt,
		shift,
	};
};

const isMacPlatform = (): boolean => {
	if (typeof navigator === 'undefined') return false;
	const platform = navigator.platform ?? navigator.userAgent ?? '';
	return /mac|iphone|ipad|ipod/i.test(platform);
};

const matchKey = (bindingKey: string, eventKey: string): boolean => {
	if (bindingKey === '=' && (eventKey === '=' || eventKey === '+')) return true;
	if (bindingKey === '-' && (eventKey === '-' || eventKey === '_')) return true;
	return bindingKey === eventKey;
};

const chordMatches = (event: KeyboardEvent, chord: ParsedChord): boolean => {
	const isMac = isMacPlatform();
	const needsCmd = chord.cmd || (chord.cmdOrCtrl && isMac);
	const needsCtrl = chord.ctrl || (chord.cmdOrCtrl && !isMac);

	if (event.metaKey !== needsCmd) return false;
	if (event.ctrlKey !== needsCtrl) return false;
	if (event.altKey !== chord.alt) return false;
	if (event.shiftKey !== chord.shift) return false;

	if (chord.code) {
		return normalizeCode(event.code) === chord.code;
	}

	if (!chord.key) return false;
	return matchKey(chord.key, normalizeEventKey(event.key));
};

const buildActionList = (bindings: TerminalKeybindings): ResolvedTerminalKeybindings => {
	const actions: Array<{ action: string; chords: ParsedChord[] }> = [];
	const orderedActions = Object.keys(DEFAULT_TERMINAL_KEYBINDINGS) as TerminalKeybindingAction[];

	for (const action of orderedActions) {
		const rawChords = bindings[action] ?? [];
		const chords = rawChords.map(parseChord).filter((chord): chord is ParsedChord => !!chord);
		if (chords.length > 0) {
			actions.push({ action, chords });
		}
	}

	for (const [action, rawChords] of Object.entries(bindings)) {
		if (action in DEFAULT_TERMINAL_KEYBINDINGS) continue;
		const chords = rawChords.map(parseChord).filter((chord): chord is ParsedChord => !!chord);
		if (chords.length > 0) {
			actions.push({ action, chords });
		}
	}

	return { actions };
};

export const resolveTerminalKeybindings = (
	overrides?: Record<string, string[]> | null,
): ResolvedTerminalKeybindings => {
	const resolved: TerminalKeybindings = { ...DEFAULT_TERMINAL_KEYBINDINGS };
	if (!overrides) return buildActionList(resolved);

	for (const [action, value] of Object.entries(overrides)) {
		if (!Array.isArray(value)) continue;
		resolved[action] = value.filter((entry) => typeof entry === 'string');
	}

	return buildActionList(resolved);
};

export const matchTerminalKeybinding = (
	event: KeyboardEvent,
	resolved: ResolvedTerminalKeybindings,
): string | null => {
	for (const entry of resolved.actions) {
		for (const chord of entry.chords) {
			if (chordMatches(event, chord)) {
				return entry.action;
			}
		}
	}
	return null;
};

export { DEFAULT_TERMINAL_KEYBINDINGS };
export type { ResolvedTerminalKeybindings, TerminalKeybindingAction, TerminalKeybindings };
