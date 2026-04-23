<script lang="ts">
	import '../../app.css';
	import { onMount } from 'svelte';
	import { connect, disconnect } from '$lib/stores/websocket.svelte';

	let { children } = $props();

	onMount(() => {
		connect();
		return () => disconnect();
	});
</script>

<svelte:head>
	<style>
		html,
		body {
			background: transparent !important;
		}
	</style>
</svelte:head>

<div class="h-full w-full">
	{@render children()}
</div>
