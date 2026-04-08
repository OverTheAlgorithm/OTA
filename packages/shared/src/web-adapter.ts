// Web API adapter — uses cookie-based credentials:"include" for auth
// Platform counterpart to mobile's SecureStore-based Bearer token approach
//
// Usage (in web/src/lib/api.ts):
//   import { createApiClient } from "@wizletter/shared";
//   import { webAdapter } from "@wizletter/shared/web-adapter";
//   export const api = createApiClient(API_BASE, webAdapter);

import type { ApiAdapter } from "./api";

export const webAdapter: ApiAdapter = {
  async getToken(): Promise<string | null> {
    // Web uses HTTP-only cookies — no explicit token management
    return null;
  },

  async setToken(): Promise<void> {
    // no-op: web relies on Set-Cookie from server
  },

  async removeToken(): Promise<void> {
    // no-op: cookie cleared by server on logout
  },

  fetch(input: string, init?: RequestInit): Promise<Response> {
    return fetch(input, { credentials: "include", ...init });
  },
};
