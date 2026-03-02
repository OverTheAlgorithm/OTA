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
  const [mode, setMode] = useState<ThemeMode>(DEFAULT_THEME);
  const [mounted, setMounted] = useState(false);

  // Initialize theme from localStorage
  useEffect(() => {
    const stored = localStorage.getItem(THEME_STORAGE_KEY) as ThemeMode | null;
    const initial = stored || DEFAULT_THEME;
    setMode(initial);
    applyTheme(initial);
    setMounted(true);
  }, []);

  const applyTheme = (theme: ThemeMode) => {
    document.documentElement.setAttribute("data-theme", theme);
    // Also update body class for tailwind support if needed
    document.documentElement.classList.remove("light", "dark");
    document.documentElement.classList.add(theme);
  };

  const setTheme = (newMode: ThemeMode) => {
    setMode(newMode);
    applyTheme(newMode);
    localStorage.setItem(THEME_STORAGE_KEY, newMode);
  };

  const toggleTheme = () => {
    const newMode = mode === "light" ? "dark" : "light";
    setTheme(newMode);
  };

  // Prevent flash of unstyled content
  if (!mounted) {
    return <>{children}</>;
  }

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
