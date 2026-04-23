<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getSnapshots, getTicks } from '$lib/stores/websocket.svelte';
	import { fetchStatus } from '$lib/stores/api';
	import SplitscreenGrid from '$lib/components/overlay/SplitscreenGrid.svelte';
	import type { TickPlayer } from '$lib/types';

	let requested = $derived(page.url.searchParams.get('instance') ?? '');
	let xboxToHost = $state<Record<string, string>>({});

	onMount(async () => {
		try {
			const status = await fetchStatus();
			const map: Record<string, string> = {};
			for (const [hostName, h] of Object.entries(status.hosts ?? {})) {
				if (h.xbox_name) map[h.xbox_name] = hostName;
			}
			xboxToHost = map;
		} catch {
			// Silent — overlay will just fall through to no-match.
		}
	});

	let instance = $derived.by(() => {
		if (!requested) return '';
		if (getSnapshots()[requested]) return requested;
		return xboxToHost[requested] ?? '';
	});

	let snapshot = $derived(instance ? getSnapshots()[instance] : undefined);
	let tick = $derived(instance ? getTicks()[instance] : undefined);
	let show = $derived(!!snapshot && snapshot.game_state === 'in_game');

	let tickByIndex = $derived.by(() => {
		const map: Record<number, TickPlayer> = {};
		for (const tp of tick?.players ?? []) map[tp.index] = tp;
		return map;
	});

	let namedPlayers = $derived(
		(snapshot?.players ?? []).filter((p) => p.name && p.name.trim() !== '')
	);

	let detectionUnavailable = $derived(
		namedPlayers.length > 0 && namedPlayers.every((p) => p.is_local == null)
	);

	let locals = $derived.by(() => {
		const ls = namedPlayers.filter((p) => p.is_local === true);
		return ls.sort((a, b) => (a.local_index ?? 0) - (b.local_index ?? 0));
	});
</script>

{#if show && snapshot}
	{#if detectionUnavailable}
		<div class="flex h-screen items-center justify-center">
			<div
				class="rounded border border-border bg-[#0c0c12]/80 px-4 py-2 font-mono text-xs text-dim shadow-lg backdrop-blur-sm"
			>
				Local player detection unavailable for this game
			</div>
		</div>
	{:else if locals.length > 0}
		<SplitscreenGrid
			players={locals}
			{tickByIndex}
			isTeamGame={snapshot.is_team_game}
		/>
	{/if}
{/if}
