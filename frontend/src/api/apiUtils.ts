import { api } from "./config";
import type { APIResponse } from "./types";

/**
 * Generic API request function to reduce repetitive code
 */
export async function apiRequest<T>(
  method: "get" | "post" | "put" | "delete",
  url: string,
  data?: any,
  options?: { timeout?: number }
): Promise<APIResponse<T>> {
  const config = options?.timeout ? { timeout: options.timeout * 1000 } : {}; // Convert seconds to milliseconds
  
  let response;
  if (method === "get" || method === "delete") {
    response = await api[method]<APIResponse<T>>(url, config);
  } else {
    response = await api[method]<APIResponse<T>>(url, data, config);
  }
  return response.data;
}

/**
 * Shorthand methods for common API operations
 */
export const apiClient = {
  get: <T>(url: string, options?: { timeout?: number }) => apiRequest<T>("get", url, undefined, options),
  post: <T>(url: string, data?: any, options?: { timeout?: number }) => apiRequest<T>("post", url, data, options),
  put: <T>(url: string, data?: any, options?: { timeout?: number }) => apiRequest<T>("put", url, data, options),
  delete: <T>(url: string, options?: { timeout?: number }) => apiRequest<T>("delete", url, undefined, options)
};
