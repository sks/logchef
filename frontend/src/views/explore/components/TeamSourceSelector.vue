<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { Button } from '@/components/ui/button'
import { formatSourceName } from '@/utils/format'
import { useSourceTeamManagement } from '@/composables/useSourceTeamManagement'
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

const {
  isProcessingTeamChange,
  isProcessingSourceChange,
  currentTeamId,
  currentSourceId,
  availableTeams,
  availableSources,
  selectedTeamName,
  selectedSourceName,
  handleTeamChange,
  handleSourceChange
} = useSourceTeamManagement()

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
      :disabled="isProcessingTeamChange">
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
      :disabled="isProcessingSourceChange || !currentTeamId || availableSources.length === 0">
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

    <!-- Add Source Button (when needed) -->
    <Button v-if="currentTeamId && (!currentSourceId || availableSources.length === 0)" 
      size="sm" class="h-8" 
      @click="router.push({ name: 'NewSource' })">
      Add Source
    </Button>
  </div>
</template>