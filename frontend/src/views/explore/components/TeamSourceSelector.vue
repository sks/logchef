<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { formatSourceName } from '@/utils/format'
import { useContextStore } from '@/stores/context'
import { useTeamsStore } from '@/stores/teams'
import { useSourcesStore } from '@/stores/sources'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const router = useRouter()

// Use the centralized stores
const contextStore = useContextStore()
const teamsStore = useTeamsStore()
const sourcesStore = useSourcesStore()

// Computed properties for the clean approach
const currentTeamId = computed(() => contextStore.teamId)
const currentSourceId = computed(() => contextStore.sourceId)
const availableTeams = computed(() => teamsStore.teams || [])
const availableSources = computed(() => sourcesStore.teamSources || [])
const selectedTeamName = computed(() => teamsStore.currentTeam?.name || 'Select team')
const selectedSourceName = computed(() => {
  if (!currentSourceId.value) return 'Select source'
  const source = availableSources.value.find(s => s.id === currentSourceId.value)
  return source ? formatSourceName(source) : 'Select source'
})

// Computed properties for loading states (direct boolean values)
const isTeamDisabled = ref(false)
const isSourceDisabled = ref(false)

// Watch the store loading states and update our local refs
watch(() => sourcesStore.isLoadingTeamSources, (value) => {
  isTeamDisabled.value = Boolean(value)
}, { immediate: true })

watch(() => [sourcesStore.isLoadingSourceDetails, currentTeamId.value, availableSources.value.length], 
([loadingDetails, teamId, sourcesLength]) => {
  isSourceDisabled.value = Boolean(loadingDetails) || !teamId || sourcesLength === 0
}, { immediate: true })

// Simple handlers using router
function handleTeamChange(teamIdStr: string) {
  const teamId = parseInt(teamIdStr)
  if (isNaN(teamId)) return
  
  router.replace({
    query: {
      ...router.currentRoute.value.query,
      team: String(teamId),
      source: undefined // Clear source when team changes
    }
  })
}

function handleSourceChange(sourceIdStr: string) {
  const sourceId = parseInt(sourceIdStr)
  if (isNaN(sourceId)) return
  
  router.replace({
    query: {
      ...router.currentRoute.value.query,
      source: String(sourceId)
    }
  })
}

// Expose the source connection status to parent components
const isSourceConnected = computed(() => {
  if (!currentSourceId.value || !availableSources.value.length) return false
  
  const currentSource = availableSources.value.find(s => s.id === currentSourceId.value)
  return currentSource?.is_connected ?? false
})

defineExpose({
  isSourceConnected
})
</script>

<template>
  <div class="flex items-center space-x-3">
    <!-- Team Selector -->
    <Select :model-value="currentTeamId?.toString() ?? ''" @update:model-value="handleTeamChange"
      :disabled="isTeamDisabled">
      <SelectTrigger class="h-8 text-sm w-48">
        <SelectValue placeholder="Select team">{{ selectedTeamName }}</SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          <SelectLabel>Teams</SelectLabel>
          <SelectItem v-for="team in availableTeams" :key="team.id" :value="team.id.toString()">
            {{ team.name }}
          </SelectItem>
        </SelectGroup>
      </SelectContent>
    </Select>

    <!-- Source Selector -->
    <Select :model-value="currentSourceId?.toString() ?? ''" @update:model-value="handleSourceChange"
      :disabled="isSourceDisabled">
      <SelectTrigger class="h-8 text-sm w-64">
        <SelectValue placeholder="Select source">{{ selectedSourceName }}</SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          <SelectLabel>Log Sources</SelectLabel>
          <SelectItem v-if="!currentTeamId" value="no-team" disabled>Select a team first</SelectItem>
          <SelectItem v-else-if="availableSources.length === 0" value="no-sources" disabled>No sources available
          </SelectItem>
          <template v-for="source in availableSources" :key="source.id">
            <SelectItem :value="source.id.toString()">
              <div class="flex items-center gap-2">
                <span>{{ formatSourceName(source) }}</span>
                <span v-if="!source.is_connected"
                  class="inline-flex items-center text-xs bg-destructive/15 text-destructive px-1.5 py-0.5 rounded">
                  Disconnected
                </span>
              </div>
            </SelectItem>
          </template>
        </SelectGroup>
      </SelectContent>
    </Select>


  </div>
</template>