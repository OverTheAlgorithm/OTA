// Ported from: web/src/pages/admin.tsx
import React, { useCallback, useEffect, useState } from "react";
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
import type { BrainCategory, TestEmailResult } from "../../../packages/shared/src/types";

type NavProp = NativeStackNavigationProp<Record<string, object | undefined>>;

type CollectState =
  | { status: "idle" }
  | { status: "running" }
  | { status: "requested" }
  | { status: "error"; message: string };

type TestEmailState =
  | { status: "idle" }
  | { status: "sending" }
  | { status: "done"; result: TestEmailResult }
  | { status: "error"; message: string };

// --- BrainCategoryManager ---

function BrainCategoryManager() {
  const [categories, setCategories] = useState<BrainCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editForm, setEditForm] = useState({
    emoji: "",
    label: "",
    accent_color: "",
    display_order: 0,
    instruction: "",
  });
  const [showNew, setShowNew] = useState(false);
  const [newForm, setNewForm] = useState({
    key: "",
    emoji: "",
    label: "",
    accent_color: "#6b8db5",
    display_order: 0,
    instruction: "",
  });
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    setLoading(true);
    api
      .getBrainCategories()
      .then(setCategories)
      .catch(() => setError("불러오기 실패"))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const startEdit = (bc: BrainCategory) => {
    setEditingKey(bc.key);
    setEditForm({
      emoji: bc.emoji,
      label: bc.label,
      accent_color: bc.accent_color,
      display_order: bc.display_order,
      instruction: bc.instruction ?? "",
    });
  };

  const saveEdit = async () => {
    if (!editingKey) return;
    setError(null);
    try {
      await api.updateBrainCategory(editingKey, {
        ...editForm,
        instruction: editForm.instruction.trim() || null,
      });
      setEditingKey(null);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "수정 실패");
    }
  };

  const handleDelete = (key: string) => {
    Alert.alert("삭제 확인", `"${key}" 카테고리를 삭제하시겠습니까?`, [
      { text: "취소", style: "cancel" },
      {
        text: "삭제",
        style: "destructive",
        onPress: async () => {
          setError(null);
          try {
            await api.deleteBrainCategory(key);
            load();
          } catch (e) {
            setError(e instanceof Error ? e.message : "삭제 실패");
          }
        },
      },
    ]);
  };

  const handleCreate = async () => {
    if (!newForm.key || !newForm.emoji || !newForm.label) {
      setError("key, emoji, label은 필수입니다");
      return;
    }
    setError(null);
    try {
      await api.createBrainCategory({
        ...newForm,
        instruction: newForm.instruction.trim() || null,
      });
      setShowNew(false);
      setNewForm({
        key: "",
        emoji: "",
        label: "",
        accent_color: "#6b8db5",
        display_order: 0,
        instruction: "",
      });
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "생성 실패");
    }
  };

  if (loading) {
    return <ActivityIndicator color="#4a9fe5" style={{ marginVertical: 8 }} />;
  }

  return (
    <View>
      {error && <Text style={styles.errorText}>{error}</Text>}

      {categories.map((bc) => (
        <View key={bc.key} style={styles.bcCard}>
          {editingKey === bc.key ? (
            <View style={{ gap: 8 }}>
              <View style={styles.row}>
                <TextInput
                  value={editForm.emoji}
                  onChangeText={(v) => setEditForm({ ...editForm, emoji: v })}
                  style={[styles.input, { width: 50 }]}
                  placeholder="emoji"
                />
                <TextInput
                  value={editForm.label}
                  onChangeText={(v) => setEditForm({ ...editForm, label: v })}
                  style={[styles.input, { flex: 1 }]}
                  placeholder="label"
                />
              </View>
              <View style={styles.row}>
                <TextInput
                  value={editForm.accent_color}
                  onChangeText={(v) => setEditForm({ ...editForm, accent_color: v })}
                  style={[styles.input, { width: 90 }]}
                  placeholder="color"
                />
                <TextInput
                  value={String(editForm.display_order)}
                  onChangeText={(v) =>
                    setEditForm({ ...editForm, display_order: parseInt(v) || 0 })
                  }
                  style={[styles.input, { width: 60 }]}
                  placeholder="순서"
                  keyboardType="number-pad"
                />
                <Pressable style={styles.btnSmall} onPress={saveEdit}>
                  <Text style={styles.btnSmallText}>저장</Text>
                </Pressable>
                <Pressable
                  style={[styles.btnSmall, styles.btnSmallSecondary]}
                  onPress={() => setEditingKey(null)}
                >
                  <Text style={styles.btnSmallSecondaryText}>취소</Text>
                </Pressable>
              </View>
              <TextInput
                value={editForm.instruction}
                onChangeText={(v) => setEditForm({ ...editForm, instruction: v })}
                style={[styles.input, { minHeight: 60 }]}
                placeholder="AI 지시사항 (선택)"
                multiline
              />
            </View>
          ) : (
            <View style={styles.bcRow}>
              <View style={{ flex: 1 }}>
                <Text style={styles.bcLabel}>
                  {bc.emoji} {bc.label}
                </Text>
                <Text style={styles.bcMeta}>
                  key: {bc.key} · 순서: {bc.display_order}
                </Text>
                {bc.instruction ? (
                  <Text style={styles.bcInstruction} numberOfLines={2}>
                    지시: {bc.instruction}
                  </Text>
                ) : null}
              </View>
              <View style={styles.bcActions}>
                <Pressable onPress={() => startEdit(bc)}>
                  <Text style={styles.linkText}>수정</Text>
                </Pressable>
                <Pressable onPress={() => handleDelete(bc.key)}>
                  <Text style={[styles.linkText, { color: "#ff5442" }]}>삭제</Text>
                </Pressable>
              </View>
            </View>
          )}
        </View>
      ))}

      {showNew ? (
        <View style={styles.bcNewCard}>
          <View style={styles.row}>
            <TextInput
              value={newForm.key}
              onChangeText={(v) => setNewForm({ ...newForm, key: v })}
              style={[styles.input, { width: 90 }]}
              placeholder="key"
              autoCapitalize="none"
            />
            <TextInput
              value={newForm.emoji}
              onChangeText={(v) => setNewForm({ ...newForm, emoji: v })}
              style={[styles.input, { width: 50 }]}
              placeholder="emoji"
            />
            <TextInput
              value={newForm.label}
              onChangeText={(v) => setNewForm({ ...newForm, label: v })}
              style={[styles.input, { flex: 1 }]}
              placeholder="label"
            />
          </View>
          <View style={styles.row}>
            <TextInput
              value={newForm.accent_color}
              onChangeText={(v) => setNewForm({ ...newForm, accent_color: v })}
              style={[styles.input, { width: 90 }]}
              placeholder="color"
            />
            <TextInput
              value={String(newForm.display_order)}
              onChangeText={(v) =>
                setNewForm({ ...newForm, display_order: parseInt(v) || 0 })
              }
              style={[styles.input, { width: 60 }]}
              placeholder="순서"
              keyboardType="number-pad"
            />
            <Pressable style={styles.btnSmall} onPress={handleCreate}>
              <Text style={styles.btnSmallText}>추가</Text>
            </Pressable>
            <Pressable
              style={[styles.btnSmall, styles.btnSmallSecondary]}
              onPress={() => setShowNew(false)}
            >
              <Text style={styles.btnSmallSecondaryText}>취소</Text>
            </Pressable>
          </View>
          <TextInput
            value={newForm.instruction}
            onChangeText={(v) => setNewForm({ ...newForm, instruction: v })}
            style={[styles.input, { minHeight: 60 }]}
            placeholder="AI 지시사항 (선택)"
            multiline
          />
        </View>
      ) : (
        <Pressable style={styles.addDashedBtn} onPress={() => setShowNew(true)}>
          <Text style={styles.addDashedBtnText}>+ 새 카테고리 추가</Text>
        </Pressable>
      )}
    </View>
  );
}

