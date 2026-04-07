# WizLetter Mobile App

React Native (Expo managed workflow) mobile client for WizLetter. This app is a native port of the existing web frontend (`web/`).

## Core Principle: Web as Source of Truth

Every feature, flow, and business rule in this app must match the web implementation exactly. The web app is the reference — never invent new behavior.

**Workflow for any task:**
1. Read the corresponding web code first (`web/src/`)
2. Understand the full logic (API calls, state transitions, edge cases, error handling)
3. Implement the same logic in mobile, adapting only platform-specific concerns (UI components, navigation, storage)

**Shared code extraction:**
- When implementing a feature, if logic is identical to web (API calls, types, business rules, constants, validation, formatting), extract it into `packages/shared/` as a clean, well-designed common module
- Mark every shared module with a comment block at the top: which web file(s) it originates from and what it replaces (e.g. `// Extracted from: web/src/lib/api.ts — apiFetch, token refresh logic`)
- **Do NOT modify or replace any web code.** The shared module is mobile-only for now. Web continues using its original code untouched
- Focus on code quality — think carefully about the abstraction boundary. A sloppy extraction that half-fits both platforms is worse than duplication. Clean interfaces now make web adoption easy later
- The goal is speed to a working mobile app first, then consolidate web onto shared modules later with confidence

## Tech Stack

- **Framework**: React Native + Expo (managed workflow)
- **Language**: TypeScript
- **Navigation**: React Navigation (maps to React Router in web)
- **Storage**: expo-secure-store (auth tokens), @react-native-async-storage (general)
- **Styling**: NativeWind (Tailwind for RN) or StyleSheet
- **Auth**: Kakao OAuth via @react-native-kakao/core, JWT stored in SecureStore
- **Analytics**: @react-native-firebase/analytics (replaces GTM/dataLayer)
- **Bot protection**: Device attestation or reCAPTCHA Mobile (replaces Cloudflare Turnstile)
- **Ads**: react-native-google-mobile-ads (replaces web AdSense)

## Monorepo Structure (Target)

```
wizletter/
├── packages/
│   └── shared/              # Pure TS — no platform dependencies
│       ├── api/             # API client factory, endpoint functions, types
│       ├── domain/          # Coin calc, level logic, subscription filtering
│       ├── constants.ts
│       └── utils.ts         # Date/number formatting, validation
├── apps/
│   ├── web/                 # Existing React (Vite) — currently at web/
│   └── mobile/              # This app — currently at mobile/
```

Shared package uses adapter pattern for platform-specific concerns (token storage, analytics, etc.). See root CLAUDE.md for full project context.

## Anti-Cheat Layer Mapping

| Layer | Web | Mobile | Notes |
|-------|-----|--------|-------|
| L1 DevTools blocking | keyboard/contextmenu events | N/A | Not applicable on mobile |
| L2 Adblock detection | bait script `/ads.js` | N/A | Mobile ads are SDK-embedded |
| L3 Bot protection | Cloudflare Turnstile | Device attestation / reCAPTCHA Mobile | Requires server-side changes to accept mobile tokens |
| L4 Dwell timer | requestAnimationFrame + performance.now() | react-native-reanimated useFrameCallback or setInterval + Date.now() | Must match server's EARN_MIN_DURATION_SEC |
| L5 Visibility/exit blocking | visibilitychange + history.pushState + beforeunload | AppState API + BackHandler (Android) | iOS has no back button — handle via navigation guards |

## Web-to-Mobile Component Mapping

| Web (React) | Mobile (React Native) |
|-------------|----------------------|
| `<div>` | `<View>` |
| `<span>`, `<p>` | `<Text>` |
| `<img>` | `<Image>` |
| `<a>` / `<Link>` | `<TouchableOpacity>` + navigation |
| `<input>` | `<TextInput>` |
| `<button>` | `<Pressable>` or `<TouchableOpacity>` |
| `<select>` | `@react-native-picker/picker` |
| React Router | React Navigation (stack, tab, modal) |
| localStorage | AsyncStorage |
| sessionStorage | In-memory (React state/ref) |
| document.cookie | expo-secure-store |
| fetch + credentials:"include" | fetch + Authorization header (Bearer token) |
| IntersectionObserver | onScroll / FlatList viewability |
| window.scrollTo | ScrollView.scrollTo / FlatList.scrollToOffset |

