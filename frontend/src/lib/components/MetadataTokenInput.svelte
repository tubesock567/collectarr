<script>
	let {
		label,
		values = [],
		suggestions = [],
		placeholder = '',
		disabled = false,
		helpText = '',
		excludeValues = [],
		emptyText = 'Start typing to add items.',
		onChange = () => {}
	} = $props();

	let inputValue = $state('');
	let activeSuggestionIndex = $state(-1);
	let inputFocused = $state(false);

	const normalizedInput = $derived(inputValue.trim().toLowerCase());
	const normalizedValues = $derived(values.map((value) => value.toLowerCase()));
	const normalizedExcludedValues = $derived(
		(excludeValues || []).map((value) => value.toLowerCase())
	);
	const filteredSuggestions = $derived.by(() => {
		const selectedSet = new Set(normalizedValues);
		const excludedSet = new Set(normalizedExcludedValues);
		return (suggestions || [])
			.filter((suggestion) => {
				const normalizedSuggestion = suggestion.toLowerCase();
				if (selectedSet.has(normalizedSuggestion) || excludedSet.has(normalizedSuggestion)) {
					return false;
				}
				if (!normalizedInput) {
					return true;
				}
				return normalizedSuggestion.includes(normalizedInput);
			})
			.slice(0, 8);
	});
	const showSuggestions = $derived(inputFocused && filteredSuggestions.length > 0);

	function emitChange(nextValues) {
		onChange(nextValues);
	}

	function normalizeCandidate(rawValue) {
		const trimmed = rawValue.trim();
		if (!trimmed) {
			return '';
		}

		const existingSuggestion = (suggestions || []).find(
			(suggestion) => suggestion.toLowerCase() === trimmed.toLowerCase()
		);
		const existingValue = values.find((value) => value.toLowerCase() === trimmed.toLowerCase());
		return existingSuggestion || existingValue || trimmed;
	}

	function addValue(rawValue) {
		const nextValue = normalizeCandidate(rawValue);
		if (!nextValue) {
			return;
		}
		if (values.some((value) => value.toLowerCase() === nextValue.toLowerCase())) {
			inputValue = '';
			activeSuggestionIndex = -1;
			return;
		}
		if ((excludeValues || []).some((value) => value.toLowerCase() === nextValue.toLowerCase())) {
			inputValue = '';
			activeSuggestionIndex = -1;
			return;
		}

		emitChange([...values, nextValue]);
		inputValue = '';
		activeSuggestionIndex = -1;
	}

	function removeValue(valueToRemove) {
		emitChange(values.filter((value) => value !== valueToRemove));
		activeSuggestionIndex = -1;
	}

	function commitInput() {
		if (activeSuggestionIndex >= 0 && filteredSuggestions[activeSuggestionIndex]) {
			addValue(filteredSuggestions[activeSuggestionIndex]);
			return;
		}
		addValue(inputValue);
	}

	function handleInput(event) {
		inputValue = event.currentTarget.value;
		activeSuggestionIndex = filteredSuggestions.length > 0 ? 0 : -1;
	}

	function handleKeydown(event) {
		if (disabled) {
			return;
		}

		if (event.key === 'Enter' || event.key === 'Tab' || event.key === ',') {
			if (inputValue.trim() || activeSuggestionIndex >= 0) {
				event.preventDefault();
				commitInput();
			}
			return;
		}

		if (event.key === 'Backspace' && !inputValue && values.length > 0) {
			event.preventDefault();
			removeValue(values[values.length - 1]);
			return;
		}

		if (event.key === 'ArrowDown' && filteredSuggestions.length > 0) {
			event.preventDefault();
			activeSuggestionIndex =
				activeSuggestionIndex < filteredSuggestions.length - 1 ? activeSuggestionIndex + 1 : 0;
			return;
		}

		if (event.key === 'ArrowUp' && filteredSuggestions.length > 0) {
			event.preventDefault();
			activeSuggestionIndex =
				activeSuggestionIndex > 0 ? activeSuggestionIndex - 1 : filteredSuggestions.length - 1;
			return;
		}

		if (event.key === 'Escape') {
			activeSuggestionIndex = -1;
		}
	}
</script>

<div>
	<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-400">{label}</p>
	<div class="relative mt-3">
		<div
			class="min-h-[3.25rem] border border-neutral-700 bg-black px-3 py-3 text-sm text-white transition-colors focus-within:border-neutral-400"
		>
			<div class="flex flex-wrap items-center gap-2">
				{#each values as value (value)}
					<span
						class="inline-flex items-center gap-2 border border-neutral-700 bg-neutral-900 px-2 py-1 text-xs text-white"
					>
						<span>{value}</span>
						<button
							type="button"
							class="text-neutral-400 transition-colors hover:text-white disabled:cursor-not-allowed"
							onclick={() => removeValue(value)}
							{disabled}
							aria-label={`Remove ${value}`}
						>
							×
						</button>
					</span>
				{/each}

				<input
					type="text"
					value={inputValue}
					oninput={handleInput}
					onkeydown={handleKeydown}
					onfocus={() => (inputFocused = true)}
					onblur={() => {
						inputFocused = false;
						activeSuggestionIndex = -1;
					}}
					{placeholder}
					{disabled}
					class="min-w-[10rem] flex-1 bg-transparent text-sm text-white outline-none placeholder:text-neutral-600 disabled:cursor-not-allowed"
				/>
			</div>
		</div>

		{#if showSuggestions}
			<div
				class="absolute left-0 right-0 top-full z-20 mt-2 border border-neutral-700 bg-black shadow-2xl"
			>
				{#each filteredSuggestions as suggestion, index (suggestion)}
					<button
						type="button"
						class="block w-full px-3 py-2 text-left text-sm transition-colors {index ===
						activeSuggestionIndex
							? 'bg-neutral-900 text-white'
							: 'text-neutral-300 hover:bg-neutral-900 hover:text-white'}"
						onmousedown={(event) => event.preventDefault()}
						onclick={() => addValue(suggestion)}
					>
						{suggestion}
					</button>
				{/each}
			</div>
		{/if}
	</div>

	<p class="mt-2 text-xs text-neutral-500">{helpText || emptyText}</p>
</div>
