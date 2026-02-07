import {
	DeleteSkill as WailsDeleteSkill,
	GetSkill as WailsGetSkill,
	ListSkills as WailsListSkills,
	SaveSkill as WailsSaveSkill,
	SyncSkill as WailsSyncSkill,
} from '../../../wailsjs/go/main/App';

export type SkillInfo = {
	name: string;
	description: string;
	dirName: string;
	scope: string;
	tools: string[];
	path: string;
};

export type SkillContent = SkillInfo & {
	content: string;
};

export async function listSkills(workspaceId?: string): Promise<SkillInfo[]> {
	return (await WailsListSkills({ workspaceId: workspaceId ?? '' })) as SkillInfo[];
}

export async function getSkill(
	scope: string,
	dirName: string,
	tool: string,
	workspaceId?: string,
): Promise<SkillContent> {
	return (await WailsGetSkill({
		scope,
		dirName,
		tool,
		workspaceId: workspaceId ?? '',
	})) as SkillContent;
}

export async function saveSkill(
	scope: string,
	dirName: string,
	tool: string,
	content: string,
	workspaceId?: string,
): Promise<void> {
	await WailsSaveSkill({ scope, dirName, tool, content, workspaceId: workspaceId ?? '' });
}

export async function deleteSkill(
	scope: string,
	dirName: string,
	tool: string,
	workspaceId?: string,
): Promise<void> {
	await WailsDeleteSkill({ scope, dirName, tool, workspaceId: workspaceId ?? '' });
}

export async function syncSkill(
	scope: string,
	dirName: string,
	fromTool: string,
	toTools: string[],
	workspaceId?: string,
): Promise<void> {
	await WailsSyncSkill({ scope, dirName, fromTool, toTools, workspaceId: workspaceId ?? '' });
}
