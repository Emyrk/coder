import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import type { AnthropicAgent } from "#/api/typesGenerated";
import { mockApiError } from "#/testHelpers/entities";
import { AnthropicPageView } from "./AnthropicPageView";

const sampleAgents: readonly AnthropicAgent[] = [
	{
		id: "agent_01HZ8K3CODER",
		name: "Coder Reviewer",
		description: "Reviews pull requests against the Coder house style.",
		model: "claude-opus-4-1",
		version: 7,
		archived: false,
		created_at: "2026-01-12T14:23:00Z",
		updated_at: "2026-06-29T09:18:00Z",
	},
	{
		id: "agent_01HZ9M8DOCS",
		name: "Doc writer",
		description: "Drafts release notes from merged PRs.",
		model: "claude-sonnet-4-5",
		version: 2,
		archived: false,
		created_at: "2026-02-04T10:05:00Z",
		updated_at: "2026-05-20T16:42:00Z",
	},
	{
		id: "agent_01HZAR4ARCHIVE",
		name: "Legacy migrator",
		description: "",
		model: "claude-sonnet-4-0",
		version: 1,
		archived: true,
		created_at: "2025-09-30T11:00:00Z",
		updated_at: "2025-11-04T12:13:00Z",
	},
];

const meta: Meta<typeof AnthropicPageView> = {
	title: "pages/UserSettingsPage/AnthropicPageView",
	component: AnthropicPageView,
	args: {
		hasApiKey: true,
		isCheckingKey: false,
		isSavingKey: false,
		isRemovingKey: false,
		agents: sampleAgents,
		isLoadingAgents: false,
		isRefreshingAgents: false,
		isCreatingSession: false,
		isSendingEvent: false,
		onSaveKey: fn(),
		onRemoveKey: fn(),
		onRefreshAgents: fn(),
		onCreateSession: fn(async () => ({
			id: "sesn_01HSTORYBOOK",
			agent_id: "agent_01HZ8K3CODER",
			environment_id: "env_storybook",
			title: "Coder session tester",
			metadata: { coder_user_id: "00000000-0000-0000-0000-000000000000" },
			created_at: "2026-06-30T19:00:00Z",
			coder_user_id: "00000000-0000-0000-0000-000000000000",
		})),
		onSendEvent: fn(async () => ({
			events: [
				{
					id: "sevt_01HSTORYBOOK",
					type: "user.message",
					processed_at: "2026-06-30T19:00:01Z",
				},
			],
		})),
	},
};

export default meta;
type Story = StoryObj<typeof AnthropicPageView>;

/**
 * Default render: the user has an API key configured and the agents
 * call returned a populated list.
 */
export const Configured: Story = {};

/**
 * First-time experience. The user has not pasted a key yet, so the
 * agents section is hidden entirely.
 */
export const NotConfigured: Story = {
	args: {
		hasApiKey: false,
		agents: undefined,
		isLoadingAgents: false,
	},
};

/**
 * Save button is disabled while the input is empty and re-enables
 * once the user types a value.
 */
export const SaveEnablesWhenInputHasValue: Story = {
	args: {
		hasApiKey: false,
		agents: undefined,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const saveButton = await canvas.findByRole("button", { name: /save key/i });
		expect(saveButton).toBeDisabled();

		const input = canvas.getByLabelText(/api key/i);
		await userEvent.type(input, "sk-ant-api03-fixture");
		expect(saveButton).toBeEnabled();
	},
};

/**
 * Loading state while we wait for the agents-list call to come back
 * for the first time after the key was saved.
 */
export const LoadingAgents: Story = {
	args: {
		agents: undefined,
		isLoadingAgents: true,
	},
};

/**
 * The key is set but Anthropic returned an error (typically a 401
 * for an invalid key, or 404 if the org is not configured for the
 * integration).
 */
export const AgentsError: Story = {
	args: {
		agents: undefined,
		isLoadingAgents: false,
		agentsError: mockApiError({
			message: "Anthropic rejected the agents list request.",
			detail: "401 unauthorized",
		}),
	},
};

/**
 * Save attempt failed: surface the error in-line above the form.
 */
export const KeySaveError: Story = {
	args: {
		hasApiKey: false,
		agents: undefined,
		keyMutationError: mockApiError({
			message: "Failed to save Anthropic API key.",
			detail: "secret length exceeds limit",
		}),
	},
};

/**
 * Empty agent list when the user's workspace exists but has no
 * agents created yet. The table still renders so the user sees
 * what shape data is expected.
 */
export const NoAgents: Story = {
	args: {
		agents: [],
	},
};

/**
 * Session-tester panel ready for a create+send round-trip. Render
 * confirms the panel is mounted alongside the agents list whenever
 * agents are available.
 */
export const SessionTesterReady: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const sendButton = await canvas.findByRole("button", {
			name: /send user message/i,
		});
		expect(sendButton).toBeDisabled();
	},
};

/**
 * Session-tester surfacing a send-event error from the backend so
 * operators can iterate on payloads without leaving the page.
 */
export const SessionTesterSendError: Story = {
	args: {
		sendEventError: mockApiError({
			message: "Anthropic rejected the send-event request.",
			detail: "Cannot send events to archived session",
		}),
	},
};
