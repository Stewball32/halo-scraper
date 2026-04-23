<script lang="ts">
	import type { SnapshotPlayer, TickPlayer } from '$lib/types';
	import PlayerCard from './PlayerCard.svelte';

	let {
		players,
		tickByIndex,
		isTeamGame
	}: {
		players: SnapshotPlayer[];
		tickByIndex: Record<number, TickPlayer>;
		isTeamGame: boolean;
	} = $props();

	let gridClass = $derived.by(() => {
		if (players.length <= 1) return 'flex';
		if (players.length === 2) return 'grid grid-rows-2 h-screen';
		return 'grid grid-cols-2 grid-rows-2 h-screen';
	});
</script>

<div class="{gridClass} w-full">
	{#each players as p (p.index)}
		<div class="flex items-start justify-start p-2">
			<PlayerCard player={p} tick={tickByIndex[p.index]} {isTeamGame} />
		</div>
	{/each}
</div>
