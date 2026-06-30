import { BotIcon, KeyRoundIcon, RefreshCwIcon, TrashIcon } from "lucide-react";
import { type FC, type FormEvent, useId, useState } from "react";
import type { AnthropicAgent } from "#/api/typesGenerated";
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

	onSaveKey: (value: string) => Promise<void> | void;
	onRemoveKey: () => Promise<void> | void;
	onRefreshAgents: () => void;
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
	onSaveKey,
	onRemoveKey,
	onRefreshAgents,
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
