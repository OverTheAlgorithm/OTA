export const COLORS = {
  light: {
    background: "white",
    foreground: "#1e3a5f",
    cardBg: "#f0f7ff",
    border: "#d4e6f5",
    text: {
      primary: "#1e3a5f",
      secondary: "#6b8db5",
      muted: "#a8bcc9",
    },
    button: {
      primary: "#26b0ff",
      primaryHover: "#1a9fed",
      icon: "#4a9fe5",
      iconBg: "#4a9fe5",
    },
    feedback: {
      error: "#ff5442",
      errorBg: "#ff5442",
      success: "green",
      warning: "#f0923b",
    },
  },
  dark: {
    background: "#0f0a19",
    foreground: "#f5f0ff",
    cardBg: "#1a1229",
    border: "#2d1f42",
    text: {
      primary: "#f5f0ff",
      secondary: "#9b8bb4",
      muted: "#6b5b7f",
    },
    button: {
      primary: "#5ba4d9",
      primaryHover: "#4a8fc2",
      icon: "#5ba4d9",
      iconBg: "#5ba4d9",
    },
    feedback: {
      error: "#e84d3d",
      errorBg: "#e84d3d",
      success: "#7bc67e",
      warning: "#f0923b",
    },
  },
} as const;

export type ThemeMode = "light" | "dark";
export type Colors = (typeof COLORS)[ThemeMode];
