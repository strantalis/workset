import { describe, expect, it } from 'vitest';
import { getRepoFileIcon } from './fileIcons';

describe('getRepoFileIcon', () => {
	it('returns the Terraform icon for Terraform files', () => {
		expect(getRepoFileIcon('infra/main.tf')).toBe('file-icons:terraform');
		expect(getRepoFileIcon('infra/dev.tfvars')).toBe('file-icons:terraform');
		expect(getRepoFileIcon('stacks/api.tfcomponent.hcl')).toBe('file-icons:terraform');
		expect(getRepoFileIcon('deploy/prod.tfdeploy.hcl')).toBe('file-icons:terraform');
		expect(getRepoFileIcon('queries/network.tfquery.hcl')).toBe('file-icons:terraform');
	});

	it('preserves non-Terraform icon mappings', () => {
		expect(getRepoFileIcon('src/main.ts')).toBe('file-icons:typescript');
		expect(getRepoFileIcon('Dockerfile')).toBe('file-icons:docker');
	});
});
