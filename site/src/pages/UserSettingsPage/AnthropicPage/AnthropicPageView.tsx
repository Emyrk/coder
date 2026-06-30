import {
	BotIcon,
	KeyRoundIcon,
	RefreshCwIcon,
	SendIcon,
	TrashIcon,
} from "lucide-react";
import { type FC, type FormEvent, useId, useState } from "react";
import type {
	AnthropicAgent,
	AnthropicSession,
	SendAnthropicEventResponse,
} from "#/api/typesGenerated";
import { ErrorAlert } from "#/components/Alert/ErrorAlert";
import { Badge } from "#/components/Badge/Badge";
import { Button } from "#/components/Button/Button";
import { FeatureStageBadge } from "#/components/FeatureStageBadge/FeatureStageBadge";
import { Input } from "#/components/Input/Input";
import { Label } from "#/components/Label/Label";
import { Loader } from "#/components/Loader/Loader";
import {
	SettingsHeader,
	SettingsHeaderDescription,
	SettingsHeaderTitle,
} from "#/components/SettingsHeader/SettingsHeader";
import { Spinner } from "#/components/Spinner/Spinner";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "#/components/Table/Table";
import { TableEmpty } from "#/components/TableEmpty/TableEmpty";

type AnthropicPageViewProps = {
	/** Whether the user already has an Anthropic API key user-secret. */
	hasApiKey: boolean;
	/** Loading state for the initial secret lookup. */
	isCheckingKey: boolean;
	/** Mutation in-flight indicator for saving the key. */
	isSavingKey: boolean;
	/** Mutation in-flight indicator for removing the key. */
	isRemovingKey: boolean;
	/** Most recent save/remove error to surface in-line. */
	keyMutationError?: unknown;

	/** Agents the user's key can see; undefined while loading or when key is unset. */
	agents?: readonly AnthropicAgent[];
	/** Whether the agents query is in flight (initial or refetch). */
	isLoadingAgents: boolean;
	/** Whether the agents query is refreshing in the background. */
	isRefreshingAgents: boolean;
	/** Error from the most recent agents fetch, if any. */
	agentsError?: unknown;

	/** Session-create mutation in-flight indicator. */
	isCreatingSession: boolean;
	/** Most recent create-session error to surface in the tester. */
	createSessionError?: unknown;
	/** Send-event mutation in-flight indicator. */
	isSendingEvent: boolean;
	/** Most recent send-event error to surface in the tester. */
	sendEventError?: unknown;

	onSaveKey: (value: string) => Promise<void> | void;
	onRemoveKey: () => Promise<void> | void;
	onRefreshAgents: () => void;
	onCreateSession: (agentId: string) => Promise<AnthropicSession>;
	onSendEvent: (
		sessionId: string,
		text: string,
	) => Promise<SendAnthropicEventResponse>;
};

/**
 * AnthropicPageView is the presentational layer for the Settings ->
 * Anthropic tab. It is intentionally side-effect free so Storybook
 * stories can render every state combination without touching the
 * network.
 */
export const AnthropicPageView: FC<AnthropicPageViewProps> = ({
	hasApiKey,
	isCheckingKey,
	isSavingKey,
	isRemovingKey,
	keyMutationError,
	agents,
	isLoadingAgents,
	isRefreshingAgents,
	agentsError,
	isCreatingSession,
	createSessionError,
	isSendingEvent,
	sendEventError,
	onSaveKey,
	onRemoveKey,
	onRefreshAgents,
	onCreateSession,
	onSendEvent,
}) => {
	return (
		<div className="flex flex-col gap-8">
			<SettingsHeader>
				<SettingsHeaderTitle>
					<span className="flex items-center gap-2">
						Anthropic
						<FeatureStageBadge contentType="beta" size="sm" />
					</span>
				</SettingsHeaderTitle>
				<SettingsHeaderDescription>
					Connect Coder to your Anthropic workspace so you can launch managed
					Claude sessions from Coder. Your API key is stored as a user secret
					named <code>ANTHROPIC_API_KEY</code> and is only used for requests
					made on your behalf.
				</SettingsHeaderDescription>
			</SettingsHeader>

			<ApiKeyCard
				hasApiKey={hasApiKey}
				isCheckingKey={isCheckingKey}
				isSavingKey={isSavingKey}
				isRemovingKey={isRemovingKey}
				mutationError={keyMutationError}
				onSave={onSaveKey}
				onRemove={onRemoveKey}
			/>

			{hasApiKey && (
				<AgentsSection
					agents={agents}
					isLoading={isLoadingAgents}
					isRefreshing={isRefreshingAgents}
					error={agentsError}
					onRefresh={onRefreshAgents}
				/>
			)}

			{hasApiKey && agents && agents.length > 0 && (
				<SessionTesterSection
					agents={agents}
					isCreatingSession={isCreatingSession}
					createSessionError={createSessionError}
					isSendingEvent={isSendingEvent}
					sendEventError={sendEventError}
					onCreateSession={onCreateSession}
					onSendEvent={onSendEvent}
				/>
			)}
		</div>
	);
};

