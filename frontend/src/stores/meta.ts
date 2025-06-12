import { defineStore } from "pinia";
import { computed } from "vue";
import { metaApi, type MetaResponse } from "@/api/meta";
import { useBaseStore } from "./base";
import type { APIErrorResponse } from "@/api/types";

interface MetaState {
  version: string | null;
  httpServerTimeout: string | null;
  isInitialized: boolean;
}

export const useMetaStore = defineStore("meta", () => {
  const state = useBaseStore<MetaState>({
    version: null,
    httpServerTimeout: null,
    isInitialized: false,
  });

  // Computed properties
  const version = computed(() => state.data.value.version);
  const httpServerTimeout = computed(() => state.data.value.httpServerTimeout);
  const isInitialized = computed(() => state.data.value.isInitialized);
  const error = computed(() => state.error.value);

  // Load server metadata
  async function loadMeta() {
    if (isInitialized.value) {
      return { success: true, message: "Meta already loaded" };
    }

    return await state.withLoading('loadMeta', async () => {
      try {
        const result = await state.callApi({
          apiCall: () => metaApi.getMeta(),
          showToast: false, // Don't show toast for meta loading
          operationKey: 'loadMeta',
          onSuccess: (response: MetaResponse | null) => {
            if (response) {
              state.data.value.version = response.version;
              state.data.value.httpServerTimeout = response.http_server_timeout;
              state.data.value.isInitialized = true;
              console.log("Meta loaded successfully:", {
                version: response.version,
                httpServerTimeout: response.http_server_timeout,
              });
            }
          },
          onError: (error) => {
            console.error("Failed to load server metadata:", error);
          },
        });

        return result;
      } catch (error) {
        return state.handleError(error as Error | APIErrorResponse, 'loadMeta');
      }
    });
  }

  // Clear meta state
  function clearState() {
    state.data.value.version = null;
    state.data.value.httpServerTimeout = null;
    state.data.value.isInitialized = false;
  }

  return {
    version,
    httpServerTimeout,
    isInitialized,
    error,
    loadMeta,
    clearState,
    // Loading state helpers
    isLoading: computed(() => state.isLoading.value),
    isLoadingOperation: state.isLoadingOperation,
  };
});