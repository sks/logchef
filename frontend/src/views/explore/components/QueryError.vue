<script setup lang="ts">
import { computed } from 'vue'
import { useExploreStore } from '@/stores/explore'

interface Props {
  queryError?: string
}

const props = defineProps<Props>()
const exploreStore = useExploreStore()

// Combined error display from props and store
const displayError = computed(() => props.queryError || exploreStore.error?.message)
</script>

<template>
  <div v-if="displayError" class="mt-2 text-sm text-destructive bg-destructive/10 p-2 rounded">
    <div class="font-medium">Query Error:</div>
    <div>{{ displayError }}</div>
    <div v-if="displayError.includes('Missing boolean operator')"
      class="mt-1.5 pt-1.5 border-t border-destructive/20 text-xs">
      <div class="font-medium">Hint:</div>
      <div>Use <code class="bg-muted px-1 rounded">and</code> or <code
          class="bg-muted px-1 rounded">or</code>
        between conditions.</div>
      <div class="mt-1">Example: <code class="bg-muted px-1 rounded">level="error" and
    service_name="api-gateway"</code></div>
    </div>
  </div>
</template>