type ApiKeyCardProps = {
	hasApiKey: boolean;
	isCheckingKey: boolean;
	isSavingKey: boolean;
	isRemovingKey: boolean;
	mutationError?: unknown;
	onSave: (value: string) => Promise<void> | void;
	onRemove: () => Promise<void> | void;
};

const ApiKeyCard: FC<ApiKeyCardProps> = ({
	hasApiKey,
	isCheckingKey,
	isSavingKey,
	isRemovingKey,
	mutationError,
	onSave,
	onRemove,
}) => {
	const inputId = useId();
	const [draft, setDraft] = useState("");
	const trimmed = draft.trim();
	const submitDisabled = isSavingKey || isRemovingKey || trimmed.length === 0;

	const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (trimmed.length === 0) {
			return;
		}
		void (async () => {
			await onSave(trimmed);
			setDraft("");
		})();
	};

	return (
		<section className="flex flex-col gap-4 rounded-lg border border-border bg-surface-secondary p-6">
			<div className="flex items-start gap-3">
				<KeyRoundIcon
					aria-hidden
					className="mt-1 size-5 shrink-0 text-content-secondary"
				/>
				<div className="flex flex-1 flex-col gap-1">
					<h2 className="m-0 text-base font-semibold">API key</h2>
					<p className="m-0 text-sm text-content-secondary">
						Generate an API key at{" "}
						<a
							className="font-medium text-content-link underline"
							href="https://console.anthropic.com/settings/keys"
							rel="noreferrer"
							target="_blank"
						>
							console.anthropic.com
						</a>{" "}
						and paste it below. Coder never reads the value back, so saving an
						empty key is not possible; use Remove to clear it.
					</p>
				</div>
				<KeyStatusBadge hasApiKey={hasApiKey} isChecking={isCheckingKey} />
			</div>

			{mutationError ? <ErrorAlert error={mutationError} /> : null}

			<form className="flex flex-col gap-3" onSubmit={handleSubmit}>
				<div className="flex flex-col gap-2">
					<Label htmlFor={inputId}>
						{hasApiKey ? "Replace API key" : "API key"}
					</Label>
					<Input
						aria-describedby={`${inputId}-help`}
						autoComplete="off"
						id={inputId}
						onChange={(event) => setDraft(event.target.value)}
						placeholder="sk-ant-api03-..."
						spellCheck={false}
						type="password"
						value={draft}
					/>
					<p
						className="m-0 text-xs text-content-secondary"
						id={`${inputId}-help`}
					>
						Stored encrypted as the user secret <code>ANTHROPIC_API_KEY</code>.
						You can review or remove it on this page at any time.
					</p>
				</div>
				<div className="flex flex-wrap items-center gap-2">
					<Button disabled={submitDisabled} type="submit">
						<Spinner loading={isSavingKey} />
						{hasApiKey ? "Replace key" : "Save key"}
					</Button>
					{hasApiKey && (
						<Button
							disabled={isSavingKey || isRemovingKey}
							onClick={() => void onRemove()}
							type="button"
							variant="outline"
						>
							<Spinner loading={isRemovingKey}>
								<TrashIcon aria-hidden className="size-4" />
							</Spinner>
							Remove key
						</Button>
					)}
				</div>
			</form>
		</section>
	);
};

type KeyStatusBadgeProps = {
	hasApiKey: boolean;
	isChecking: boolean;
};

