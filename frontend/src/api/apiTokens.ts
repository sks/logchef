import { apiClient } from "./apiUtils";

export interface APIToken {
  id: number;
  user_id: number;
  name: string;
  prefix: string;
  last_used_at?: string;
  expires_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateAPITokenRequest {
  name: string;
  expires_at?: string;
}

export interface CreateAPITokenResponse {
  token: string;
  api_token: APIToken;
}

export const apiTokensApi = {
  listTokens: () => apiClient.get<APIToken[]>("/me/tokens"),
  createToken: (data: CreateAPITokenRequest) => 
    apiClient.post<CreateAPITokenResponse>("/me/tokens", data),
  deleteToken: (tokenId: number) => 
    apiClient.delete<{ message: string }>(`/me/tokens/${tokenId}`)
};