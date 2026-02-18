import {
	deleteSkill,
	getSkill,
	listSkills,
	saveSkill,
	type SkillContent,
	type SkillInfo,
} from '../api/skills';

export type SkillRegistryState = {
	items: SkillInfo[];
	loading: boolean;
	error: string | null;
};

export const loadSkillsState = async (): Promise<SkillRegistryState> => {
	try {
		const items = await listSkills();
		return { items, loading: false, error: null };
	} catch (error) {
		return {
			items: [],
			loading: false,
			error: error instanceof Error ? error.message : 'Failed to load skills',
		};
	}
};

export const resolvePreferredTool = (skill: SkillInfo): string => {
	if (skill.tools.includes('agents')) return 'agents';
	if (skill.tools.includes('claude')) return 'claude';
	if (skill.tools.includes('codex')) return 'codex';
	return skill.tools[0] ?? 'agents';
};

export const loadSkillContent = async (skill: SkillInfo): Promise<SkillContent> => {
	const tool = resolvePreferredTool(skill);
	return getSkill(skill.scope, skill.dirName, tool);
};

export const saveSkillContent = async (
	scope: string,
	dirName: string,
	tool: string,
	content: string,
): Promise<void> => {
	await saveSkill(scope, dirName, tool, content);
};

export const removeSkill = async (skill: SkillInfo): Promise<void> => {
	const tool = resolvePreferredTool(skill);
	await deleteSkill(skill.scope, skill.dirName, tool);
};
