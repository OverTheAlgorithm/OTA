import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import type { ThemeMode } from "@/lib/colors";

interface ThemeContextType {
  mode: ThemeMode;
  toggleTheme: () => void;
  setTheme: (mode: ThemeMode) => void;
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

const THEME_STORAGE_KEY = "ota-theme-mode";
const DEFAULT_THEME: ThemeMode = "light";

export function ThemeProvider({ children }: { children: ReactNode }) {
  // Initialize from localStorage if available (client-side only)
  const getInitialTheme = (): ThemeMode => {
    if (typeof window === "undefined") return DEFAULT_THEME;
    const stored = localStorage.getItem(THEME_STORAGE_KEY) as ThemeMode | null;
    return stored || DEFAULT_THEME;
  };

  const [mode, setMode] = useState<ThemeMode>(DEFAULT_THEME);

  // Apply theme whenever mode changes
  useEffect(() => {
    const initial = getInitialTheme();
    setMode(initial);
    applyTheme(initial);
  }, []);

  // Watch for changes to mode and apply theme
  useEffect(() => {
    applyTheme(mode);
  }, [mode]);

  const applyTheme = (theme: ThemeMode) => {
    document.documentElement.setAttribute("data-theme", theme);
    document.documentElement.classList.remove("light", "dark");
    document.documentElement.classList.add(theme);
  };

  const setTheme = (newMode: ThemeMode) => {
    setMode(newMode);
    localStorage.setItem(THEME_STORAGE_KEY, newMode);
  };

  const toggleTheme = () => {
    const newMode = mode === "light" ? "dark" : "light";
    setTheme(newMode);
  };

  return (
    <ThemeContext.Provider value={{ mode, toggleTheme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (!context) {
    throw new Error("useTheme must be used within ThemeProvider");
  }
  return context;
}
