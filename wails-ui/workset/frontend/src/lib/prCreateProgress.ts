export type PrCreateStage = 'generating' | 'creating';

type PrCreateStageCopy = {
	button: string;
	detail: string;
};

const stageCopy: Record<PrCreateStage, PrCreateStageCopy> = {
	generating: {
		button: 'Generating title...',
		detail: 'Step 1/2: Generating title...',
	},
	creating: {
		button: 'Creating PR...',
		detail: 'Step 2/2: Creating PR...',
	},
};

export const getPrCreateStageCopy = (stage: PrCreateStage | null): PrCreateStageCopy | null => {
	if (!stage) return null;
	return stageCopy[stage];
};
