// Ported from: web/src/pages/mypage.tsx

import React, { useEffect, useState } from "react";
import {
  View,
  Text,
  StyleSheet,
  FlatList,
  TouchableOpacity,
  Alert,
  ScrollView,
  ActivityIndicator,
} from "react-native";
import { useNavigation } from "@react-navigation/native";
import type { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { api } from "../lib/api";
import { useAuth } from "../contexts/auth-context";
import { UserLevelCard } from "../components/UserLevelCard";
import { InterestSection } from "../components/InterestSection";
import { ChannelPreferencesSection } from "../components/ChannelPreferencesSection";
import type { CoinTransaction, WithdrawalDetail } from "../../../packages/shared/src/types";

type Tab = "points" | "withdrawals" | "settings";

type RootStackParamList = {
  Landing: undefined;
  Withdrawal: undefined;
};

const COIN_TYPE_LABELS: Record<string, string> = {
  signup_bonus: "가입 보너스",
  admin_set: "포인트 조정",
  admin_adjust: "포인트 조정",
};

function getCoinLabel(tx: CoinTransaction): string {
  return COIN_TYPE_LABELS[tx.type] ?? tx.description ?? tx.type;
}

function formatDateTime(iso: string): string {
  const d = new Date(iso);
  return `${d.getFullYear()}.${String(d.getMonth() + 1).padStart(2, "0")}.${String(d.getDate()).padStart(2, "0")} ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

const STATUS_LABELS: Record<string, string> = {
  pending: "처리중",
  approved: "완료",
  rejected: "거절",
  cancelled: "취소",
};

const STATUS_COLORS: Record<string, { bg: string; text: string; border: string }> = {
  pending: { bg: "#ffefc6", text: "#8a6d00", border: "#e0b830" },
  approved: { bg: "#d4f5e2", text: "#1a6b3a", border: "#2ea55e" },
  rejected: { bg: "#ffe0dc", text: "#9c2b1e", border: "#e04b3a" },
  cancelled: { bg: "#e8e8e8", text: "#636363", border: "#999" },
};

// ── Point History Tab ────────────────────────────────────────────────────────

function PointHistoryTab() {
  const [transactions, setTransactions] = useState<CoinTransaction[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .getCoinHistory(20, 0)
      .then(({ data, has_more }) => {
        setTransactions(data);
        setHasMore(has_more);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const loadMore = async () => {
    try {
      const { data, has_more } = await api.getCoinHistory(20, transactions.length);
      setTransactions((prev) => [...prev, ...data]);
      setHasMore(has_more);
    } catch {
      // silently fail
    }
  };

  if (loading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator color="#43b9d6" />
      </View>
    );
  }

  if (transactions.length === 0) {
    return (
      <View style={styles.centered}>
        <Text style={styles.emptyText}>포인트 내역이 없습니다.</Text>
      </View>
    );
  }

  return (
    <View>
      {transactions.map((tx) => (
        <View key={tx.id} style={styles.historyItem}>
          <View style={styles.historyRow}>
            <View style={styles.historyLeft}>
              <Text style={styles.historyTitle}>{getCoinLabel(tx)}</Text>
              <Text style={styles.historyDate}>{formatDateTime(tx.created_at)}</Text>
            </View>
            {tx.amount > 0 && (
              <View style={[styles.badge, styles.badgePositive]}>
                <Text style={styles.badgeText}>+{tx.amount}포인트</Text>
              </View>
            )}
            {tx.amount < 0 && (
              <View style={[styles.badge, styles.badgeNegative]}>
                <Text style={styles.badgeText}>{tx.amount}포인트</Text>
              </View>
            )}
          </View>
        </View>
      ))}
      {hasMore && (
        <TouchableOpacity onPress={loadMore} style={styles.loadMoreButton}>
          <Text style={styles.loadMoreText}>더 보기</Text>
        </TouchableOpacity>
      )}
    </View>
  );
}

// ── Withdrawal History Tab ───────────────────────────────────────────────────

function WithdrawalHistoryTab() {
  const [items, setItems] = useState<WithdrawalDetail[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .getWithdrawalHistory(20, 0)
      .then(({ data, has_more }) => {
        setItems(data);
        setHasMore(has_more);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const loadMore = async () => {
    try {
      const { data, has_more } = await api.getWithdrawalHistory(20, items.length);
      setItems((prev) => [...prev, ...data]);
      setHasMore(has_more);
    } catch {
      // silently fail
    }
  };

  if (loading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator color="#43b9d6" />
      </View>
    );
  }

  if (items.length === 0) {
    return (
      <View style={styles.centered}>
        <Text style={styles.emptyText}>출금 내역이 없습니다.</Text>
      </View>
    );
  }

  return (
    <View>
      {items.map((w) => {
        const sc = STATUS_COLORS[w.current_status];
        return (
          <View key={w.id} style={styles.historyItem}>
            <View style={styles.historyRow}>
              <View style={styles.historyLeft}>
                <Text style={styles.historyDate}>{formatDateTime(w.created_at)}</Text>
                <Text style={styles.historyTitle}>
                  {w.amount.toLocaleString()}포인트 출금
                </Text>
              </View>
              {sc && (
                <View
                  style={[
                    styles.badge,
                    { backgroundColor: sc.bg, borderColor: sc.border },
                  ]}
                >
                  <Text style={[styles.badgeText, { color: sc.text }]}>
                    {STATUS_LABELS[w.current_status] ?? w.current_status}
                  </Text>
                </View>
              )}
            </View>
          </View>
        );
      })}
      {hasMore && (
        <TouchableOpacity onPress={loadMore} style={styles.loadMoreButton}>
          <Text style={styles.loadMoreText}>더 보기</Text>
        </TouchableOpacity>
      )}
    </View>
  );
}

// ── Settings Tab ─────────────────────────────────────────────────────────────

function SettingsTab({ onLogout }: { onLogout: () => void }) {
  const [subscriptions, setSubscriptions] = useState<string[]>([]);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    api.getSubscriptions().then(setSubscriptions).catch(() => {});
  }, []);

  const handleDeleteAccount = () => {
    Alert.alert(
      "회원 탈퇴",
      "삭제된 계정은 복구할 수 없으며, 보유 중인 포인트와 모든 데이터는 삭제됩니다. 그래도 탈퇴하시겠습니까?",
      [
        { text: "취소", style: "cancel" },
        {
          text: "탈퇴 하기",
          style: "destructive",
          onPress: async () => {
            setDeleting(true);
            try {
              await api.deleteAccount();
              onLogout();
            } catch (e) {
              Alert.alert(
                "오류",
                e instanceof Error ? e.message : "계정 삭제에 실패했습니다"
              );
            } finally {
              setDeleting(false);
            }
          },
        },
      ]
    );
  };

  return (
    <View style={styles.settingsContainer}>
      <InterestSection selected={subscriptions} onChange={setSubscriptions} />
      <View style={styles.sectionSpacer} />
      <ChannelPreferencesSection />
      <View style={styles.sectionSpacer} />
      <TouchableOpacity
        onPress={handleDeleteAccount}
        disabled={deleting}
        style={styles.deleteButton}
      >
        <Text style={styles.deleteButtonText}>
          {deleting ? "처리 중..." : "회원 탈퇴"}
        </Text>
      </TouchableOpacity>
    </View>
  );
}

// ── Screen ────────────────────────────────────────────────────────────────────

export function MyPageScreen() {
  const { user, loading: authLoading, logout } = useAuth();
  const navigation = useNavigation<NativeStackNavigationProp<RootStackParamList>>();
  const [tab, setTab] = useState<Tab>("points");

  useEffect(() => {
    if (!authLoading && !user) {
      navigation.replace("Landing");
    }
  }, [user, authLoading, navigation]);

  if (authLoading || !user) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator color="#43b9d6" />
      </View>
    );
  }

  const handleLogout = async () => {
    await logout();
    navigation.replace("Landing");
  };

  return (
    <ScrollView style={styles.screen} contentContainerStyle={styles.content}>
      <View style={styles.levelCardWrapper}>
        <UserLevelCard
          onWithdrawPress={() => navigation.navigate("Withdrawal")}
        />
      </View>

      {/* Tab bar */}
      <View style={styles.tabBar}>
        {(["points", "withdrawals", "settings"] as Tab[]).map((t) => (
          <TouchableOpacity
            key={t}
            onPress={() => setTab(t)}
            style={[styles.tabItem, tab === t && styles.tabItemActive]}
          >
            <Text style={[styles.tabLabel, tab === t && styles.tabLabelActive]}>
              {t === "points" ? "포인트 내역" : t === "withdrawals" ? "출금 내역" : "내 정보"}
            </Text>
          </TouchableOpacity>
        ))}
      </View>

      {/* Tab content */}
      <View style={styles.tabContent}>
        {tab === "points" && <PointHistoryTab />}
        {tab === "withdrawals" && <WithdrawalHistoryTab />}
        {tab === "settings" && <SettingsTab onLogout={handleLogout} />}
      </View>

      {/* Logout */}
      <TouchableOpacity onPress={handleLogout} style={styles.logoutButton}>
        <Text style={styles.logoutText}>로그아웃</Text>
      </TouchableOpacity>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: "#fdf9ee",
  },
  content: {
    padding: 16,
    paddingBottom: 40,
  },
  centered: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    paddingVertical: 40,
  },
  emptyText: {
    fontSize: 13,
    color: "rgba(35,24,21,0.5)",
  },
  levelCardWrapper: {
    marginBottom: 20,
  },
  tabBar: {
    flexDirection: "row",
    borderBottomWidth: 1,
    borderBottomColor: "#dbdade",
    marginBottom: 20,
  },
  tabItem: {
    flex: 1,
    paddingVertical: 12,
    alignItems: "center",
  },
  tabItemActive: {
    borderBottomWidth: 3,
    borderBottomColor: "#008fb2",
  },
  tabLabel: {
    fontSize: 14,
    fontWeight: "500",
    color: "#231815",
  },
  tabLabelActive: {
    color: "#008fb2",
  },
  tabContent: {
    minHeight: 200,
  },
  historyItem: {
    borderLeftWidth: 3,
    borderLeftColor: "#43b9d6",
    paddingLeft: 16,
    paddingVertical: 8,
    marginBottom: 12,
  },
  historyRow: {
    flexDirection: "row",
    alignItems: "flex-start",
    justifyContent: "space-between",
    gap: 8,
  },
  historyLeft: {
    flex: 1,
  },
  historyTitle: {
    fontSize: 15,
    fontWeight: "700",
    color: "#231815",
  },
  historyDate: {
    fontSize: 12,
    color: "rgba(35,24,21,0.6)",
    marginTop: 2,
  },
  badge: {
    paddingHorizontal: 10,
    paddingVertical: 3,
    borderRadius: 999,
    borderWidth: 1,
    borderColor: "#231815",
    backgroundColor: "#43b9d6",
  },
  badgePositive: {
    backgroundColor: "#43b9d6",
  },
  badgeNegative: {
    backgroundColor: "#e8e8e8",
  },
  badgeText: {
    fontSize: 12,
    fontWeight: "700",
    color: "#231815",
  },
  loadMoreButton: {
    paddingVertical: 12,
    alignItems: "center",
  },
  loadMoreText: {
    fontSize: 13,
    fontWeight: "500",
    color: "#008fb2",
  },
  settingsContainer: {
    gap: 0,
  },
  sectionSpacer: {
    height: 24,
  },
  deleteButton: {
    paddingTop: 16,
  },
  deleteButtonText: {
    fontSize: 13,
    color: "rgba(35,24,21,0.6)",
    textDecorationLine: "underline",
  },
  logoutButton: {
    marginTop: 32,
    alignItems: "center",
    paddingVertical: 12,
  },
  logoutText: {
    fontSize: 14,
    color: "rgba(35,24,21,0.6)",
    textDecorationLine: "underline",
  },
});
