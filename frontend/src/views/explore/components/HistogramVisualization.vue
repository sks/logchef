<script setup lang="ts">
import { computed } from 'vue'
import { useExploreStore } from '@/stores/explore'
import { useSourcesStore } from '@/stores/sources'
import LogHistogram from '@/components/visualizations/LogHistogram.vue'

interface TimeRangeUpdateEvent {
  start: Date;
  end: Date;
}

const emit = defineEmits<{
  (e: 'zoom-time-range', range: TimeRangeUpdateEvent): void
  (e: 'update:timeRange', range: TimeRangeUpdateEvent): void
}>()

const exploreStore = useExploreStore()
const sourcesStore = useSourcesStore()

// Reactive computed properties
const isExecutingQuery = computed(() => exploreStore.isLoadingOperation('executeQuery'))
const timeRange = computed(() => exploreStore.timeRange)
const groupByField = computed(() => exploreStore.groupByField === '__none__' ? undefined : exploreStore.groupByField)
const sourceId = computed(() => exploreStore.sourceId)

// Event handlers for histogram interactions
const handleZoomTimeRange = (range: TimeRangeUpdateEvent) => {
  emit('zoom-time-range', range)
}

const handleTimeRangeUpdate = (range: TimeRangeUpdateEvent) => {
  emit('update:timeRange', range)
}
</script>

<template>
  <div class="histogram-container">
    <LogHistogram
      :key="`histogram-${sourceId}`"
      :time-range="timeRange"
      :is-loading="isExecutingQuery"
      :group-by="groupByField"
      @zoom-time-range="handleZoomTimeRange"
      @update:timeRange="handleTimeRangeUpdate"
    />
  </div>
</template>