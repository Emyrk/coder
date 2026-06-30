import { API } from "#/api/api";

// All Anthropic-related query keys nest under "anthropic" so a single
// invalidateQueries({queryKey: ["anthropic"]}) can flush every cached
// piece of the integration after, for example, the user updates their
// API key.

const anthropicAgentsKey = (organization: string, userId: string) => [
	"anthropic",
	organization,
	"agents",
	userId,
];

/**
 * agentsForUser fetches the Anthropic agents visible to the user's
 * stored ANTHROPIC_API_KEY user secret. The query is disabled until
 * the caller passes both organization and userId so the UI never
 * fires a request with placeholder values.
 */
export const agentsForUser = (organization: string, userId: string) => {
	return {
		queryKey: anthropicAgentsKey(organization, userId),
		queryFn: () => API.getAnthropicAgents(organization, userId),
	};
};
