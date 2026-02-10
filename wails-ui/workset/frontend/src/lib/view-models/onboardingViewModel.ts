import { getGroup, listGroups, listRegisteredRepos } from '../api/settings';
import type { Alias } from '../types';

export type RepoTemplate = {
	name: string;
	remoteUrl?: string;
	hooks: string[];
	aliasName?: string;
	sourceType: 'alias' | 'direct';
};

export type WorksetTemplate = {
	id: string;
	name: string;
	description: string;
	groupName: string;
	repos: RepoTemplate[];
};

export type RegisteredRepo = {
	id: string;
	name: string;
	aliasName: string;
	remoteUrl: string;
	defaultBranch: string;
	language: string;
	tags: string[];
};

export type OnboardingCatalog = {
	templates: WorksetTemplate[];
	repoRegistry: RegisteredRepo[];
};

export const hookPresets = [
	'npm install',
	'npm run build',
	'npm run dev',
	'npm test',
	'docker compose up',
	'go mod download',
	'pip install -r requirements.txt',
	'cargo build',
];

export const languageColors: Record<string, string> = {
	Go: '#00ADD8',
	TypeScript: '#3178C6',
	Python: '#3776AB',
	Scala: '#DC322F',
	JSON: '#A3B5C9',
	MDX: '#F28C28',
	Rust: '#DEA584',
	Java: '#B07219',
	Repository: '#A3B5C9',
};

const normalizeSource = (alias: Alias): string => alias.url ?? alias.path ?? '';

const inferLanguage = (alias: Alias): string => {
	const source = normalizeSource(alias).toLowerCase();
	const name = (alias.name ?? '').toLowerCase();
	const haystack = `${source} ${name}`;
	if (/\bgo\b|\.go(?:\b|$)/.test(haystack)) return 'Go';
	if (/\btypescript\b|\.tsx?(?:\b|$)|\bnode\b/.test(haystack)) return 'TypeScript';
	if (/\bpython\b|\.py(?:\b|$)/.test(haystack)) return 'Python';
	if (/\bscala\b|\.scala(?:\b|$)/.test(haystack)) return 'Scala';
	if (/\.json(?:\b|$)/.test(haystack)) return 'JSON';
	if (/\bdocs\b|\.mdx(?:\b|$)/.test(haystack)) return 'MDX';
	return 'Repository';
};

const inferTags = (alias: Alias): string[] => {
	const source = normalizeSource(alias);
	const tags = new Set<string>();
	if (source.startsWith('git@') || source.startsWith('https://')) tags.add('remote');
	if (source.startsWith('/') || source.startsWith('~') || source.startsWith('.')) tags.add('local');
	if ((alias.name ?? '').includes('frontend')) tags.add('frontend');
	if ((alias.name ?? '').includes('service')) tags.add('backend');
	return Array.from(tags);
};

const mapAliasesToRegistry = (aliases: Alias[]): RegisteredRepo[] =>
	aliases
		.map((alias) => ({
			id: alias.name,
			name: alias.name,
			aliasName: alias.name,
			remoteUrl: normalizeSource(alias),
			defaultBranch: alias.default_branch ?? 'main',
			language: inferLanguage(alias),
			tags: inferTags(alias),
		}))
		.sort((left, right) => left.name.localeCompare(right.name));

export const loadOnboardingCatalog = async (): Promise<OnboardingCatalog> => {
	const [aliases, groups] = await Promise.all([listRegisteredRepos(), listGroups()]);
	const aliasesByName = new Map(aliases.map((alias) => [alias.name, alias]));

	const templates = await Promise.all(
		groups.map(async (group) => {
			try {
				const fullGroup = await getGroup(group.name);
				const repos: RepoTemplate[] = fullGroup.members.map((member) => {
					const alias = aliasesByName.get(member.repo);
					return {
						name: member.repo,
						remoteUrl: alias ? normalizeSource(alias) : '',
						hooks: [],
						aliasName: member.repo,
						sourceType: 'alias',
					};
				});
				return {
					id: group.name,
					name: group.name,
					description:
						group.description?.trim() ||
						`${repos.length} ${repos.length === 1 ? 'repository' : 'repositories'}`,
					groupName: group.name,
					repos,
				} satisfies WorksetTemplate;
			} catch {
				return {
					id: group.name,
					name: group.name,
					description:
						group.description?.trim() ||
						`${group.repo_count} ${group.repo_count === 1 ? 'repository' : 'repositories'}`,
					groupName: group.name,
					repos: [],
				} satisfies WorksetTemplate;
			}
		}),
	);

	return {
		templates: templates.sort((left, right) => left.name.localeCompare(right.name)),
		repoRegistry: mapAliasesToRegistry(aliases),
	};
};
