import { useEffect, useState } from "react";
import { getUserLevel, type LevelInfo } from "@/lib/api";
import { useAuth } from "@/contexts/auth-context";
import { LevelCard } from "./level-card";

/**
 * Self-contained level card that fetches its own data.
 * Renders nothing when the user is not logged in or data is unavailable.
 * Pass a changing `refreshKey` to trigger a re-fetch (e.g. after earning coins).
 */
export function UserLevelCard({ refreshKey = 0 }: { refreshKey?: number }) {
  const { user } = useAuth();
  const [level, setLevel] = useState<LevelInfo | null>(null);

  useEffect(() => {
    if (!user) {
      setLevel(null);
      return;
    }
    getUserLevel().then(setLevel).catch(() => {});
  }, [user, refreshKey]);

  if (!level) return null;

  return <LevelCard level={level} />;
}