const KeyStatusBadge: FC<KeyStatusBadgeProps> = ({ hasApiKey, isChecking }) => {
	if (isChecking) {
		return (
			<Badge size="sm" variant="default">
				Checking
			</Badge>
		);
	}
	if (hasApiKey) {
		return (
			<Badge size="sm" variant="default">
				Configured
			</Badge>
		);
	}
	return (
		<Badge size="sm" variant="warning">
			Not configured
		</Badge>
	);
};

type AgentsSectionProps = {
	agents?: readonly AnthropicAgent[];
	isLoading: boolean;
	isRefreshing: boolean;
	error?: unknown;
	onRefresh: () => void;
};

const AgentsSection: FC<AgentsSectionProps> = ({
	agents,
	isLoading,
	isRefreshing,
	error,
	onRefresh,
}) => {
	return (
		<section className="flex flex-col gap-4 rounded-lg border border-border bg-surface-secondary p-6">
			<div className="flex items-start gap-3">
				<BotIcon
					aria-hidden
					className="mt-1 size-5 shrink-0 text-content-secondary"
				/>
				<div className="flex flex-1 flex-col gap-1">
					<h2 className="m-0 text-base font-semibold">Available agents</h2>
					<p className="m-0 text-sm text-content-secondary">
						Agents your API key can see in your Anthropic workspace. Sessions
						created from Coder bind to one of these.
					</p>
				</div>
				<Button
					disabled={isLoading || isRefreshing}
					onClick={onRefresh}
					size="sm"
					variant="outline"
				>
					<Spinner loading={isRefreshing}>
						<RefreshCwIcon aria-hidden className="size-4" />
					</Spinner>
					Refresh
				</Button>
			</div>

			{error ? <ErrorAlert error={error} /> : null}

			{isLoading && !agents ? (
				<Loader />
			) : (
				<Table aria-label="Anthropic agents">
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead>Model</TableHead>
							<TableHead>Version</TableHead>
							<TableHead className="w-32">Status</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{agents && agents.length > 0 ? (
							agents.map((agent) => (
								<TableRow key={agent.id}>
									<TableCell>
										<div className="flex flex-col gap-0.5">
											<span className="font-medium">
												{agent.name || "Untitled agent"}
											</span>
											<span className="text-xs text-content-secondary">
												{agent.id}
											</span>
											{agent.description ? (
												<span className="text-xs text-content-secondary">
													{agent.description}
												</span>
											) : null}
										</div>
									</TableCell>
									<TableCell className="font-mono text-xs">
										{agent.model || "N/A"}
									</TableCell>
									<TableCell>v{agent.version}</TableCell>
									<TableCell>
										{agent.archived ? (
											<Badge size="sm" variant="warning">
												Archived
											</Badge>
										) : (
											<Badge size="sm" variant="default">
												Active
											</Badge>
										)}
									</TableCell>
								</TableRow>
							))
						) : (
							<TableEmpty
								message="No agents yet"
								description="Create an agent at console.anthropic.com to see it here."
							/>
						)}
					</TableBody>
				</Table>
			)}
		</section>
	);
};

type SessionTesterSectionProps = {
	agents: readonly AnthropicAgent[];
	isCreatingSession: boolean;
	createSessionError?: unknown;
	isSendingEvent: boolean;
	sendEventError?: unknown;
	onCreateSession: (agentId: string) => Promise<AnthropicSession>;
	onSendEvent: (
		sessionId: string,
		text: string,
	) => Promise<SendAnthropicEventResponse>;
};

/**
 * SessionTesterSection lets an operator drive the Anthropic session
 * pipeline end-to-end from the Coder UI. The flow mirrors the backend
 * smoke test: pick an agent, create a session, send a user-message
 * event. The poller logs the resulting work item server-side; this
 * panel only confirms the Anthropic-side handshake succeeded.
 */
