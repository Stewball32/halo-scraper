<script lang="ts">
	let {
		open = false,
		title = 'Confirm',
		message = 'Are you sure?',
		confirmLabel = 'Confirm',
		onconfirm,
		oncancel
	}: {
		open?: boolean;
		title?: string;
		message?: string;
		confirmLabel?: string;
		onconfirm: () => void;
		oncancel: () => void;
	} = $props();
</script>

{#if open}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_interactive_supports_focus a11y_no_noninteractive_element_interactions -->
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60" role="dialog" aria-modal="true" tabindex="-1" onclick={oncancel} onkeydown={(e) => e.key === 'Escape' && oncancel()}>
		<div
			class="mx-4 w-full max-w-sm rounded border border-border bg-card p-5"
			role="document"
			onclick={(e) => e.stopPropagation()}
		>
			<h3 class="mb-2 text-sm font-bold text-text">{title}</h3>
			<p class="mb-4 text-xs text-dim">{message}</p>
			<div class="flex justify-end gap-2">
				<button
					class="rounded border border-border bg-card2 px-3 py-1.5 text-xs text-text hover:bg-border"
					onclick={oncancel}
				>
					Cancel
				</button>
				<button
					class="rounded border border-hred/30 bg-hred/10 px-3 py-1.5 text-xs text-hred hover:bg-hred/20"
					onclick={onconfirm}
				>
					{confirmLabel}
				</button>
			</div>
		</div>
	</div>
{/if}
