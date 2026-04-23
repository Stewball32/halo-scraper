<script lang="ts">
	import { page } from '$app/state';
	import { getSnapshots } from '$lib/stores/websocket.svelte';
	import TeamScorePanel from '$lib/components/overlay/TeamScorePanel.svelte';

	let instance = $derived(page.url.searchParams.get('instance') ?? '');
	let snapshot = $derived(instance ? getSnapshots()[instance] : undefined);
	let show = $derived(!!snapshot && snapshot.game_state === 'in_game' && snapshot.is_team_game);
</script>

{#if show && snapshot}
	<div class="p-2">
		<TeamScorePanel teamScores={snapshot.team_scores} scoreLimit={snapshot.score_limit} />
	</div>
{/if}
