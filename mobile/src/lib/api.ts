// Mobile API client — uses shared createApiClient with the mobile adapter
// Extracted from: web/src/lib/api.ts — adapted with ApiAdapter pattern

import { createApiClient } from "../../../packages/shared/src/api";
import { mobileAdapter } from "./api-adapter";
import { API_BASE } from "./config";

export * from "../../../packages/shared/src/types";
export * from "../../../packages/shared/src/push-admin";

export const api = createApiClient(API_BASE, mobileAdapter);

export const defaultImage = `${API_BASE}/static/default.png`;
