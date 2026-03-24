export interface AppDefinition {
	id: string;
	name: string;
	route: string;
	icon: 'document';
	description: string;
}

export const APPS: AppDefinition[] = [
	{
		id: 'pdf-view',
		name: 'PDF View',
		route: '/apps/pdf-view',
		icon: 'document',
		description: 'Chat about PDFs'
	}
];
