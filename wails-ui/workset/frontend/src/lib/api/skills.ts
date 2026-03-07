import { Call } from '@wailsio/runtime';
import {
	DeleteSkill as WailsDeleteSkill,
	GetSkill as WailsGetSkill,
	ListSkills as WailsListSkills,
	SaveSkill as WailsSaveSkill,
	SyncSkill as WailsSyncSkill,
} from '../../../bindings/workset/app';

export type SkillScope = 'global' | 'project';
export type MarketplaceProvider = 'skills.sh';

export type SkillInfo = {
	name: string;
	description: string;
	dirName: string;
	scope: SkillScope;
	tools: string[];
	path: string;
	marketplace?: SkillMarketplaceSource | null;
};

export type SkillMarketplaceSource = {
	provider: MarketplaceProvider;
	externalId: string;
	sourceRepo?: string;
	sourceUrl?: string;
	listingUrl?: string;
	rawSkillUrl?: string;
};

export type SkillContent = SkillInfo & {
	content: string;
};

export type MarketplaceSkill = {
	provider: MarketplaceProvider;
	externalId: string;
	name: string;
	description: string;
	sourceRepo: string;
	sourceUrl: string;
	listingUrl?: string;
	rawSkillUrl: string;
	installCount?: number | null;
	weeklyInstalls?: number | null;
	githubStars?: number | null;
	firstSeen?: string | null;
	repoVerified?: boolean | null;
	auditSummaries?: MarketplaceAuditSummary[];
	verified?: boolean | null;
	trustScore?: number | null;
	benchmarkScore?: number | null;
	relevance?: number | null;
};

export type MarketplaceAuditSummary = {
	provider: string;
	status: string;
	detailUrl?: string;
};

export type MarketplaceSkillContent = {
	skill: MarketplaceSkill;
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

export async function searchMarketplaceSkills(
	input: {
		query: string;
		limit?: number;
	},
	workspaceId?: string,
): Promise<MarketplaceSkill[]> {
	return (await Call.ByName('main.App.SearchMarketplaceSkills', {
		workspaceId: workspaceId ?? '',
		provider: 'skills.sh',
		query: input.query,
		limit: input.limit ?? 24,
	})) as MarketplaceSkill[];
}

export async function getMarketplaceSkillContent(
	skill: MarketplaceSkill,
	workspaceId?: string,
): Promise<MarketplaceSkillContent> {
	return (await Call.ByName('main.App.GetMarketplaceSkillContent', {
		workspaceId: workspaceId ?? '',
		provider: skill.provider,
		externalId: skill.externalId,
		name: skill.name,
		description: skill.description,
		sourceRepo: skill.sourceRepo,
		sourceUrl: skill.sourceUrl,
		listingUrl: skill.listingUrl ?? '',
		rawSkillUrl: skill.rawSkillUrl,
		installCount: skill.installCount ?? null,
	})) as MarketplaceSkillContent;
}

export async function getMarketplaceSkillMetadata(
	skill: MarketplaceSkill,
	workspaceId?: string,
): Promise<MarketplaceSkill> {
	return (await Call.ByName('main.App.GetMarketplaceSkillMetadata', {
		workspaceId: workspaceId ?? '',
		provider: skill.provider,
		externalId: skill.externalId,
		name: skill.name,
		description: skill.description,
		sourceRepo: skill.sourceRepo,
		sourceUrl: skill.sourceUrl,
		listingUrl: skill.listingUrl ?? '',
		rawSkillUrl: skill.rawSkillUrl,
		installCount: skill.installCount ?? null,
	})) as MarketplaceSkill;
}

export async function installMarketplaceSkill(
	input: {
		skill: MarketplaceSkill;
		scope: SkillScope;
		dirName: string;
		tools: string[];
	},
	workspaceId?: string,
): Promise<SkillInfo> {
	return (await Call.ByName('main.App.InstallMarketplaceSkill', {
		workspaceId: workspaceId ?? '',
		provider: input.skill.provider,
		externalId: input.skill.externalId,
		name: input.skill.name,
		description: input.skill.description,
		sourceRepo: input.skill.sourceRepo,
		sourceUrl: input.skill.sourceUrl,
		listingUrl: input.skill.listingUrl ?? '',
		rawSkillUrl: input.skill.rawSkillUrl,
		installCount: input.skill.installCount ?? null,
		scope: input.scope,
		dirName: input.dirName,
		tools: input.tools,
	})) as SkillInfo;
}

export async function attachSkillMarketplaceSource(
	skill: SkillInfo,
	marketplace: MarketplaceSkill,
	workspaceId?: string,
): Promise<void> {
	await Call.ByName('main.App.AttachSkillMarketplaceSource', {
		workspaceId: workspaceId ?? '',
		scope: skill.scope,
		dirName: skill.dirName,
		tools: skill.tools,
		provider: marketplace.provider,
		externalId: marketplace.externalId,
		sourceRepo: marketplace.sourceRepo,
		sourceUrl: marketplace.sourceUrl,
		listingUrl: marketplace.listingUrl ?? '',
		rawSkillUrl: marketplace.rawSkillUrl,
	});
}
