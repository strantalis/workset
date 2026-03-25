import type { Extension } from '@codemirror/state';
import type { RepoFileDefinitionTarget } from '../../types';
import { getRepoFileDefinition, getRepoFileHover } from '../../api/repo-files';
import { semanticDefinitionExtension } from '../editor/semanticDefinition';
import { semanticHoverExtension } from '../editor/semanticHover';

export function createRepoSemanticHoverExtensions(
	workspaceId: string,
	repoId: string | null,
	filePath: string | null,
	onDefinitionNavigate?: ((target: RepoFileDefinitionTarget) => void) | null,
): Extension[] {
	if (!workspaceId || !repoId || !filePath) return [];
	const extensions: Extension[] = [
		semanticHoverExtension({
			filePath,
			fetchHover: ({ content, line, character }) =>
				getRepoFileHover({
					workspaceId,
					repoId,
					path: filePath,
					content,
					line,
					character,
				}),
		}),
	];
	if (onDefinitionNavigate) {
		extensions.push(
			semanticDefinitionExtension({
				filePath,
				currentRepoId: repoId,
				fetchDefinition: ({ content, line, character }) =>
					getRepoFileDefinition({
						workspaceId,
						repoId,
						path: filePath,
						content,
						line,
						character,
					}),
				onNavigate: onDefinitionNavigate,
			}),
		);
	}
	return extensions;
}
