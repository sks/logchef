import { defineStore } from 'pinia'

/**
 * Clean, minimal context store for team and source selection.
 * 
 * RULES:
 * 1. Team selection clears source (user must explicitly select source)
 * 2. No automatic fallbacks or magic
 * 3. Route is source of truth, this store is derived from route
 * 4. No complex coordination - just simple state
 */

interface ContextState {
  teamId: number | null
  sourceId: number | null
}

export const useContextStore = defineStore('context', {
  state: (): ContextState => ({
    teamId: null,
    sourceId: null,
  }),

  getters: {
    hasTeam: (state) => state.teamId !== null && state.teamId > 0,
    hasSource: (state) => state.sourceId !== null && state.sourceId > 0,
    hasValidContext: (state) => 
      state.teamId !== null && state.teamId > 0 && 
      state.sourceId !== null && state.sourceId > 0,
  },

  actions: {
    /**
     * Select a team. This clears the source - user must explicitly select a new source.
     */
    selectTeam(teamId: number) {
      console.log(`ContextStore: Selecting team ${teamId}, clearing source`)
      this.teamId = teamId
      this.sourceId = null // Hard reset - no automatic source selection
    },

    /**
     * Select a source. Team must already be selected.
     */
    selectSource(sourceId: number) {
      if (!this.hasTeam) {
        console.warn(`ContextStore: Cannot select source ${sourceId} without team`)
        return
      }
      console.log(`ContextStore: Selecting source ${sourceId} for team ${this.teamId}`)
      this.sourceId = sourceId
    },

    /**
     * Clear all context (useful for logout, etc.)
     */
    clear() {
      console.log('ContextStore: Clearing all context')
      this.teamId = null
      this.sourceId = null
    },

    /**
     * Set context from route params (used by router guard)
     */
    setFromRoute(teamId: number | null, sourceId: number | null) {
      console.log(`ContextStore: Setting from route - team: ${teamId}, source: ${sourceId}`)
      this.teamId = teamId
      this.sourceId = sourceId
    },
  },
})
