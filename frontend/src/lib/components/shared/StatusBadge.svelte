<script lang="ts">
	let { state }: { state: string } = $props();

	const styles: Record<string, string> = {
		online: 'bg-[#1a3320] text-hgreen',
		connecting: 'bg-[#2a2510] text-hyellow',
		offline: 'bg-[#2a1010] text-hred',
		running: 'bg-[#1a3320] text-hgreen',
		created: 'bg-[#2a2510] text-hyellow',
		exited: 'bg-[#2a1010] text-hred',
		stopped: 'bg-[#2a1010] text-hred',
		menu: 'bg-[#252535] text-dim',
		pregame: 'bg-[#2e2a10] text-hyellow',
		in_game: 'bg-[#102e18] text-hgreen',
		postgame: 'bg-[#2e1a10] text-horange'
	};
</script>

<span
	class="inline-block rounded px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-wide {styles[
		state
	] ?? 'bg-card2 text-dim'}"
>
	{state}
</span>
