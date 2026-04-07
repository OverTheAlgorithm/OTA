// Mobile API adapter — stores JWT in SecureStore and attaches as Bearer token
// Platform counterpart to web's cookie-based credentials:"include" approach

import * as SecureStore from "expo-secure-store";
import type { ApiAdapter } from "../../../packages/shared/src/api";

const TOKEN_KEY = "wizletter_token";

export const mobileAdapter: ApiAdapter = {
  async getToken(): Promise<string | null> {
    return SecureStore.getItemAsync(TOKEN_KEY);
  },

  async setToken(token: string): Promise<void> {
    await SecureStore.setItemAsync(TOKEN_KEY, token);
  },

  async removeToken(): Promise<void> {
    await SecureStore.deleteItemAsync(TOKEN_KEY);
  },

  fetch(input: string, init?: RequestInit): Promise<Response> {
    return fetch(input, init);
  },
};
