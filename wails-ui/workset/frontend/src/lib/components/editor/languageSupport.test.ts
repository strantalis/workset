import { describe, expect, it } from 'vitest';
import { loadLanguage } from './languageSupport';

describe('loadLanguage', () => {
	it('loads Terraform and HCL language support for supported file paths', async () => {
		await expect(loadLanguage('infra/main.tf')).resolves.not.toBeNull();
		await expect(loadLanguage('infra/dev.tfvars')).resolves.not.toBeNull();
		await expect(loadLanguage('generic/settings.hcl')).resolves.not.toBeNull();
		await expect(loadLanguage('stacks/api.tfcomponent.hcl')).resolves.not.toBeNull();
		await expect(loadLanguage('deploy/prod.tfdeploy.hcl')).resolves.not.toBeNull();
		await expect(loadLanguage('queries/network.tfquery.hcl')).resolves.not.toBeNull();
	});

	it('returns null for unsupported files', async () => {
		await expect(loadLanguage('README.txt')).resolves.toBeNull();
	});
});
