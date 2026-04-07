// Environment configuration for the mobile app.
// Change API_BASE to your local server for development:
//   e.g., "http://192.168.0.10:8080" (use your PC's local IP, not localhost)
//
// To find your local IP:
//   Windows: ipconfig → IPv4 Address
//   Mac/Linux: ifconfig | grep inet

const __DEV__ = process.env.NODE_ENV !== "production";

export const API_BASE = __DEV__
  ? "http://192.168.0.10:8080" // ← 개발 시 본인 PC의 로컬 IP로 변경
  : "https://server.mindhacker.club";
