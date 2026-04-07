// Ported from: web/src/pages/admin-coins.tsx
import React, { useState } from "react";
import {
  View,
  Text,
  ScrollView,
  Pressable,
  TextInput,
  ActivityIndicator,
  Alert,
  StyleSheet,
} from "react-native";
import { useNavigation } from "@react-navigation/native";
import { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { useAuth } from "../contexts/auth-context";
import { api } from "../lib/api";
import type { AdminUserSearchResult } from "../../../packages/shared/src/types";
import { formatDateTime } from "../../../packages/shared/src/utils";

type NavProp = NativeStackNavigationProp<Record<string, object | undefined>>;

export function AdminCoinsScreen() {
  const { user, loading: authLoading } = useAuth();
  const navigation = useNavigation<NavProp>();

  const [searchType, setSearchType] = useState<"id" | "email">("email");
  const [query, setQuery] = useState("");
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [result, setResult] = useState<AdminUserSearchResult | null>(null);

  const [newCoins, setNewCoins] = useState("");
  const [memo, setMemo] = useState("");
  const [adjusting, setAdjusting] = useState(false);
  const [adjustError, setAdjustError] = useState<string | null>(null);

  const handleSearch = async () => {
    const q = query.trim();
    if (!q) return;
    setSearching(true);
    setSearchError(null);
    setResult(null);
    setNewCoins("");
    setMemo("");
    setAdjustError(null);
    try {
      const data = await api.adminSearchUser(searchType, q);
      setResult(data);
      setNewCoins(String(data.level.total_coins));
    } catch (e) {
      setSearchError(e instanceof Error ? e.message : "검색 실패");
    } finally {
      setSearching(false);
    }
  };

  const handleAdjust = () => {
    if (!result) return;

    const trimmedMemo = memo.trim();
    if (!trimmedMemo) {
      setAdjustError("비고(사유)는 필수입니다");
      return;
    }

    const coins = parseInt(newCoins, 10);
    if (isNaN(coins) || coins < 0) {
      setAdjustError("포인트는 0 이상의 정수여야 합니다");
      return;
    }

    const delta = coins - result.level.total_coins;
    const deltaStr = delta >= 0 ? `+${delta.toLocaleString()}` : delta.toLocaleString();

    Alert.alert(
      "포인트 수정 확인",
      `사용자의 포인트 보유량을 직접적으로 수정하는 것은 매우 신중한 결정이 필요합니다.\n\n` +
        `대상: ${result.user.nickname || result.user.email || result.user.id}\n` +
        `현재 포인트: ${result.level.total_coins.toLocaleString()}\n` +
        `변경 후: ${coins.toLocaleString()}\n` +
        `차이: ${deltaStr}\n` +
        `사유: ${trimmedMemo}\n\n` +
        `정말 진행하시겠습니까?`,
      [
        { text: "취소", style: "cancel" },
        {
          text: "수정",
          style: "destructive",
          onPress: async () => {
            setAdjusting(true);
            setAdjustError(null);
            try {
              const adjustResult = await api.adminAdjustCoins(result.user.id, coins, trimmedMemo);
              setResult({ user: result.user, level: adjustResult.level });
              setNewCoins(String(adjustResult.new_coins));
              setMemo("");
              const adjDelta = adjustResult.delta >= 0
                ? `+${adjustResult.delta.toLocaleString()}`
                : adjustResult.delta.toLocaleString();
              Alert.alert(
                "수정 완료",
                `포인트가 수정되었습니다.\n\n변동: ${adjDelta}\n현재 보유량: ${adjustResult.new_coins.toLocaleString()}`
              );
            } catch (e) {
              setAdjustError(e instanceof Error ? e.message : "포인트 수정 실패");
            } finally {
              setAdjusting(false);
            }
          },
        },
      ]
    );
  };

  if (authLoading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator size="large" color="#4a9fe5" />
      </View>
    );
  }

  if (!user || user.role !== "admin") {
    return (
      <View style={styles.centered}>
        <Text style={styles.deniedText}>접근 권한이 없습니다</Text>
        <Pressable style={styles.backBtn} onPress={() => navigation.goBack()}>
          <Text style={styles.backBtnText}>돌아가기</Text>
        </Pressable>
      </View>
    );
  }

  const parsedNewCoins = parseInt(newCoins, 10);
  const delta = result && !isNaN(parsedNewCoins)
    ? parsedNewCoins - result.level.total_coins
    : null;

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {/* Search */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>유저 검색</Text>

        <View style={styles.typeRow}>
          <Pressable
            style={[styles.typeBtn, searchType === "email" && styles.typeBtnActive]}
            onPress={() => setSearchType("email")}
          >
            <Text style={[styles.typeBtnText, searchType === "email" && styles.typeBtnTextActive]}>
              이메일
            </Text>
          </Pressable>
          <Pressable
            style={[styles.typeBtn, searchType === "id" && styles.typeBtnActive]}
            onPress={() => setSearchType("id")}
          >
            <Text style={[styles.typeBtnText, searchType === "id" && styles.typeBtnTextActive]}>
              UUID
            </Text>
          </Pressable>
        </View>

        <View style={styles.searchRow}>
          <TextInput
            value={query}
            onChangeText={setQuery}
            onSubmitEditing={handleSearch}
            placeholder={searchType === "email" ? "user@example.com" : "유저 UUID"}
            style={[styles.input, { flex: 1 }]}
            autoCapitalize="none"
            keyboardType={searchType === "email" ? "email-address" : "default"}
            returnKeyType="search"
          />
          <Pressable
            style={[styles.searchBtn, (searching || !query.trim()) && styles.btnDisabled]}
            onPress={handleSearch}
            disabled={searching || !query.trim()}
          >
            <Text style={styles.searchBtnText}>{searching ? "검색 중..." : "검색"}</Text>
          </Pressable>
        </View>

        {searchError && <Text style={styles.errorText}>{searchError}</Text>}
      </View>

      {/* User info */}
      {result && (
        <>
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>유저 정보</Text>

            <View style={styles.infoGrid}>
              <View style={styles.infoRow}>
                <Text style={styles.infoLabel}>ID</Text>
                <Text style={[styles.infoValue, styles.mono]} numberOfLines={1} ellipsizeMode="middle">
                  {result.user.id}
                </Text>
              </View>
              <View style={styles.infoRow}>
                <Text style={styles.infoLabel}>닉네임</Text>
                <Text style={styles.infoValue}>{result.user.nickname || "-"}</Text>
              </View>
              <View style={styles.infoRow}>
                <Text style={styles.infoLabel}>이메일</Text>
                <Text style={styles.infoValue}>
                  {result.user.email || "-"}
                  {result.user.email_verified ? " (인증됨)" : ""}
                </Text>
              </View>
              <View style={styles.infoRow}>
                <Text style={styles.infoLabel}>역할</Text>
                <Text style={styles.infoValue}>{result.user.role}</Text>
              </View>
              <View style={styles.infoRow}>
                <Text style={styles.infoLabel}>가입일</Text>
                <Text style={styles.infoValue}>{formatDateTime(result.user.created_at)}</Text>
              </View>
            </View>

            <View style={styles.coinSummary}>
              <View style={styles.coinItem}>
                <Text style={styles.coinLabel}>현재 포인트</Text>
                <Text style={styles.coinValue}>{result.level.total_coins.toLocaleString()}</Text>
              </View>
              <View style={styles.coinItem}>
                <Text style={styles.coinLabel}>레벨</Text>
                <Text style={[styles.coinValue, { color: "#4a9fe5" }]}>
                  Lv.{result.level.level}
                </Text>
              </View>
            </View>
          </View>

          {/* Adjust */}
          <View style={[styles.section, styles.sectionDanger]}>
            <Text style={[styles.sectionTitle, { color: "#ff5442" }]}>포인트 보유량 수정</Text>
            <Text style={styles.warningText}>
              이 작업은 최후의 수단입니다. 모든 수정은 관리자 ID와 함께 영구적으로 기록됩니다.
            </Text>

            <Text style={styles.fieldLabel}>변경할 포인트 값</Text>
            <TextInput
              value={newCoins}
              onChangeText={setNewCoins}
              style={styles.input}
              placeholder="새로운 포인트 보유량"
              keyboardType="number-pad"
            />
            {newCoins !== "" && delta !== null && (
              <Text style={[styles.deltaText, { color: delta >= 0 ? "#15803d" : "#ff5442" }]}>
                차이: {delta >= 0 ? "+" : ""}{delta.toLocaleString()}
              </Text>
            )}

            <Text style={styles.fieldLabel}>비고 (필수)</Text>
            <TextInput
              value={memo}
              onChangeText={setMemo}
              style={[styles.input, { minHeight: 70 }]}
              placeholder="수정 사유를 반드시 입력하세요 (예: 장애 보상, 오류 수정 등)"
              multiline
            />

            {adjustError && <Text style={styles.errorText}>{adjustError}</Text>}

            <Pressable
              style={[
                styles.adjustBtn,
                (adjusting || !memo.trim() || newCoins === "") && styles.btnDisabled,
              ]}
              onPress={handleAdjust}
              disabled={adjusting || !memo.trim() || newCoins === ""}
            >
              <Text style={styles.adjustBtnText}>
                {adjusting ? "수정 중..." : "포인트 수정 실행"}
              </Text>
            </Pressable>
          </View>
        </>
      )}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "#f8fafc",
  },
  content: {
    padding: 16,
    paddingBottom: 40,
    gap: 16,
  },
  centered: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "#f8fafc",
    gap: 16,
  },
  section: {
    borderRadius: 16,
    borderWidth: 1,
    borderColor: "#d4e6f5",
    backgroundColor: "#f0f7ff",
    padding: 16,
    gap: 10,
  },
  sectionDanger: {
    borderColor: "rgba(255,84,66,0.3)",
    backgroundColor: "rgba(255,84,66,0.05)",
  },
  sectionTitle: {
    fontSize: 16,
    fontWeight: "600",
    color: "#1e3a5f",
  },
  typeRow: {
    flexDirection: "row",
    gap: 8,
  },
  typeBtn: {
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: "#d4e6f5",
    backgroundColor: "#fff",
  },
  typeBtnActive: {
    backgroundColor: "#4a9fe5",
    borderColor: "#4a9fe5",
  },
  typeBtnText: {
    fontSize: 13,
    color: "#6b8db5",
  },
  typeBtnTextActive: {
    color: "#fff",
    fontWeight: "600",
  },
  searchRow: {
    flexDirection: "row",
    gap: 8,
    alignItems: "center",
  },
  input: {
    backgroundColor: "#fff",
    borderWidth: 1,
    borderColor: "#d4e6f5",
    borderRadius: 10,
    paddingHorizontal: 12,
    paddingVertical: 9,
    fontSize: 13,
    color: "#1e3a5f",
  },
  searchBtn: {
    backgroundColor: "#4a9fe5",
    paddingHorizontal: 16,
    paddingVertical: 9,
    borderRadius: 10,
  },
  searchBtnText: {
    color: "#fff",
    fontWeight: "600",
    fontSize: 13,
  },
  errorText: {
    fontSize: 13,
    color: "#ff5442",
  },
  infoGrid: {
    gap: 8,
  },
  infoRow: {
    flexDirection: "row",
    gap: 8,
  },
  infoLabel: {
    fontSize: 13,
    color: "#6b8db5",
    width: 60,
  },
  infoValue: {
    fontSize: 13,
    color: "#1e3a5f",
    flex: 1,
  },
  mono: {
    fontFamily: "monospace",
    fontSize: 11,
  },
  coinSummary: {
    flexDirection: "row",
    gap: 24,
    borderTopWidth: 1,
    borderTopColor: "#d4e6f5",
    paddingTop: 12,
    marginTop: 4,
  },
  coinItem: {
    gap: 2,
  },
  coinLabel: {
    fontSize: 11,
    color: "#6b8db5",
  },
  coinValue: {
    fontSize: 22,
    fontWeight: "700",
    color: "#1e3a5f",
  },
  warningText: {
    fontSize: 11,
    color: "#6b8db5",
  },
  fieldLabel: {
    fontSize: 12,
    color: "#6b8db5",
  },
  deltaText: {
    fontSize: 12,
    fontWeight: "600",
  },
  adjustBtn: {
    backgroundColor: "#ff5442",
    borderRadius: 12,
    paddingVertical: 12,
    alignItems: "center",
    marginTop: 4,
  },
  adjustBtnText: {
    color: "#fff",
    fontWeight: "600",
    fontSize: 14,
  },
  btnDisabled: {
    opacity: 0.5,
  },
  deniedText: {
    fontSize: 16,
    color: "#6b8db5",
  },
  backBtn: {
    paddingHorizontal: 20,
    paddingVertical: 10,
    borderRadius: 12,
    backgroundColor: "#4a9fe5",
  },
  backBtnText: {
    color: "#fff",
    fontWeight: "600",
    fontSize: 14,
  },
});