// --- AdminScreen ---

export function AdminScreen() {
  const { user, loading } = useAuth();
  const navigation = useNavigation<NavProp>();

  const [collectState, setCollectState] = useState<CollectState>({ status: "idle" });
  const [testEmailState, setTestEmailState] = useState<TestEmailState>({ status: "idle" });

  const handleCollect = async () => {
    setCollectState({ status: "running" });
    try {
      await api.triggerCollection();
      setCollectState({ status: "requested" });
    } catch (e) {
      setCollectState({
        status: "error",
        message: e instanceof Error ? e.message : "알 수 없는 오류",
      });
    }
  };

  const handleTestEmail = async () => {
    setTestEmailState({ status: "sending" });
    try {
      const result = await api.sendTestEmail();
      setTestEmailState({ status: "done", result });
    } catch (e) {
      setTestEmailState({
        status: "error",
        message: e instanceof Error ? e.message : "알 수 없는 오류",
      });
    }
  };

  if (loading) {
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

  const collectDisabled =
    collectState.status === "running" || collectState.status === "requested";

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <View style={styles.titleRow}>
        <Text style={styles.pageTitle}>관리자 페이지</Text>
        <Pressable onPress={() => navigation.navigate("Latest" as never)}>
          <Text style={styles.navLink}>← 홈으로</Text>
        </Pressable>
      </View>

      {/* Points */}
      <View style={[styles.section, styles.sectionDanger]}>
        <Text style={[styles.sectionTitle, { color: "#ff5442" }]}>포인트 수정</Text>
        <Text style={styles.sectionDesc}>
          유저의 포인트 보유량을 직접 수정합니다. 장애 보상, 오류 수정 등 최후의 수단으로만
          사용하세요.
        </Text>
        <Pressable
          style={[styles.sectionBtn, { backgroundColor: "#ff5442" }]}
          onPress={() => navigation.navigate("AdminCoins" as never)}
        >
          <Text style={styles.sectionBtnText}>포인트 수정</Text>
        </Pressable>
      </View>

      {/* Withdrawals */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>출금 관리</Text>
        <Text style={styles.sectionDesc}>유저의 출금 신청을 확인하고 승인/거절할 수 있습니다.</Text>
        <Pressable
          style={styles.sectionBtn}
          onPress={() => navigation.navigate("AdminWithdrawals" as never)}
        >
          <Text style={styles.sectionBtnText}>출금 관리</Text>
        </Pressable>
      </View>

      {/* Terms */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>이용 약관 관리</Text>
        <Text style={styles.sectionDesc}>
          서비스 이용 약관을 관리하고 새 약관을 추가할 수 있습니다.
        </Text>
        <Pressable
          style={styles.sectionBtn}
          onPress={() => navigation.navigate("AdminTerms" as never)}
        >
          <Text style={styles.sectionBtnText}>이용 약관 관리</Text>
        </Pressable>
      </View>

      {/* Collection */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>데이터 수집</Text>
        <Text style={styles.sectionDesc}>
          AI를 통해 오늘의 한국 트렌드를 즉시 수집합니다. 이 작업은 1시간까지 소요될 수 있습니다.
        </Text>
        <Pressable
          style={[styles.sectionBtn, collectDisabled && styles.btnDisabled]}
          onPress={handleCollect}
          disabled={collectDisabled}
        >
          <Text style={styles.sectionBtnText}>
            {collectState.status === "running" && "수집 중..."}
            {collectState.status === "requested" && "수집 요청 완료"}
            {(collectState.status === "idle" || collectState.status === "error") && "수집 실행"}
          </Text>
        </Pressable>
        {collectState.status === "requested" && (
          <Text style={styles.hintText}>작업 완료시 슬랙으로 통지합니다.</Text>
        )}
        {collectState.status === "error" && (
          <View style={styles.errorBox}>
            <Text style={styles.errorTitle}>수집 실패</Text>
            <Text style={styles.errorMsg}>{collectState.message}</Text>
          </View>
        )}
      </View>

      {/* Test Email */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>테스트 이메일</Text>
        <Text style={styles.sectionDesc}>
          최신 브리핑을 내 이메일로 즉시 전송합니다. 이메일 채널이 활성화되어 있어야 합니다.
        </Text>
        <Pressable
          style={[styles.sectionBtn, testEmailState.status === "sending" && styles.btnDisabled]}
          onPress={handleTestEmail}
          disabled={testEmailState.status === "sending"}
        >
          <Text style={styles.sectionBtnText}>
            {testEmailState.status === "sending" ? "전송 중..." : "테스트 이메일 전송하기"}
          </Text>
        </Pressable>
        {testEmailState.status === "done" && (
          <View style={styles.successBox}>
            {testEmailState.result.success_count > 0 && (
              <Text style={styles.successText}>✓ 이메일이 전송됐습니다</Text>
            )}
            {testEmailState.result.skipped_count > 0 && (
              <Text style={styles.hintText}>이미 전송된 브리핑입니다 (중복 방지)</Text>
            )}
            {testEmailState.result.failure_count > 0 && (
              <Text style={styles.errorText}>
                전송 실패 — {Object.values(testEmailState.result.errors).join(", ")}
              </Text>
            )}
          </View>
        )}
        {testEmailState.status === "error" && (
          <View style={styles.errorBox}>
            <Text style={styles.errorTitle}>전송 실패</Text>
            <Text style={styles.errorMsg}>{testEmailState.message}</Text>
          </View>
        )}
      </View>

      {/* Push Notifications */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>푸시 알림 관리</Text>
        <Text style={styles.sectionDesc}>
          푸시 알림을 생성하고 예약 전송하거나 즉시 전송할 수 있습니다.
        </Text>
        <Pressable
          style={styles.sectionBtn}
          onPress={() => navigation.navigate("AdminPush" as never)}
        >
          <Text style={styles.sectionBtnText}>푸시 관리</Text>
        </Pressable>
      </View>

      {/* Brain Categories */}
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Brain Category 관리</Text>
        <Text style={styles.sectionDesc}>
          각 토픽에 부여되는 행동 지침 라벨입니다. AI가 수집 시 이 목록에서 선택합니다.
        </Text>
        <BrainCategoryManager />
      </View>
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
  titleRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    marginBottom: 8,
  },
  pageTitle: {
    fontSize: 22,
    fontWeight: "700",
    color: "#1e3a5f",
  },
  navLink: {
    fontSize: 13,
    color: "#6b8db5",
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
    borderColor: "rgba(255,84,66,0.2)",
    backgroundColor: "rgba(255,84,66,0.05)",
  },
  sectionTitle: {
    fontSize: 16,
    fontWeight: "600",
    color: "#1e3a5f",
  },
  sectionDesc: {
    fontSize: 13,
    color: "#6b8db5",
    lineHeight: 18,
  },
  sectionBtn: {
    backgroundColor: "#4a9fe5",
    borderRadius: 12,
    paddingVertical: 12,
    alignItems: "center",
  },
  sectionBtnText: {
    color: "#fff",
    fontWeight: "600",
    fontSize: 14,
  },
  btnDisabled: {
    opacity: 0.5,
  },
  hintText: {
    fontSize: 12,
    color: "#6b8db5",
  },
  errorBox: {
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "rgba(255,84,66,0.3)",
    backgroundColor: "rgba(255,84,66,0.1)",
    padding: 12,
    gap: 4,
  },
  errorTitle: {
    fontSize: 13,
    fontWeight: "600",
    color: "#ff5442",
  },
  errorMsg: {
    fontSize: 12,
    color: "#6b8db5",
  },
  errorText: {
    fontSize: 13,
    color: "#ff5442",
  },
  successBox: {
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "#86efac",
    backgroundColor: "#dcfce7",
    padding: 12,
    gap: 4,
  },
  successText: {
    fontSize: 13,
    fontWeight: "600",
    color: "#15803d",
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
  // BrainCategoryManager
  bcCard: {
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "#d4e6f5",
    backgroundColor: "#f0f7ff",
    padding: 12,
    marginBottom: 8,
  },
  bcNewCard: {
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "rgba(74,159,229,0.3)",
    backgroundColor: "#fff",
    padding: 12,
    marginBottom: 8,
    gap: 8,
  },
  bcRow: {
    flexDirection: "row",
    alignItems: "flex-start",
    gap: 8,
  },
  bcLabel: {
    fontSize: 14,
    fontWeight: "600",
    color: "#1e3a5f",
  },
  bcMeta: {
    fontSize: 11,
    color: "#6b8db5",
    marginTop: 2,
  },
  bcInstruction: {
    fontSize: 11,
    color: "#4a9fe5",
    marginTop: 2,
  },
  bcActions: {
    flexDirection: "row",
    gap: 12,
  },
  linkText: {
    fontSize: 13,
    color: "#6b8db5",
  },
  row: {
    flexDirection: "row",
    gap: 6,
    alignItems: "center",
  },
  input: {
    backgroundColor: "#fff",
    borderWidth: 1,
    borderColor: "#d4e6f5",
    borderRadius: 8,
    paddingHorizontal: 10,
    paddingVertical: 7,
    fontSize: 13,
    color: "#1e3a5f",
  },
  btnSmall: {
    paddingHorizontal: 12,
    paddingVertical: 7,
    borderRadius: 8,
    backgroundColor: "rgba(74,159,229,0.2)",
  },
  btnSmallText: {
    fontSize: 12,
    fontWeight: "600",
    color: "#4a9fe5",
  },
  btnSmallSecondary: {
    backgroundColor: "#d4e6f5",
  },
  btnSmallSecondaryText: {
    fontSize: 12,
    color: "#6b8db5",
  },
  addDashedBtn: {
    borderWidth: 1,
    borderStyle: "dashed",
    borderColor: "#d4e6f5",
    borderRadius: 12,
    paddingVertical: 12,
    alignItems: "center",
    marginTop: 4,
  },
  addDashedBtnText: {
    fontSize: 13,
    color: "#6b8db5",
  },
});