const SessionTesterSection: FC<SessionTesterSectionProps> = ({
	agents,
	isCreatingSession,
	createSessionError,
	isSendingEvent,
	sendEventError,
	onCreateSession,
	onSendEvent,
}) => {
	const agentSelectId = useId();
	const sessionInputId = useId();
	const textareaId = useId();
	const activeAgents = agents.filter((a) => !a.archived);
	const firstAgentId = activeAgents[0]?.id ?? agents[0]?.id ?? "";
	const [agentId, setAgentId] = useState(firstAgentId);
	const [sessionId, setSessionId] = useState("");
	const [eventText, setEventText] = useState("");
	const [lastSent, setLastSent] = useState<{
		eventId: string;
		at: Date;
	} | null>(null);

	const trimmedSession = sessionId.trim();
	const trimmedText = eventText.trim();
	const canCreate = !!agentId && !isCreatingSession;
	const canSend =
		trimmedSession.length > 0 && trimmedText.length > 0 && !isSendingEvent;

	const handleCreate = () => {
		if (!canCreate) {
			return;
		}
		void (async () => {
			const created = await onCreateSession(agentId);
			setSessionId(created.id);
			setLastSent(null);
		})();
	};

	const handleSend = (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (!canSend) {
			return;
		}
		void (async () => {
			const sent = await onSendEvent(trimmedSession, trimmedText);
			const first = sent.events[0];
			if (first) {
				setLastSent({ eventId: first.id, at: new Date() });
			}
			setEventText("");
		})();
	};

	return (
		<section className="flex flex-col gap-4 rounded-lg border border-border bg-surface-secondary p-6">
			<div className="flex items-start gap-3">
				<SendIcon
					aria-hidden
					className="mt-1 size-5 shrink-0 text-content-secondary"
				/>
				<div className="flex flex-1 flex-col gap-1">
					<h2 className="m-0 text-base font-semibold">
						Test the session pipeline
					</h2>
					<p className="m-0 text-sm text-content-secondary">
						Create a managed-agent session bound to one of your agents, or paste
						an existing session ID, then send a user message. The Coder server's
						poller will claim the resulting work item; check the server log to
						confirm.
					</p>
				</div>
			</div>

			{createSessionError ? <ErrorAlert error={createSessionError} /> : null}

			<div className="flex flex-col gap-2">
				<Label htmlFor={agentSelectId}>Agent</Label>
				<select
					className="h-9 rounded-md border border-border bg-surface-primary px-3 text-sm"
					id={agentSelectId}
					onChange={(event) => setAgentId(event.target.value)}
					value={agentId}
				>
					{agents.map((agent) => (
						<option key={agent.id} value={agent.id}>
							{(agent.name || "Untitled agent") +
								(agent.archived ? " (archived)" : "") +
								" (" +
								agent.id +
								")"}
						</option>
					))}
				</select>
			</div>

			<div className="flex flex-wrap items-center gap-3">
				<Button disabled={!canCreate} onClick={handleCreate} type="button">
					<Spinner loading={isCreatingSession} />
					{trimmedSession === "" ? "Create session" : "Create another session"}
				</Button>
			</div>

			<div className="flex flex-col gap-2">
				<Label htmlFor={sessionInputId}>Session ID</Label>
				<Input
					aria-describedby={`${sessionInputId}-help`}
					autoComplete="off"
					id={sessionInputId}
					onChange={(event) => setSessionId(event.target.value)}
					placeholder="sesn_..."
					spellCheck={false}
					value={sessionId}
				/>
				<p
					className="m-0 text-xs text-content-secondary"
					id={`${sessionInputId}-help`}
				>
					Auto-filled when you create a session above. Paste any
					<code className="px-0.5">sesn_...</code>
					ID here to send to a different session.
				</p>
			</div>

			{sendEventError ? <ErrorAlert error={sendEventError} /> : null}

			<form className="flex flex-col gap-2" onSubmit={handleSend}>
				<Label htmlFor={textareaId}>User message</Label>
				<textarea
					className="min-h-20 rounded-md border border-border bg-surface-primary p-2 text-sm font-mono"
					disabled={trimmedSession === ""}
					id={textareaId}
					onChange={(event) => setEventText(event.target.value)}
					placeholder={
						trimmedSession === ""
							? "Set a session ID above to enable sending."
							: "Tell the agent what to do."
					}
					value={eventText}
				/>
				<div className="flex flex-wrap items-center gap-3">
					<Button disabled={!canSend} type="submit">
						<Spinner loading={isSendingEvent} />
						Send user message
					</Button>
					{lastSent ? (
						<span className="text-xs text-content-secondary">
							Last event:
							<span className="font-mono">{lastSent.eventId}</span> at{" "}
							{lastSent.at.toLocaleTimeString()}
						</span>
					) : null}
				</div>
			</form>
		</section>
	);
};
