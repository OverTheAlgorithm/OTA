import { COLORS, type ThemeMode } from "./colors";

/**
 * Get color value for current theme
 * Usage: const bgColor = getThemeColor("light", "background");
 */
export function getThemeColor(mode: ThemeMode, path: string): string {
  const keys = path.split(".");
  let value: any = COLORS[mode];

  for (const key of keys) {
    value = value?.[key];
  }

  return value || "";
}

/**
 * Get all colors for current theme
 */
export function getThemeColors(mode: ThemeMode) {
  return COLORS[mode];
}

/**
 * Create inline styles for a theme
 * Example: { backgroundColor: colors.background, color: colors.text.primary }
 */
export function createThemeStyle(
  mode: ThemeMode,
  styleMap: Record<string, string>
): Record<string, any> {
  const colors = getThemeColors(mode);
  const result: Record<string, any> = {};

  for (const [cssProperty, colorPath] of Object.entries(styleMap)) {
    result[cssProperty] = getThemeColor(mode, colorPath);
  }

  return result;
}
