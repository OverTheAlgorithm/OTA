// Extracted from: web/src/components/user-level-card.tsx + web/src/components/level-card.tsx

import React, { useEffect, useState } from "react";
import {
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  Alert,
} from "react-native";
import { api } from "../lib/api";
import { useAuth } from "../contexts/auth-context";
import type { LevelInfo } from "../../../packages/shared/src/types";

interface Props {
  refreshKey?: number;
  onWithdrawPress?: () => void;
}

export function UserLevelCard({ refreshKey = 0, onWithdrawPress }: Props) {
  const { user } = useAuth();
  const [level, setLevel] = useState<LevelInfo | null>(null);

  useEffect(() => {
    if (!user) {
      setLevel(null);
      return;
    }
    api.getUserLevel().then(setLevel).catch(() => {});
  }, [user, refreshKey]);

  if (!level) return null;

  const { total_coins, coin_cap, thresholds } = level;
  const currentThreshold =
    [...thresholds].reverse().find((t) => t <= total_coins) ?? 0;
  const nextThreshold = thresholds.find((t) => t > total_coins) ?? coin_cap;
  const levelRange = nextThreshold - currentThreshold;
  const levelProgress = total_coins - currentThreshold;
  const fillPercent =
    levelRange > 0 ? Math.min((levelProgress / levelRange) * 100, 100) : 100;
  const remaining = Math.max(nextThreshold - total_coins, 0);
  const isMaxLevel =
    total_coins >= (thresholds[thresholds.length - 1] ?? 0);

  return (
    <View style={styles.card}>
      {onWithdrawPress && (
        <TouchableOpacity
          style={styles.withdrawButton}
          onPress={onWithdrawPress}
        >
          <Text style={styles.withdrawButtonText}>출금하기</Text>
        </TouchableOpacity>
      )}

      <View style={styles.content}>
        <Text style={styles.levelText}>Lv.{level.level}</Text>
        <View style={styles.coinsRow}>
          <Text style={styles.coinsText}>
            {total_coins.toLocaleString()}
          </Text>
          <Text style={styles.coinsUnit}>포인트</Text>
        </View>

        <View style={styles.progressRow}>
          <View style={styles.progressTrack}>
            <View
              style={[styles.progressFill, { width: `${fillPercent}%` }]}
            />
          </View>
          <Text style={styles.progressLabel}>
            {levelProgress.toLocaleString()} / {levelRange.toLocaleString()}
          </Text>
        </View>

        <Text style={styles.remainingText}>
          {isMaxLevel
            ? "최고 레벨 달성!"
            : `${remaining.toLocaleString()} 포인트를 더 모으면 레벨업!`}
          {"\n"}현재 일일 한도: {level.daily_limit} 포인트
        </Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: "#fff",
    borderRadius: 22,
    borderWidth: 2,
    borderColor: "#231815",
    padding: 20,
  },
  withdrawButton: {
    position: "absolute",
    top: 14,
    right: 14,
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 999,
    backgroundColor: "#43b9d6",
    borderWidth: 2,
    borderColor: "#231815",
  },
  withdrawButtonText: {
    fontSize: 14,
    fontWeight: "600",
    color: "#231815",
  },
  content: {
    flex: 1,
  },
  levelText: {
    fontSize: 20,
    fontWeight: "700",
    color: "#231815",
  },
  coinsRow: {
    flexDirection: "row",
    alignItems: "baseline",
    gap: 4,
    marginTop: 2,
  },
  coinsText: {
    fontSize: 36,
    fontWeight: "700",
    color: "#231815",
  },
  coinsUnit: {
    fontSize: 14,
    fontWeight: "700",
    color: "#231815",
  },
  progressRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    marginTop: 8,
  },
  progressTrack: {
    flex: 1,
    height: 14,
    borderRadius: 999,
    backgroundColor: "#e8f4fd",
    borderWidth: 1,
    borderColor: "rgba(35,24,21,0.3)",
    overflow: "hidden",
  },
  progressFill: {
    height: "100%",
    borderRadius: 999,
    backgroundColor: "#43b9d6",
  },
  progressLabel: {
    fontSize: 12,
    fontWeight: "700",
    color: "#231815",
    minWidth: 60,
    textAlign: "right",
  },
  remainingText: {
    fontSize: 12,
    fontWeight: "700",
    color: "#231815",
    marginTop: 4,
    lineHeight: 18,
  },
});
