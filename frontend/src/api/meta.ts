import { apiClient } from "./apiUtils";
import type { APIResponse } from "./types";

export interface MetaResponse {
  version: string;
  http_server_timeout: string;
}

export const metaApi = {
  /**
   * Get server metadata information
   */
  getMeta: () => apiClient.get<MetaResponse>("/meta"),
};