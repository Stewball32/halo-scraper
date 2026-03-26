<script lang="ts">
	import { getSnapshots, getTicks, getEvents, getMessageRate, getConnected } from '$lib/stores/websocket.svelte';
	import PlayerTable from '$lib/components/game/PlayerTable.svelte';
	import Minimap from '$lib/components/game/Minimap.svelte';
	import EventLog from '$lib/components/game/EventLog.svelte';
	import StatusBadge from '$lib/components/shared/StatusBadge.svelte';
	import type { SnapshotPayload, TickPlayer } from '$lib/types';

	let selectedHost = $state<string | null>(null);
	let showEventLog = $state(true);

	let instances = $derived(Object.keys(getSnapshots()).sort());
	let activeHost = $derived(selectedHost && instances.includes(selectedHost) ? selectedHost : null);

	// Auto-select first host when none is selected or selected host disappeared.
	$effect(() => {
		if (!activeHost && instances.length > 0) {
			selectedHost = instances[0];
		}
	});

	let snapshot = $derived<SnapshotPayload | null>(activeHost ? getSnapshots()[activeHost] ?? null : null);
	let tickPayload = $derived(activeHost ? getTicks()[activeHost] ?? null : null);

	let tickPlayerMap = $derived<Record<number, TickPlayer>>(() => {
		const m: Record<number, TickPlayer> = {};
		if (tickPayload) {
			for (const p of tickPayload.players) {
				m[p.index] = p;
			}
		}
		return m;
	});

	function teamScore(team: number): number {
		if (!snapshot?.team_scores) return 0;
		return snapshot.team_scores.find((t) => t.team === team)?.score ?? 0;
	}

	// Power items
	let powerItems = $derived(tickPayload?.power_items ?? []);
	let powerSpawns = $derived(snapshot?.power_item_spawns ?? []);

	function powerItemTag(spawnId: number): string {
		const s = powerSpawns.find((sp) => sp.spawn_id === spawnId);
		return s?.tag ?? `#${spawnId}`;
	}

	function powerItemInfo(pi: { status: string; held_by: number | null; respawn_in_ticks: number | null }): string {
		if (pi.status === 'held' && pi.held_by != null) {
			const name = snapshot?.players.find((p) => p.index === pi.held_by)?.name ?? `P${pi.held_by}`;
			return name;
		}
		if (pi.status === 'respawning' && pi.respawn_in_ticks != null) {
			return `${Math.ceil(pi.respawn_in_ticks / 30)}s`;
		}
		return '';
	}
</script>

<svelte:head>
	<title>Halo Scraper — Game</title>
</svelte:head>

