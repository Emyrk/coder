import type { FC } from "react";
import { useMutation, useQuery, useQueryClient } from "react-query";
import { toast } from "sonner";
import { API } from "#/api/api";
import { getErrorMessage } from "#/api/errors";
import { agentsForUser } from "#/api/queries/anthropic";
import {
	createUserSecret,
	deleteUserSecret,
	updateUserSecret,
	userSecrets,
} from "#/api/queries/userSecrets";
import {
	AnthropicAPIKeySecretName,
	type AnthropicSession,
	type SendAnthropicEventResponse,
} from "#/api/typesGenerated";
import { useAuthenticated } from "#/hooks/useAuthenticated";
import { useDashboard } from "#/modules/dashboard/useDashboard";
import { AnthropicPageView } from "./AnthropicPageView";

/**
 * AnthropicPage is the container for the Settings -> Anthropic tab. It
 * wires three react-query primitives:
 *
 *   - user_secrets list, filtered down to ANTHROPIC_API_KEY presence.
 *   - create/update/delete of that single secret.
 *   - agents-for-user list, only enabled once the key is present.
 *
 * Anthropic integration is per-organization; the page targets the
 * first organization in the user's session. Multi-org pickers can
 * land in a follow-up once we know which org runs the integration.
 */
const AnthropicPage: FC = () => {
	const { user: me } = useAuthenticated();
	const { organizations } = useDashboard();
	const queryClient = useQueryClient();

	// Anthropic integration is per-org. We pick the first org in the
	// user's session because the PoC only supports one Anthropic env
	// per Coder org and most deployments have a single "default" org.
	const organization = organizations[0]?.id ?? "default";

	const secretsQueryOptions = userSecrets(me.id);
	const secretsQuery = useQuery(secretsQueryOptions);
	const existingSecret = secretsQuery.data?.find(
		(s) => s.name === AnthropicAPIKeySecretName,
	);
	const hasApiKey = existingSecret !== undefined;

	const createMutation = useMutation(createUserSecret(queryClient, me.id));
	const updateMutation = useMutation(updateUserSecret(queryClient, me.id));
	const deleteMutation = useMutation(deleteUserSecret(queryClient, me.id));

	const agentsQueryOptions = agentsForUser(organization, me.id);
	const agentsQuery = useQuery({
		...agentsQueryOptions,
		// Hold the agents call until the key exists, otherwise the user
		// would see a 412 on first load every time.
		enabled: hasApiKey,
	});

	const saveKey = async (value: string) => {
		try {
			if (existingSecret) {
				await updateMutation.mutateAsync({
					name: existingSecret.name,
					request: { value },
				});
				toast.success("Anthropic API key updated.");
			} else {
				await createMutation.mutateAsync({
					name: AnthropicAPIKeySecretName,
					value,
					description: "Anthropic platform API key used by Coder.",
					env_name: "",
					file_path: "",
				});
				toast.success("Anthropic API key saved.");
			}
			await queryClient.invalidateQueries({
				queryKey: agentsQueryOptions.queryKey,
			});
		} catch (error) {
			toast.error(getErrorMessage(error, "Failed to save Anthropic API key."));
		}
	};

	const removeKey = async () => {
		if (!existingSecret) {
			return;
		}
		try {
			await deleteMutation.mutateAsync(existingSecret.name);
			toast.success("Anthropic API key removed.");
			await queryClient.invalidateQueries({
				queryKey: agentsQueryOptions.queryKey,
			});
		} catch (error) {
			toast.error(
				getErrorMessage(error, "Failed to remove Anthropic API key."),
			);
		}
	};

	const refreshAgents = () => {
		void queryClient.invalidateQueries({
			queryKey: agentsQueryOptions.queryKey,
		});
	};

	const createSessionMutation = useMutation<AnthropicSession, unknown, string>({
		mutationFn: (agentId: string) =>
			API.createAnthropicSession(organization, me.id, {
				agent_id: agentId,
				title: "Coder session tester",
			}),
		onSuccess: (session) => {
			toast.success(`Anthropic session ${session.id} created.`);
		},
		onError: (error) => {
			toast.error(
				getErrorMessage(error, "Failed to create Anthropic session."),
			);
		},
	});

	const sendEventMutation = useMutation<
		SendAnthropicEventResponse,
		unknown,
		{ sessionId: string; text: string }
	>({
		mutationFn: ({ sessionId, text }) =>
			API.sendAnthropicEvent(organization, me.id, sessionId, { text }),
		onSuccess: (response) => {
			const id = response.events[0]?.id;
			toast.success(
				id
					? `Sent event ${id} to Anthropic.`
					: "Event sent, but Anthropic returned no event id.",
			);
		},
		onError: (error) => {
			toast.error(getErrorMessage(error, "Failed to send Anthropic event."));
		},
	});

	const mutationError =
		createMutation.error ?? updateMutation.error ?? deleteMutation.error;

	return (
		<AnthropicPageView
			hasApiKey={hasApiKey}
			isCheckingKey={secretsQuery.isLoading}
			isSavingKey={createMutation.isPending || updateMutation.isPending}
			isRemovingKey={deleteMutation.isPending}
			keyMutationError={mutationError ?? undefined}
			agents={agentsQuery.data?.agents}
			isLoadingAgents={agentsQuery.isFetching && !agentsQuery.data}
			isRefreshingAgents={
				agentsQuery.isFetching && agentsQuery.data !== undefined
			}
			agentsError={agentsQuery.error ?? undefined}
			isCreatingSession={createSessionMutation.isPending}
			createSessionError={createSessionMutation.error ?? undefined}
			isSendingEvent={sendEventMutation.isPending}
			sendEventError={sendEventMutation.error ?? undefined}
			onSaveKey={saveKey}
			onRemoveKey={removeKey}
			onRefreshAgents={refreshAgents}
			onCreateSession={(agentId) => createSessionMutation.mutateAsync(agentId)}
			onSendEvent={(sessionId, text) =>
				sendEventMutation.mutateAsync({ sessionId, text })
			}
		/>
	);
};

export default AnthropicPage;
