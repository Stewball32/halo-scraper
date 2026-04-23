<script lang="ts">
	import type { SnapshotPlayer, TickPlayer } from '$lib/types';
	import { hpBarColor, shBarColor } from '$lib/utils/bars';

	let {
		player,
		tick,
		isTeamGame
	}: { player: SnapshotPlayer; tick: TickPlayer | undefined; isTeamGame: boolean } = $props();

	let alive = $derived(tick?.alive ?? false);
	let hp = $derived(tick?.health ?? 0);
	let sh = $derived(tick?.shields ?? 0);
	let os = $derived(tick?.has_overshield ?? false);

	function nameColor(team: number): string {
		if (!isTeamGame) return 'text-text';
		return team === 0 ? 'text-hred' : 'text-hblue';
	}
</script>

<div
	class="w-56 rounded border border-border bg-[#0c0c12]/80 px-2 py-1.5 font-mono shadow-lg backdrop-blur-sm transition-opacity {tick &&
	!alive
		? 'opacity-40'
		: ''}"
>
	<div class="mb-1 flex items-baseline justify-between gap-2">
		<span class="truncate text-sm font-bold {nameColor(player.team)}">{player.name}</span>
		<span class="whitespace-nowrap text-[10px] tabular-nums">
			<span class="text-hgreen">{player.kills}</span><span class="text-dim">/</span><span
				class="text-hred">{player.deaths}</span
			><span class="text-dim">/</span><span class="text-dim">{player.assists}</span>
		</span>
	</div>

	{#if tick}
		<div class="flex items-center gap-1">
			<span class="w-4 text-[8px] uppercase tracking-wider text-dim">HP</span>
			<div class="h-1.5 flex-1 overflow-hidden rounded-sm bg-[#252535]">
				<div
					class="h-full rounded-sm transition-[width] duration-100 {hpBarColor(hp)}"
					style="width: {Math.round(hp * 100)}%"
				></div>
			</div>
		</div>
		<div class="mt-0.5 flex items-center gap-1">
			<span class="w-4 text-[8px] uppercase tracking-wider text-dim">SH</span>
			<div class="h-1.5 flex-1 overflow-hidden rounded-sm bg-[#252535]">
				<div
					class="h-full rounded-sm transition-[width] duration-100 {shBarColor(sh, os)}"
					style="width: {Math.min(Math.round(sh * 100), 100)}%"
				></div>
			</div>
		</div>
	{/if}

	{#if player.kill_streak >= 2 || player.multikill >= 2}
		<div class="mt-1 flex gap-2 text-[9px] uppercase tracking-wider">
			{#if player.kill_streak >= 2}
				<span class="text-hgold">streak ×{player.kill_streak}</span>
			{/if}
			{#if player.multikill >= 2}
				<span class="text-hgold">×{player.multikill} multi</span>
			{/if}
		</div>
	{/if}
</div>
