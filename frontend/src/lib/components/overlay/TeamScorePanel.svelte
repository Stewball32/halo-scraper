<script lang="ts">
	import type { TeamScore } from '$lib/types';

	let { teamScores, scoreLimit }: { teamScores: TeamScore[]; scoreLimit: number } = $props();

	let red = $derived(teamScores.find((t) => t.team === 0)?.score ?? 0);
	let blue = $derived(teamScores.find((t) => t.team === 1)?.score ?? 0);

	function pad(n: number): string {
		return n.toString().padStart(2, '0');
	}
</script>

<div
	class="inline-flex items-center gap-4 rounded border border-border bg-[#0c0c12]/80 px-4 py-2 font-mono shadow-lg backdrop-blur-sm"
>
	<div class="flex flex-col items-center">
		<span class="text-[10px] uppercase tracking-widest text-hred">Red</span>
		<span class="text-4xl font-bold leading-none text-hred tabular-nums">{pad(red)}</span>
	</div>

	<span class="text-2xl font-bold leading-none text-dim">—</span>

	<div class="flex flex-col items-center">
		<span class="text-[10px] uppercase tracking-widest text-hblue">Blue</span>
		<span class="text-4xl font-bold leading-none text-hblue tabular-nums">{pad(blue)}</span>
	</div>

	{#if scoreLimit > 0}
		<div class="ml-2 border-l border-border pl-3">
			<span class="block text-[9px] uppercase tracking-wider text-dim">Limit</span>
			<span class="text-sm text-text tabular-nums">{scoreLimit}</span>
		</div>
	{/if}
</div>
