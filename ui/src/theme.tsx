import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type ThemePreference =
  | "system"
  | "light"
  | "sand"
  | "dark"
  | "slate"
  | "forest"
  | "ocean";
export type ResolvedTheme = "light" | "dark";
export type AppliedTheme = Exclude<ThemePreference, "system">;

export const THEME_OPTIONS: ReadonlyArray<{
  id: ThemePreference;
  label: string;
}> = [
  { id: "system", label: "System" },
  { id: "light", label: "Paper" },
  { id: "sand", label: "Sand" },
  { id: "dark", label: "Night" },
  { id: "slate", label: "Slate" },
  { id: "forest", label: "Forest" },
  { id: "ocean", label: "Ocean" },
];

export const THEME_STORAGE_KEY = "wts.appearance.theme.v1";
export const SYSTEM_DARK_QUERY = "(prefers-color-scheme: dark)";

interface ThemeContextValue {
  preference: ThemePreference;
  activeTheme: AppliedTheme;
  resolvedTheme: ResolvedTheme;
  setPreference: (preference: ThemePreference) => void;
  toggleTheme: () => void;
}

const fallbackThemeContext: ThemeContextValue = {
  preference: "system",
  activeTheme: "light",
  resolvedTheme: "light",
  setPreference: () => undefined,
  toggleTheme: () => undefined,
};

const ThemeContext = createContext<ThemeContextValue>(fallbackThemeContext);

function availableStorage(): Pick<Storage, "getItem" | "setItem"> | undefined {
  try {
    return globalThis.localStorage;
  } catch {
    return undefined;
  }
}

export function normalizeThemePreference(
  value: string | null | undefined,
): ThemePreference {
  return THEME_OPTIONS.find((option) => option.id === value)?.id ?? "system";
}

export function loadThemePreference(
  storage: Pick<Storage, "getItem"> | undefined = availableStorage(),
): ThemePreference {
  if (!storage) return "system";
  try {
    return normalizeThemePreference(storage.getItem(THEME_STORAGE_KEY));
  } catch {
    return "system";
  }
}

export function resolveTheme(
  preference: ThemePreference,
  systemPrefersDark: boolean,
): ResolvedTheme {
  const activeTheme = resolveAppliedTheme(preference, systemPrefersDark);
  return activeTheme === "light" || activeTheme === "sand" ? "light" : "dark";
}

export function resolveAppliedTheme(
  preference: ThemePreference,
  systemPrefersDark: boolean,
): AppliedTheme {
  return preference === "system"
    ? systemPrefersDark
      ? "dark"
      : "light"
    : preference;
}

export function applyResolvedTheme(
  theme: AppliedTheme,
  documentTarget: Pick<Document, "documentElement" | "querySelector"> | undefined =
    globalThis.document,
) {
  if (!documentTarget) return;
  const colorScheme = theme === "light" || theme === "sand" ? "light" : "dark";
  documentTarget.documentElement.dataset.theme = theme;
  documentTarget.documentElement.dataset.colorScheme = colorScheme;
  documentTarget.documentElement.style.colorScheme = colorScheme;
  const chromeColors: Record<AppliedTheme, string> = {
    light: "#f2f4f7",
    sand: "#f4efe5",
    dark: "#0f141c",
    slate: "#15191f",
    forest: "#0d1713",
    ocean: "#0b1720",
  };
  documentTarget
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute("content", chromeColors[theme]);
}

function currentSystemPreference() {
  return globalThis.matchMedia?.(SYSTEM_DARK_QUERY).matches ?? false;
}

export function initializeTheme() {
  const preference = loadThemePreference();
  const systemPrefersDark = currentSystemPreference();
  const activeTheme = resolveAppliedTheme(preference, systemPrefersDark);
  const resolvedTheme = resolveTheme(preference, systemPrefersDark);
  applyResolvedTheme(activeTheme);
  return { preference, activeTheme, resolvedTheme };
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreferenceState] =
    useState<ThemePreference>(loadThemePreference);
  const [systemPrefersDark, setSystemPrefersDark] = useState(
    currentSystemPreference,
  );
  const resolvedTheme = resolveTheme(preference, systemPrefersDark);
  const activeTheme = resolveAppliedTheme(preference, systemPrefersDark);

  useEffect(() => {
    const media = globalThis.matchMedia?.(SYSTEM_DARK_QUERY);
    if (!media) return;
    const update = (event: MediaQueryListEvent) =>
      setSystemPrefersDark(event.matches);
    setSystemPrefersDark(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);

  useLayoutEffect(() => {
    applyResolvedTheme(activeTheme);
  }, [activeTheme]);

  const setPreference = useCallback((next: ThemePreference) => {
    setPreferenceState(next);
    try {
      availableStorage()?.setItem(THEME_STORAGE_KEY, next);
    } catch {
      // A private or locked-down webview may reject local persistence.
    }
  }, []);

  const toggleTheme = useCallback(() => {
    setPreference(resolvedTheme === "dark" ? "light" : "dark");
  }, [resolvedTheme, setPreference]);

  const value = useMemo(
    () => ({
      preference,
      activeTheme,
      resolvedTheme,
      setPreference,
      toggleTheme,
    }),
    [activeTheme, preference, resolvedTheme, setPreference, toggleTheme],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  return useContext(ThemeContext);
}
