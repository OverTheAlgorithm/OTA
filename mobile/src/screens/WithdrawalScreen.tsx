// Ported from: web/src/pages/withdrawal.tsx

import React, { useCallback, useEffect, useState } from "react";
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TextInput,
  TouchableOpacity,
  Alert,
  ActivityIndicator,
} from "react-native";
import { useNavigation } from "@react-navigation/native";
import type { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { api } from "../lib/api";
import { useAuth } from "../contexts/auth-context";
import type {
  BankAccount,
  WithdrawalDetail,
  WithdrawalInfo,
  LevelInfo,
} from "../../../packages/shared/src/types";

type RootStackParamList = {
  Landing: undefined;
};

const STATUS_LABEL: Record<string, string> = {
  pending: "대기",
  approved: "승인",
  rejected: "거절",
  cancelled: "취소",
};

const STATUS_COLOR: Record<string, { text: string; bg: string; border: string }> = {
  pending: { text: "#e5a54a", bg: "rgba(229,165,74,0.1)", border: "rgba(229,165,74,0.3)" },
  approved: { text: "#16a34a", bg: "#dcfce7", border: "#86efac" },
  rejected: { text: "#ff5442", bg: "rgba(255,84,66,0.1)", border: "rgba(255,84,66,0.3)" },
  cancelled: { text: "#6b8db5", bg: "rgba(107,141,181,0.1)", border: "rgba(107,141,181,0.3)" },
};

function formatDateTime(iso: string): string {
  const d = new Date(iso);
  return `${d.getFullYear()}.${String(d.getMonth() + 1).padStart(2, "0")}.${String(d.getDate()).padStart(2, "0")} ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

interface BankAccountFormProps {
  initial: BankAccount | null;
  onSaved: () => void;
}

function BankAccountForm({ initial, onSaved }: BankAccountFormProps) {
  const [bankName, setBankName] = useState(initial?.bank_name ?? "");
  const [accountNumber, setAccountNumber] = useState(initial?.account_number ?? "");
  const [accountHolder, setAccountHolder] = useState(initial?.account_holder ?? "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      await api.saveBankAccount({
        bank_name: bankName.trim(),
        account_number: accountNumber.trim(),
        account_holder: accountHolder.trim(),
      });
      onSaved();
    } catch (e) {
      setError(e instanceof Error ? e.message : "저장 실패");
    } finally {
      setSaving(false);
    }
  };

  const isValid =
    bankName.trim().length > 0 &&
    accountNumber.trim().length > 0 &&
    accountHolder.trim().length > 0;

  return (
    <View style={formStyles.container}>
      <View style={formStyles.field}>
        <Text style={formStyles.label}>은행명</Text>
        <TextInput
          value={bankName}
          onChangeText={setBankName}
          placeholder="예: 카카오뱅크"
          placeholderTextColor="#96a0ad"
          style={formStyles.input}
        />
      </View>
      <View style={formStyles.field}>
        <Text style={formStyles.label}>계좌번호</Text>
        <TextInput
          value={accountNumber}
          onChangeText={setAccountNumber}
          placeholder="'-' 없이 입력"
          placeholderTextColor="#96a0ad"
          keyboardType="numeric"
          style={formStyles.input}
        />
      </View>
      <View style={formStyles.field}>
        <Text style={formStyles.label}>예금주</Text>
        <TextInput
          value={accountHolder}
          onChangeText={setAccountHolder}
          placeholder="홍길동"
          placeholderTextColor="#96a0ad"
          style={formStyles.input}
        />
      </View>
      {error && <Text style={formStyles.error}>{error}</Text>}
      <TouchableOpacity
        onPress={handleSave}
        disabled={saving || !isValid}
        style={[formStyles.saveButton, (saving || !isValid) && formStyles.saveButtonDisabled]}
      >
        <Text style={formStyles.saveButtonText}>
          {saving ? "저장 중..." : initial ? "계좌 수정" : "계좌 등록"}
        </Text>
      </TouchableOpacity>
    </View>
  );
}

const formStyles = StyleSheet.create({
  container: { gap: 10 },
  field: { gap: 4 },
  label: { fontSize: 12, color: "#6b8db5" },
  input: {
    backgroundColor: "#fff",
    borderWidth: 1,
    borderColor: "#d4e6f5",
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 10,
    fontSize: 13,
    color: "#1e3a5f",
  },
  error: { fontSize: 13, color: "#ff5442" },
  saveButton: {
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 8,
    backgroundColor: "#1e3a5f",
    alignSelf: "flex-start",
  },
  saveButtonDisabled: { opacity: 0.5 },
  saveButtonText: { fontSize: 13, fontWeight: "600", color: "#fff" },
});

// ── Screen ────────────────────────────────────────────────────────────────────

export function WithdrawalScreen() {
  const { user, loading: authLoading } = useAuth();
  const navigation = useNavigation<NativeStackNavigationProp<RootStackParamList>>();

  const [bankAccount, setBankAccount] = useState<BankAccount | null>(null);
  const [info, setInfo] = useState<WithdrawalInfo | null>(null);
  const [levelInfo, setLevelInfo] = useState<LevelInfo | null>(null);
  const [history, setHistory] = useState<WithdrawalDetail[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [amount, setAmount] = useState("");
  const [requestError, setRequestError] = useState<string | null>(null);
  const [requesting, setRequesting] = useState(false);
  const [cancellingId, setCancellingId] = useState<string | null>(null);

  const pendingAmount = history
    .filter((w) => w.current_status === "pending")
    .reduce((sum, w) => sum + w.amount, 0);

  const availableCoins = (levelInfo?.total_coins ?? 0) - pendingAmount;

  const loadAll = useCallback(async () => {
    setLoading(true);
    try {
      const [ba, wi, lv, hist] = await Promise.all([
        api.getBankAccount(),
        api.getWithdrawalInfo(),
        api.getUserLevel(),
        api.getWithdrawalHistory(20, 0),
      ]);
      setBankAccount(ba);
      setInfo(wi);
      setLevelInfo(lv);
      setHistory(hist.data);
      setHasMore(hist.has_more);
    } catch {
      // silently fail on initial load
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!authLoading && !user) {
      navigation.replace("Landing");
    }
  }, [user, authLoading, navigation]);

  useEffect(() => {
    if (user) loadAll();
  }, [user, loadAll]);

  const handleRequest = async () => {
    const numAmount = parseInt(amount, 10);
    if (!numAmount || numAmount <= 0) return;
    setRequesting(true);
    setRequestError(null);
    try {
      await api.requestWithdrawal(numAmount);
      setAmount("");
      await loadAll();
    } catch (e) {
      setRequestError(e instanceof Error ? e.message : "출금 신청 실패");
    } finally {
      setRequesting(false);
    }
  };

  const handleCancel = (id: string) => {
    Alert.alert("출금 취소", "출금 신청을 취소하시겠습니까? 포인트가 복구됩니다.", [
      { text: "아니오", style: "cancel" },
      {
        text: "취소하기",
        style: "destructive",
        onPress: async () => {
          setCancellingId(id);
          try {
            await api.cancelWithdrawal(id);
            await loadAll();
          } catch (e) {
            Alert.alert("오류", e instanceof Error ? e.message : "취소 실패");
          } finally {
            setCancellingId(null);
          }
        },
      },
    ]);
  };

  const loadMore = async () => {
    try {
      const { data, has_more } = await api.getWithdrawalHistory(20, history.length);
      setHistory((prev) => [...prev, ...data]);
      setHasMore(has_more);
    } catch {
      // silently fail
    }
  };

  if (authLoading || !user || loading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator color="#43b9d6" />
      </View>
    );
  }

  return (
    <ScrollView style={styles.screen} contentContainerStyle={styles.content}>
      {/* Balance info */}
      <View style={styles.balanceCard}>
        <View>
          <Text style={styles.balanceLabel}>보유 포인트</Text>
          <Text style={styles.balanceValue}>
            {(levelInfo?.total_coins ?? 0).toLocaleString()}
          </Text>
        </View>
        <View style={styles.balanceRight}>
          {pendingAmount > 0 && (
            <>
              <Text style={styles.pendingLabel}>출금 대기</Text>
              <Text style={styles.pendingValue}>-{pendingAmount.toLocaleString()}</Text>
            </>
          )}
          <Text style={styles.availableLabel}>출금 가능</Text>
          <Text style={styles.availableValue}>{availableCoins.toLocaleString()}</Text>
        </View>
      </View>

      {/* Bank account section */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>계좌 정보</Text>
        {bankAccount ? (
          <View style={styles.sectionBody}>
            <Text style={styles.bankInfo}>
              <Text style={styles.bankInfoLabel}>은행: </Text>
              {bankAccount.bank_name}
              {"  "}
              <Text style={styles.bankInfoLabel}>계좌: </Text>
              {bankAccount.account_number}
              {"  "}
              <Text style={styles.bankInfoLabel}>예금주: </Text>
              {bankAccount.account_holder}
            </Text>
            <BankAccountForm initial={bankAccount} onSaved={loadAll} />
          </View>
        ) : (
          <View style={styles.sectionBody}>
            <Text style={styles.sectionHint}>출금 받을 계좌를 먼저 등록해주세요.</Text>
            <BankAccountForm initial={null} onSaved={loadAll} />
          </View>
        )}
      </View>

      {/* Withdrawal request */}
      {bankAccount && (
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>출금 신청</Text>
          <View style={styles.sectionBody}>
            <Text style={styles.sectionHint}>
              최소 출금 금액: {(info?.min_withdrawal_amount ?? 0).toLocaleString()}원 · 출금 가능:{" "}
              {availableCoins.toLocaleString()}원
            </Text>
            <View style={styles.requestRow}>
              <TextInput
                value={amount}
                onChangeText={setAmount}
                placeholder="출금 금액"
                placeholderTextColor="#96a0ad"
                keyboardType="numeric"
                style={styles.amountInput}
              />
              <TouchableOpacity
                onPress={handleRequest}
                disabled={requesting || !amount || parseInt(amount) <= 0}
                style={[
                  styles.requestButton,
                  (requesting || !amount || parseInt(amount) <= 0) && styles.requestButtonDisabled,
                ]}
              >
                <Text style={styles.requestButtonText}>
                  {requesting ? "신청 중..." : "출금 신청"}
                </Text>
              </TouchableOpacity>
            </View>
            {requestError && <Text style={styles.errorText}>{requestError}</Text>}
          </View>
        </View>
      )}

      {/* Withdrawal history */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>출금 내역</Text>
        <View style={styles.sectionBody}>
          {history.length === 0 ? (
            <Text style={styles.sectionHint}>출금 내역이 없습니다.</Text>
          ) : (
            <>
              {history.map((w) => {
                const sc = STATUS_COLOR[w.current_status];
                return (
                  <View key={w.id} style={styles.historyCard}>
                    <View style={styles.historyCardRow}>
                      <View style={styles.historyCardLeft}>
                        <Text style={styles.historyAmount}>{w.amount.toLocaleString()}원</Text>
                        {sc && (
                          <View
                            style={[
                              styles.statusBadge,
                              { backgroundColor: sc.bg, borderColor: sc.border },
                            ]}
                          >
                            <Text style={[styles.statusBadgeText, { color: sc.text }]}>
                              {STATUS_LABEL[w.current_status]}
                            </Text>
                          </View>
                        )}
                      </View>
                      {w.current_status === "pending" && (
                        <TouchableOpacity
                          onPress={() => handleCancel(w.id)}
                          disabled={cancellingId === w.id}
                        >
                          <Text style={styles.cancelText}>
                            {cancellingId === w.id ? "취소 중..." : "취소"}
                          </Text>
                        </TouchableOpacity>
                      )}
                    </View>
                    <Text style={styles.historyMeta}>
                      {w.bank_name} {w.account_number} · {formatDateTime(w.created_at)}
                    </Text>
                    {w.transitions
                      ?.filter((t) => t.status === "rejected" && t.note)
                      .map((t) => (
                        <View key={t.id} style={styles.rejectNote}>
                          <Text style={styles.rejectNoteText}>거절 사유: {t.note}</Text>
                        </View>
                      ))}
                  </View>
                );
              })}
              {hasMore && (
                <TouchableOpacity onPress={loadMore} style={styles.loadMoreButton}>
                  <Text style={styles.loadMoreText}>더 보기</Text>
                </TouchableOpacity>
              )}
            </>
          )}
        </View>
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: "#f0f7ff",
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
  },
  balanceCard: {
    borderRadius: 16,
    backgroundColor: "#e8f4fd",
    borderWidth: 1,
    borderColor: "#d4e6f5",
    paddingHorizontal: 20,
    paddingVertical: 16,
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
  },
  balanceLabel: { fontSize: 11, color: "#6b8db5" },
  balanceValue: { fontSize: 22, fontWeight: "700", color: "#1e3a5f" },
  balanceRight: { alignItems: "flex-end" },
  pendingLabel: { fontSize: 11, color: "#e5a54a" },
  pendingValue: { fontSize: 16, fontWeight: "600", color: "#e5a54a" },
  availableLabel: { fontSize: 11, color: "#6b8db5", marginTop: 4 },
  availableValue: { fontSize: 16, fontWeight: "600", color: "#1e3a5f" },
  section: {
    borderRadius: 16,
    borderWidth: 1,
    borderColor: "#d4e6f5",
    backgroundColor: "#f0f7ff",
    padding: 16,
    gap: 12,
  },
  sectionTitle: {
    fontSize: 15,
    fontWeight: "600",
    color: "#1e3a5f",
  },
  sectionBody: { gap: 10 },
  sectionHint: { fontSize: 13, color: "#6b8db5" },
  bankInfo: { fontSize: 13, color: "#1e3a5f", marginBottom: 8 },
  bankInfoLabel: { color: "#6b8db5" },
  requestRow: {
    flexDirection: "row",
    gap: 8,
    alignItems: "center",
  },
  amountInput: {
    flex: 1,
    backgroundColor: "#fff",
    borderWidth: 1,
    borderColor: "#d4e6f5",
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 10,
    fontSize: 13,
    color: "#1e3a5f",
  },
  requestButton: {
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 8,
    backgroundColor: "#1e3a5f",
  },
  requestButtonDisabled: { opacity: 0.5 },
  requestButtonText: { fontSize: 13, fontWeight: "600", color: "#fff" },
  errorText: { fontSize: 13, color: "#ff5442" },
  historyCard: {
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "#d4e6f5",
    backgroundColor: "#fff",
    padding: 14,
    gap: 6,
  },
  historyCardRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  historyCardLeft: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  historyAmount: {
    fontSize: 16,
    fontWeight: "700",
    color: "#1e3a5f",
  },
  statusBadge: {
    paddingHorizontal: 8,
    paddingVertical: 2,
    borderRadius: 999,
    borderWidth: 1,
  },
  statusBadgeText: {
    fontSize: 11,
    fontWeight: "600",
  },
  cancelText: {
    fontSize: 12,
    color: "#ff5442",
  },
  historyMeta: {
    fontSize: 11,
    color: "#6b8db5",
  },
  rejectNote: {
    backgroundColor: "rgba(255,84,66,0.05)",
    borderRadius: 8,
    paddingHorizontal: 10,
    paddingVertical: 6,
  },
  rejectNoteText: {
    fontSize: 11,
    color: "#ff5442",
  },
  loadMoreButton: {
    paddingVertical: 10,
    alignItems: "center",
  },
  loadMoreText: {
    fontSize: 13,
    color: "#6b8db5",
  },
});
