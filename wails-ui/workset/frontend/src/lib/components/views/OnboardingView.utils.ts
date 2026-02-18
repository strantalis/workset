import type { HookExecution } from '../../types';
import type { WorkspaceActionPendingHook } from '../../services/workspaceActionHooks';
import type {
	RegisteredRepo,
	RepoTemplate,
	WorksetTemplate,
} from '../../view-models/onboardingViewModel';

export type OnboardingDraft = {
	workspaceName: string;
	description: string;
	repos: RepoTemplate[];
	selectedGroups: string[];
	selectedAliases: string[];
	primarySource: string;
	directRepos: Array<{ url: string; register: boolean }>;
};

export type OnboardingStartResult = {
	workspaceName: string;
	warnings: string[];
	pendingHooks: WorkspaceActionPendingHook[];
	hookRuns: HookExecution[];
};

export type ReviewRepoEntry = {
	key: string;
	repo: RepoTemplate;
	source: string;
	hooks: string[];
};

export type TemplateVisualIcon = 'code2' | 'workflow' | 'server';

export const getTemplateVisual = (
	template: Pick<WorksetTemplate, 'repos'>,
): { icon: TemplateVisualIcon; color: string } => {
	if (template.repos.length >= 6) return { icon: 'server', color: '#2D8CFF' };
	if (template.repos.length >= 3) return { icon: 'workflow', color: '#86C442' };
	return { icon: 'code2', color: '#F28C28' };
};

export const resolveHookPreviewSource = (repo: RepoTemplate): string => {
	const aliasName = (repo.aliasName ?? '').trim();
	if (repo.sourceType === 'alias' && aliasName) return aliasName;
	const remote = (repo.remoteUrl ?? '').trim();
	if (remote) return remote;
	if (aliasName) return aliasName;
	return repo.name.trim();
};

export const computeRepoPositions = (
	repos: RepoTemplate[],
): Array<{ x: number; y: number; name: string }> => {
	if (repos.length === 0) return [];
	const total = repos.length;
	const radius = total === 1 ? 120 : 140;
	return repos.map((repo, i) => {
		const angle = total === 1 ? 0 : (i / total) * 2 * Math.PI - Math.PI / 2;
		return {
			x: Math.cos(angle) * radius,
			y: Math.sin(angle) * radius,
			name: repo.name,
		};
	});
};

export const getStepStatus = (
	currentStep: number,
	stepToCheck: number,
): 'active' | 'completed' | 'pending' => {
	if (stepToCheck === currentStep) return 'active';
	if (stepToCheck < currentStep) return 'completed';
	return 'pending';
};

export type { RegisteredRepo, RepoTemplate, WorksetTemplate };