## Auth Flow Difference

Web uses HTTP-only cookies (`credentials: "include"`). Mobile cannot use cookies reliably, so:
- Store JWT in SecureStore after OAuth callback
- Attach as `Authorization: Bearer <token>` header on every request
- Server must accept both cookie and Bearer token auth (verify server supports this before implementing)

## Push Notification Architecture

Mobile uses `expo-notifications` for native push notifications. This is a mobile-only feature (web uses email delivery only).

### How It Works
1. App starts → request push permission → get Expo Push Token
2. Token sent to server via `POST /api/v1/mobile/push-token`
3. Server stores token in `push_tokens` table (user_id, token, platform)
4. Server sends push via Expo Push API: `POST https://exp.host/--/api/v2/push/send`
5. Payload: `{to: "ExponentPushToken[xxx]", title, body, data: {screen, params}}`

### Use Cases
- Daily briefing arrival (7AM KST)
- Withdrawal status updates (approved/rejected)
- Marketing/engagement notifications

### Mobile-Only API Endpoints
| Method | Path | Description |
|--------|------|-------------|
| POST | /api/v1/mobile/push-token | Register device push token |
| DELETE | /api/v1/mobile/push-token | Unregister push token |

## Web vs Mobile Differences

While mobile follows web as source of truth for business logic, these platform concerns differ:

| Concern | Web | Mobile | Notes |
|---------|-----|--------|-------|
| Auth transport | HTTP-only cookie (`credentials: "include"`) | Bearer token in SecureStore | Server accepts both |
| Token refresh | Cookie-based POST /auth/refresh | Bearer token refresh | Same endpoint, different transport |
| Bot protection | Cloudflare Turnstile | Device attestation (placeholder) | Server must accept mobile tokens |
| Ads | AdSense (web SDK) | react-native-google-mobile-ads | SDK-embedded, no adblock concern |
| Adblock detection | Bait script `/ads.js` | N/A | Mobile ads are SDK-embedded |
| Push notifications | N/A (email only) | expo-notifications | Mobile-only feature |
| DevTools blocking | keyboard/contextmenu events | N/A | Not applicable on mobile |
| Deep linking | React Router URLs | Expo Linking + React Navigation | Different URL schemes |
| API client | fetch + credentials:"include" | fetch + Authorization Bearer header | Shared via ApiAdapter pattern |
| Dwell timer | requestAnimationFrame + performance.now() | setInterval + Date.now() + AppState | Must match EARN_MIN_DURATION_SEC |
| Visibility detection | visibilitychange + beforeunload | AppState API + BackHandler (Android) | iOS has no back button |
| Storage | localStorage / sessionStorage / cookie | AsyncStorage / SecureStore / React state | See component mapping table |

## Key Web Files to Reference

| Feature | Web file | What to look at |
|---------|----------|-----------------|
| API client + token refresh | `web/src/lib/api.ts` | apiFetch wrapper, 401 retry, rate limit handling |
| Auth state | `web/src/contexts/auth-context.tsx` | fetchMe on mount, logout, refreshUser |
| Coin earning flow | `web/src/pages/topic.tsx` | init-earn → countdown → earn sequence, all 5 anti-cheat layers |
| Adblock detection | `web/src/lib/adblock.ts` | Bait method (replaced by SDK check on mobile) |
| Landing page | `web/src/pages/landing.tsx` | FadeIn animations, scroll restoration, login prompt |
| Topic listing | `web/src/pages/latest.tsx` | Brain category grouping, batch earn status, filters |
| My page | `web/src/pages/mypage.tsx` | Tabs, coin/withdrawal history, bank account modal |
| Withdrawal | `web/src/pages/withdrawal.tsx` | Request flow, bank account management |
| Terms consent | `web/src/pages/terms-consent.tsx` | Two-phase signup, required term validation |
| Kakao login | `web/src/components/kakao-login-button.tsx` | OAuth redirect (mobile uses native SDK instead) |
| Utilities | `web/src/lib/utils.ts` | Formatting helpers — candidate for shared package |

## Testing

- Unit tests for shared business logic
- Component tests with React Native Testing Library
- E2E with Detox or Maestro
- Test against same server API as web (same endpoints, same validation)
