import type { PageServerLoad } from './$types.js';

interface GitHubRelease {
	tag_name: string;
}

interface GitHubMilestone {
	number: number;
	title: string;
	due_on: string | null;
	open_issues: number;
	closed_issues: number;
}

export const load: PageServerLoad = async ({ fetch }) => {
	let release: string | null = null;
	let milestone: { number: number; title: string; progress: number } | null = null;

	try {
		const res = await fetch(
			'https://api.github.com/repos/mrlm-net/simconnect/releases/latest'
		);
		if (res.ok) {
			const data: GitHubRelease = await res.json();
			release = data.tag_name;
		}
	} catch {
		// Build continues without release data
	}

	try {
		const res = await fetch(
			'https://api.github.com/repos/mrlm-net/simconnect/milestones?state=open&sort=due_on&direction=asc'
		);
		if (res.ok) {
			const milestones: GitHubMilestone[] = await res.json();
			const candidates = milestones
				.filter((m) => m.due_on !== null && m.open_issues + m.closed_issues > 0)
				.sort((a, b) => a.title.localeCompare(b.title, undefined, { numeric: true }));
			if (candidates.length > 0) {
				const m = candidates[0];
				const total = m.open_issues + m.closed_issues;
				milestone = {
					number: m.number,
					title: m.title,
					progress: Math.round((m.closed_issues / total) * 100)
				};
			}
		}
	} catch {
		// Build continues without milestone data
	}

	return { release, milestone };
};