<div class="flex h-full flex-col overflow-hidden">
	<!-- Header bar -->
	<div class="flex shrink-0 items-center gap-2 border-b border-border bg-[#0a0a14] px-2 py-1">
		<span class="text-xs font-bold uppercase tracking-wider text-hgold">Halo Scraper</span>
		<span class="text-[11px] {getConnected() ? 'text-hgreen' : 'text-hred'}">
			{getConnected() ? '● connected' : '○ disconnected'}
		</span>
		<span class="text-[11px] text-dim">{getMessageRate()} msg/s</span>
	</div>

	<!-- Host tabs -->
	<div class="flex shrink-0 items-end gap-0.5 overflow-x-auto border-b border-border bg-[#0a0a14] px-2">
		{#each instances as inst}
			<button
				class="shrink-0 rounded-t border border-b-0 px-3 py-1 text-[11px] uppercase tracking-wide transition-colors
					{activeHost === inst
					? 'border-border bg-card text-hgold'
					: 'border-transparent text-dim hover:bg-card hover:text-text'}"
				onclick={() => (selectedHost = inst)}
			>
				{inst}
			</button>
		{/each}
		{#if instances.length === 0}
			<span class="px-3 py-1 text-[11px] text-dim">Waiting for data...</span>
		{/if}
	</div>

	{#if activeHost && snapshot}
		<!-- Instance header -->
		<div class="flex shrink-0 flex-wrap items-center gap-2 border-b border-border bg-card2 px-2 py-1">
			<span class="text-[11px] text-dim">{activeHost}</span>
			<StatusBadge state={snapshot.game_state} />
			<span class="text-[11px] text-text">{snapshot.map}</span>
			<span class="text-[10px] text-hcyan">{snapshot.gametype}</span>
			{#if snapshot.is_team_game}
				<span class="ml-auto flex items-center gap-2 text-sm">
					<span class="text-hred">{teamScore(0)}</span>
					<span class="text-dim">—</span>
					<span class="text-hblue">{teamScore(1)}</span>
				</span>
			{/if}
		</div>

		<!-- Main content — single scrollable column -->
		<div class="min-h-0 flex-1 overflow-y-auto">
			<!-- Minimap -->
			<div class="border-b border-border p-1">
				<div class="h-48 md:h-80">
					<Minimap
						players={snapshot.players}
						tickPlayers={tickPlayerMap()}
						{powerItems}
						{powerSpawns}
						isTeamGame={snapshot.is_team_game}
					/>
				</div>
			</div>

			<!-- Power items -->
			{#if powerItems.length > 0}
				<div class="flex shrink-0 flex-wrap items-center gap-1 border-b border-border px-2 py-1">
					<span class="text-[9px] uppercase text-dim">Items</span>
					{#each powerItems as pi}
						<span
							class="flex items-center gap-1 rounded border px-1.5 py-0.5 text-[10px]
								{pi.status === 'world'
								? 'border-[#1a4a2a] bg-[#0f2b1a]'
								: pi.status === 'held'
									? 'border-[#4a4000] bg-[#2a2500]'
									: 'border-[#4a1a1a] bg-[#2b0f0f]'}"
						>
							<span
								class="inline-block h-1 w-1 rounded-full
									{pi.status === 'world'
									? 'bg-hgreen'
									: pi.status === 'held'
										? 'bg-hyellow'
										: 'bg-hred'}"
							></span>
							<span class="text-text">{powerItemTag(pi.spawn_id)}</span>
							{#if powerItemInfo(pi)}
								<span class="text-[9px] text-dim">{powerItemInfo(pi)}</span>
							{/if}
						</span>
					{/each}
				</div>
			{/if}

			<!-- Player tables -->
			{#if snapshot.is_team_game}
				<!-- Red team -->
				<div class="border-b border-border">
					<div class="flex items-center gap-2 border-b border-[#3a1515] bg-[#1a0808] px-2 py-1">
						<span class="text-[11px] font-bold text-hred">Red</span>
						<span class="ml-auto text-sm text-hred">{teamScore(0)}</span>
					</div>
					<PlayerTable
						players={snapshot.players}
						tickPlayers={tickPlayerMap()}
						teamFilter={0}
						isTeamGame={true}
						gametype={snapshot.gametype}
					/>
				</div>
				<!-- Blue team -->
				<div class="border-b border-border">
					<div class="flex items-center gap-2 border-b border-[#153060] bg-[#080f1a] px-2 py-1">
						<span class="text-[11px] font-bold text-hblue">Blue</span>
						<span class="ml-auto text-sm text-hblue">{teamScore(1)}</span>
					</div>
					<PlayerTable
						players={snapshot.players}
						tickPlayers={tickPlayerMap()}
						teamFilter={1}
						isTeamGame={true}
						gametype={snapshot.gametype}
					/>
				</div>
			{:else}
				<div class="border-b border-border">
					<div class="border-b border-border bg-card2 px-2 py-1 text-[11px] font-bold text-dim">
						Players
					</div>
					<PlayerTable
						players={snapshot.players}
						tickPlayers={tickPlayerMap()}
						teamFilter={null}
						isTeamGame={false}
						gametype={snapshot.gametype}
					/>
				</div>
			{/if}

			<!-- Collapsible event log -->
			<div class="border-t border-border">
				<button
					class="flex w-full items-center gap-2 bg-card2 px-2 py-1 text-[10px] uppercase tracking-wide text-dim hover:text-text"
					onclick={() => (showEventLog = !showEventLog)}
				>
					Events
					<span class="text-[9px]">({getEvents().filter((e) => e.instance === activeHost).length})</span>
					<span class="ml-auto">{showEventLog ? '▾' : '▸'}</span>
				</button>
				{#if showEventLog}
					<div class="h-64 border-t border-border">
						<EventLog
							events={getEvents()}
							snapshots={getSnapshots()}
							instanceFilter={activeHost}
						/>
					</div>
				{/if}
			</div>
		</div>
	{:else}
		<div class="flex flex-1 items-center justify-center text-xs text-dim">
			Waiting for WebSocket data...
		</div>
	{/if}
</div>
