<script>
	import { toasts, removeToast } from '$lib/toast';

	function getIcon(type) {
		switch (type) {
			case 'success':
				return '✓';
			case 'error':
				return '✕';
			case 'warning':
				return '!';
			default:
				return 'i';
		}
	}

	function getColors(type) {
		switch (type) {
			case 'success':
				return 'border-emerald-700 bg-emerald-50 text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300';
			case 'error':
				return 'border-red-700 bg-red-50 text-red-900 dark:border-red-800 dark:bg-red-950/50 dark:text-red-300';
			case 'warning':
				return 'border-amber-700 bg-amber-50 text-amber-900 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-300';
			default:
				return 'border-blue-700 bg-blue-50 text-blue-900 dark:border-blue-800 dark:bg-blue-950/50 dark:text-blue-300';
		}
	}
</script>

<div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
	{#each $toasts as toast (toast.id)}
		<div
			class="flex items-center gap-3 border px-4 py-3 shadow-lg {getColors(toast.type)}"
		>
			<span class="flex h-5 w-5 shrink-0 items-center justify-center  border text-xs font-bold">
				{getIcon(toast.type)}
			</span>
			<span class="text-sm">{toast.message}</span>
			<button
				onclick={() => removeToast(toast.id)}
				class="ml-2 text-lg leading-none opacity-60 hover:opacity-100"
				aria-label="Dismiss"
			>
				×
			</button>
		</div>
	{/each}
</div>
