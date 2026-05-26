// Thin wrapper around the Kakao JavaScript SDK loaded from CDN in index.html.
// The SDK is initialised lazily on first use so the bundle stays unaffected
// when the key is absent (local dev without a JS key still works via the
// server-side redirect fallback).

const API_BASE = import.meta.env.VITE_API_URL || "";
const JS_KEY = import.meta.env.VITE_KAKAO_JS_KEY as string | undefined;

declare global {
  interface Window {
    Kakao?: KakaoSDK;
  }
}

interface KakaoSDK {
  isInitialized: () => boolean;
  init: (key: string) => void;
  Auth: {
    authorize: (opts: {
      redirectUri: string;
      state?: string;
      throughTalk?: boolean;
      scope?: string;
    }) => void;
  };
}

// kakaoSDKAvailable reports whether the SDK script has loaded and a JS key is
// configured. Callers fall back to the redirect flow when this returns false.
export function kakaoSDKAvailable(): boolean {
  return !!JS_KEY && typeof window !== "undefined" && !!window.Kakao;
}

// ensureInit lazily calls Kakao.init once. Safe to call repeatedly.
function ensureInit(): KakaoSDK | null {
  if (!kakaoSDKAvailable() || !JS_KEY) return null;
  const sdk = window.Kakao!;
  if (!sdk.isInitialized()) {
    try {
      sdk.init(JS_KEY);
    } catch {
      return null;
    }
  }
  return sdk;
}

// buildRedirectURI returns the absolute URI that Kakao should redirect to
// after auth. It MUST match a URI registered in the Kakao Developer Console.
// We piggy-back on the same path used by the server-side flow so only one URI
// needs to be whitelisted.
function buildRedirectURI(): string {
  const base = API_BASE || window.location.origin;
  return `${base}/api/v1/auth/kakao/callback`;
}

// fetchState pulls a one-shot CSRF state token from our backend. The same
// store backs the callback's Validate() check so the state survives the
// hop through Kakao without us needing to set any cookies.
async function fetchState(): Promise<string | null> {
  try {
    const res = await fetch(`${API_BASE}/api/v1/auth/kakao/state`, {
      method: "GET",
      credentials: "include",
    });
    if (!res.ok) return null;
    const body = (await res.json()) as { state?: string };
    return body.state ?? null;
  } catch {
    return null;
  }
}

// kakaoLogin triggers the SDK-based authorize flow. On mobile devices with the
// KakaoTalk app installed this opens the app via custom URL scheme for a true
// single-tap login. Desktop and missing-app environments fall back to Kakao's
// web login page — still one redirect, but identical UX to the server flow.
//
// Returns true when the flow was launched, false when the caller should fall
// back to the redirect endpoint.
export async function kakaoLogin(): Promise<boolean> {
  const sdk = ensureInit();
  if (!sdk) return false;

  const state = await fetchState();
  if (!state) return false;

  sdk.Auth.authorize({
    redirectUri: buildRedirectURI(),
    state,
    throughTalk: true,
  });
  return true;
}
