import type {
	CheckAnnotation,
	CheckAnnotationsResponse,
	PullRequestCreated,
	PullRequestGenerated,
	PullRequestReviewComment,
	PullRequestStatusResult,
	RemoteInfo,
} from '../../types';
import {
	CreatePullRequest,
	GeneratePullRequestText,
	GetCheckAnnotations,
	GetPullRequestReviews,
	GetPullRequestStatus,
	GetTrackedPullRequest,
	ListRemotes,
	SendPullRequestReviewsToTerminal,
} from '../../../../wailsjs/go/main/App';
import {
	mapCheckAnnotations,
	mapPullRequest,
	mapPullRequestReviewComments,
	mapPullRequestStatus,
	mapRemoteInfo,
} from './mappers';
import type {
	PullRequestCreateResponse,
	PullRequestReviewsResponse,
	PullRequestStatusResponse,
	RemoteInfoResponse,
	TrackedPullRequestResponse,
} from './types';

export async function createPullRequest(
	workspaceId: string,
	repoId: string,
	payload: {
		title: string;
		body: string;
		base?: string;
		head?: string;
		baseRemote?: string;
		draft: boolean;
		autoCommit?: boolean;
		autoPush?: boolean;
	},
): Promise<PullRequestCreated> {
	const result = (await CreatePullRequest({
		workspaceId,
		repoId,
		title: payload.title,
		body: payload.body,
		base: payload.base ?? '',
		head: payload.head ?? '',
		baseRemote: payload.baseRemote ?? '',
		draft: payload.draft,
		autoCommit: payload.autoCommit ?? false,
		autoPush: payload.autoPush ?? false,
	})) as PullRequestCreateResponse;

	return mapPullRequest(result);
}

export async function listRemotes(workspaceId: string, repoId: string): Promise<RemoteInfo[]> {
	const result = (await ListRemotes({
		workspaceId,
		repoId,
	})) as RemoteInfoResponse[];

	return result.map(mapRemoteInfo);
}

export async function fetchTrackedPullRequest(
	workspaceId: string,
	repoId: string,
): Promise<PullRequestCreated | null> {
	const result = (await GetTrackedPullRequest({
		workspaceId,
		repoId,
	})) as unknown as TrackedPullRequestResponse;

	if (!result.found || !result.pull_request) {
		return null;
	}

	return mapPullRequest(result.pull_request);
}

export async function fetchPullRequestStatus(
	workspaceId: string,
	repoId: string,
	number?: number,
	branch?: string,
): Promise<PullRequestStatusResult> {
	const result = (await GetPullRequestStatus({
		workspaceId,
		repoId,
		number: number ?? 0,
		branch: branch ?? '',
	})) as unknown as PullRequestStatusResponse;

	return mapPullRequestStatus(result);
}

export async function fetchCheckAnnotations(
	owner: string,
	repo: string,
	checkRunId: number,
): Promise<CheckAnnotation[]> {
	const result = (await GetCheckAnnotations({
		owner,
		repo,
		checkRunId,
	})) as unknown as CheckAnnotationsResponse;

	return mapCheckAnnotations(result);
}

export async function fetchPullRequestReviews(
	workspaceId: string,
	repoId: string,
	number?: number,
	branch?: string,
): Promise<PullRequestReviewComment[]> {
	const result = (await GetPullRequestReviews({
		workspaceId,
		repoId,
		number: number ?? 0,
		branch: branch ?? '',
	})) as unknown as PullRequestReviewsResponse;

	return mapPullRequestReviewComments(result.comments ?? []);
}

export async function generatePullRequestText(
	workspaceId: string,
	repoId: string,
): Promise<PullRequestGenerated> {
	const result = (await GeneratePullRequestText({
		workspaceId,
		repoId,
	})) as PullRequestGenerated;

	return result;
}

export async function sendPullRequestReviewsToTerminal(
	workspaceId: string,
	repoId: string,
	number?: number,
	branch?: string,
	terminalId?: string,
): Promise<void> {
	await SendPullRequestReviewsToTerminal({
		workspaceId,
		repoId,
		number: number ?? 0,
		branch: branch ?? '',
		terminalId: terminalId ?? '',
	});
}
