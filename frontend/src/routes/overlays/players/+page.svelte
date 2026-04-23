<script lang="ts">
	import { page } from '$app/state';
	import { getSnapshots, getTicks } from '$lib/stores/websocket.svelte';
	import PlayerCard from '$lib/components/overlay/PlayerCard.svelte';
	import type { TickPlayer } from '$lib/types';

	let instance = $derived(page.url.searchParams.get('instance') ?? '');
	let snapshot = $derived(instance ? getSnapshots()[instance] : undefined);
	let tick = $derived(instance ? getTicks()[instance] : undefined);
	let show = $derived(!!snapshot && snapshot.game_state === 'in_game');

	let tickByIndex = $derived.by(() => {
		const map: Record<number, TickPlayer> = {};
		for (const tp of tick?.players ?? []) map[tp.index] = tp;
		return map;
	});

	let visible = $derived((snapshot?.players ?? []).filter((p) => p.name && p.name.trim() !== ''));

	let sorted = $derived.by(() => {
		if (!snapshot) return visible;
		if (snapshot.is_team_game) {
			return [...visible].sort((a, b) => a.team - b.team || a.index - b.index);
		}
		return [...visible].sort((a, b) => b.kills - a.kills || a.index - b.index);
	});
</script>

{#if show && snapshot}
	<div class="flex flex-col gap-1.5 p-2">
		{#each sorted as p (p.index)}
			<PlayerCard player={p} tick={tickByIndex[p.index]} isTeamGame={snapshot.is_team_game} />
		{/each}
	</div>
{/if}
