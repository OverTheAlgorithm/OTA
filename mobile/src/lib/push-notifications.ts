// Push notification registration for WizLetter mobile.
// Requests permission, obtains Expo Push Token, and syncs with server.
//
// Flow:
//   App start → getAndRegisterAnonymous() → token stored anonymously on server
//   Login     → registerWithAuth()        → token linked to user_id
//   Logout    → unlinkToken()             → user_id set to NULL, token preserved

import { Platform } from "react-native";
import Constants from "expo-constants";
import * as Notifications from "expo-notifications";
import { api } from "./api";

// Configure how notifications are displayed when the app is in the foreground.
Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowAlert: true,
    shouldPlaySound: true,
    shouldSetBadge: false,
    shouldShowBanner: true,
    shouldShowList: true,
  }),
});

// Cached token to avoid redundant getExpoPushTokenAsync calls within the same session.
let cachedToken: string | null = null;

/**
 * Request push permission and obtain the Expo Push Token.
 * Returns the token string, or null if not available (emulator, permission denied).
 */
async function getExpoPushToken(): Promise<string | null> {
  if (cachedToken) return cachedToken;

  const { status: existing } = await Notifications.getPermissionsAsync();
  let finalStatus = existing;

  if (existing !== "granted") {
    const { status } = await Notifications.requestPermissionsAsync();
    finalStatus = status;
  }

  if (finalStatus !== "granted") {
    console.warn("[Push] Permission not granted");
    return null;
  }

  if (Platform.OS === "android") {
    await Notifications.setNotificationChannelAsync("default", {
      name: "default",
      importance: Notifications.AndroidImportance.MAX,
    });
  }

  const projectId = Constants.expoConfig?.extra?.eas?.projectId as string | undefined;
  const maxRetries = 3;
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      const tokenData = await Notifications.getExpoPushTokenAsync({ projectId });
      cachedToken = tokenData.data;
      console.log("[Push] Expo token obtained:", cachedToken);
      return cachedToken;
    } catch (err) {
      const isTransient = String(err).includes("503") || String(err).includes("isTransient");
      if (isTransient && attempt < maxRetries) {
        const delay = attempt * 3000; // 3s, 6s
        console.warn(`[Push] Transient error (attempt ${attempt}/${maxRetries}), retrying in ${delay / 1000}s...`);
        await new Promise((r) => setTimeout(r, delay));
        continue;
      }
      console.error("[Push] Failed to get Expo Push Token:", err);
      return null;
    }
  }
  return null;
}

/**
 * Register push token anonymously (no auth).
 * Called on every app start to ensure the server has the latest device token.
 */
export async function registerAnonymous(): Promise<void> {
  const token = await getExpoPushToken();
  if (!token) return;

  try {
    await api.registerPushTokenPublic(token, Platform.OS);
    console.log("[Push] Anonymous token registered");
  } catch (err) {
    console.error("[Push] Failed to register anonymous token:", err);
  }
}

/**
 * Register push token with authentication (links user_id on server).
 * Called after successful login / fetchMe.
 */
export async function registerWithAuth(): Promise<void> {
  const token = await getExpoPushToken();
  if (!token) return;

  try {
    await api.registerPushToken(token, Platform.OS);
    console.log("[Push] Token linked to user");
  } catch (err) {
    console.error("[Push] Failed to link token to user:", err);
  }
}

/**
 * Unlink the current device's push token from the user on the server.
 * The token row is preserved for anonymous push delivery.
 * Called on logout.
 */
export async function unlinkToken(): Promise<void> {
  const token = cachedToken ?? (await getExpoPushToken());
  if (!token) return;

  try {
    await api.unlinkPushToken(token);
    console.log("[Push] Token unlinked from user");
  } catch (err) {
    // Best-effort — token may already be unlinked
    console.warn("[Push] Failed to unlink token:", err);
  }
}
