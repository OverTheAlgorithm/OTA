// Ported from: web/src/pages/admin-terms.tsx
import React, { useCallback, useEffect, useState } from "react";
import {
  View,
  Text,
  ScrollView,
  Pressable,
  TextInput,
  ActivityIndicator,
  Switch,
  StyleSheet,
} from "react-native";
import { useNavigation } from "@react-navigation/native";
import { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { useAuth } from "../contexts/auth-context";
import { api } from "../lib/api";
import type { Term } from "../../../packages/shared/src/types";

type NavProp = NativeStackNavigationProp<Record<string, object | undefined>>;

// --- EditTermForm ---

function EditTermForm({
  term,
  onUpdated,
  onError,
}: {
  term: Term;
  onUpdated: (updated: Term) => void;
  onError: (msg: string) => void;
}) {
  const [form, setForm] = useState({
    url: term.url,
    description: term.description,
    required: term.required,
  });
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    setSubmitting(true);
    try {
      const updated = await api.updateTerm(term.id, form);
      onUpdated(updated);
    } catch (e) {
      onError(e instanceof Error ? e.message : "수정 실패");
      setSubmitting(false);
    }
  };

  return (
    <View style={editStyles.container}>
      <Text style={editStyles.hint}>제목·버전은 동의 기록 식별자라 변경 불가합니다.</Text>
      <TextInput
        value={form.url}
        onChangeText={(v) => setForm({ ...form, url: v })}
        placeholder="약관 전문 URL (선택)"
        style={editStyles.input}
        autoCapitalize="none"
        keyboardType="url"
      />
      <TextInput
        value={form.description}
        onChangeText={(v) => setForm({ ...form, description: v })}
        placeholder="설명 (선택)"
        style={[editStyles.input, { minHeight: 60 }]}
        multiline
      />
      <View style={editStyles.row}>
        <View style={editStyles.switchRow}>
          <Switch
            value={form.required}
            onValueChange={(v) => setForm({ ...form, required: v })}
            trackColor={{ true: "#4a9fe5", false: "#d4e6f5" }}
            thumbColor="#fff"
          />
          <Text style={editStyles.switchLabel}>필수</Text>
        </View>
        <Pressable
          style={[editStyles.saveBtn, submitting && editStyles.btnDisabled]}
          onPress={handleSubmit}
          disabled={submitting}
        >
          <Text style={editStyles.saveBtnText}>{submitting ? "저장 중..." : "저장"}</Text>
        </Pressable>
      </View>
    </View>
  );
}

const editStyles = StyleSheet.create({
  container: {
    borderTopWidth: 1,
    borderTopColor: "#d4e6f5",
    paddingTop: 12,
    gap: 8,
    marginTop: 4,
  },
  hint: {
    fontSize: 10,
    color: "#94a3b8",
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
  row: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  switchRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  switchLabel: {
    fontSize: 13,
    color: "#1e3a5f",
  },
  saveBtn: {
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 8,
    backgroundColor: "rgba(74,159,229,0.2)",
  },
  saveBtnText: {
    fontSize: 13,
    fontWeight: "600",
    color: "#4a9fe5",
  },
  btnDisabled: {
    opacity: 0.5,
  },
});

// --- CreateTermForm ---

function CreateTermForm({ onCreated }: { onCreated: () => void }) {
  const [form, setForm] = useState({
    title: "",
    description: "",
    url: "",
    version: "",
    active: true,
    required: false,
  });
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (!form.title || !form.version) {
      setError("제목, 버전은 필수입니다");
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      await api.createTerm(form);
      onCreated();
    } catch (e) {
      setError(e instanceof Error ? e.message : "생성 실패");
      setSubmitting(false);
    }
  };

  return (
    <View style={createStyles.container}>
      <Text style={createStyles.title}>새 약관 만들기</Text>

      {error && <Text style={createStyles.errorText}>{error}</Text>}

      <TextInput
        value={form.title}
        onChangeText={(v) => setForm({ ...form, title: v })}
        placeholder="제목 (예: 개인정보 처리방침)"
        style={createStyles.input}
      />
      <TextInput
        value={form.description}
        onChangeText={(v) => setForm({ ...form, description: v })}
        placeholder="설명 (선택)"
        style={[createStyles.input, { minHeight: 60 }]}
        multiline
      />
      <TextInput
        value={form.url}
        onChangeText={(v) => setForm({ ...form, url: v })}
        placeholder="약관 전문 URL (선택)"
        style={createStyles.input}
        autoCapitalize="none"
        keyboardType="url"
      />
      <TextInput
        value={form.version}
        onChangeText={(v) => setForm({ ...form, version: v })}
        placeholder="버전 (예: 1 또는 1.2)"
        style={createStyles.input}
        keyboardType="decimal-pad"
      />

      <View style={createStyles.checkRow}>
        <View style={createStyles.switchRow}>
          <Switch
            value={form.active}
            onValueChange={(v) => setForm({ ...form, active: v })}
            trackColor={{ true: "#4a9fe5", false: "#d4e6f5" }}
            thumbColor="#fff"
          />
          <Text style={createStyles.switchLabel}>활성</Text>
        </View>
        <View style={createStyles.switchRow}>
          <Switch
            value={form.required}
            onValueChange={(v) => setForm({ ...form, required: v })}
            trackColor={{ true: "#4a9fe5", false: "#d4e6f5" }}
            thumbColor="#fff"
          />
          <Text style={createStyles.switchLabel}>필수</Text>
        </View>
      </View>

      <Pressable
        style={[createStyles.submitBtn, submitting && createStyles.btnDisabled]}
        onPress={handleSubmit}
        disabled={submitting}
      >
        <Text style={createStyles.submitBtnText}>{submitting ? "생성 중..." : "생성"}</Text>
      </Pressable>
    </View>
  );
}

const createStyles = StyleSheet.create({
  container: {
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "rgba(74,159,229,0.3)",
    backgroundColor: "#fff",
    padding: 16,
    gap: 10,
  },
  title: {
    fontSize: 14,
    fontWeight: "600",
    color: "#1e3a5f",
  },
  errorText: {
    fontSize: 12,
    color: "#ff5442",
  },
  input: {
    backgroundColor: "#f0f7ff",
    borderWidth: 1,
    borderColor: "#d4e6f5",
    borderRadius: 10,
    paddingHorizontal: 12,
    paddingVertical: 9,
    fontSize: 13,
    color: "#1e3a5f",
  },
  checkRow: {
    flexDirection: "row",
    gap: 24,
  },
  switchRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  switchLabel: {
    fontSize: 13,
    color: "#1e3a5f",
  },
  submitBtn: {
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 8,
    backgroundColor: "rgba(74,159,229,0.2)",
    alignSelf: "flex-start",
  },
  submitBtnText: {
    fontSize: 13,
    fontWeight: "600",
    color: "#4a9fe5",
  },
  btnDisabled: {
    opacity: 0.5,
  },
});

// --- AdminTermsScreen ---

export function AdminTermsScreen() {
  const { user, loading: authLoading } = useAuth();
  const navigation = useNavigation<NavProp>();

  const [terms, setTerms] = useState<Term[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);

  const loadTerms = useCallback(() => {
    setLoading(true);
    setError(null);
    api
      .getAdminTerms()
      .then(setTerms)
      .catch((e) => setError(e instanceof Error ? e.message : "불러오기 실패"))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (!authLoading && user?.role === "admin") {
      loadTerms();
    }
  }, [user, authLoading, loadTerms]);

  const handleToggleActive = async (term: Term) => {
    setTogglingId(term.id);
    try {
      await api.updateTermActive(term.id, !term.active);
      setTerms((prev) =>
        prev.map((t) => (t.id === term.id ? { ...t, active: !t.active } : t))
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "상태 변경 실패");
    } finally {
      setTogglingId(null);
    }
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

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {error && (
        <View style={styles.errorBox}>
          <Text style={styles.errorText}>{error}</Text>
        </View>
      )}

      {/* Create toggle */}
      <View style={styles.createToggleRow}>
        <Pressable
          style={[styles.createToggleBtn, showCreate && styles.createToggleBtnAlt]}
          onPress={() => setShowCreate(!showCreate)}
        >
          <Text style={styles.createToggleBtnText}>
            {showCreate ? "취소" : "새 약관 만들기"}
          </Text>
        </Pressable>
      </View>

      {showCreate && (
        <CreateTermForm
          onCreated={() => {
            setShowCreate(false);
            loadTerms();
          }}
        />
      )}

      {/* Terms list */}
      {loading ? (
        <ActivityIndicator color="#4a9fe5" style={{ marginTop: 32 }} />
      ) : terms.length === 0 ? (
        <Text style={styles.emptyText}>등록된 약관이 없습니다.</Text>
      ) : (
        terms.map((t) => (
          <View key={t.id} style={styles.termCard}>
            <View style={styles.termTop}>
              <View style={{ flex: 1 }}>
                <View style={styles.termTitleRow}>
                  <Text style={styles.termTitle}>{t.title}</Text>
                  <Text style={styles.termVersion}>v{t.version}</Text>
                </View>
                {t.description ? (
                  <Text style={styles.termDesc}>{t.description}</Text>
                ) : null}
                <View style={styles.termMeta}>
                  {t.url ? (
                    <Text style={styles.termUrl} numberOfLines={1}>
                      {t.url}
                    </Text>
                  ) : null}
                  <Text style={styles.termDate}>
                    {new Date(t.created_at).toLocaleDateString("ko-KR")}
                  </Text>
                </View>
                <View style={styles.termBadges}>
                  {/* Active toggle badge */}
                  <Pressable
                    onPress={() => handleToggleActive(t)}
                    disabled={togglingId === t.id}
                    style={[
                      styles.activeBadge,
                      t.active ? styles.activeBadgeOn : styles.activeBadgeOff,
                      togglingId === t.id && { opacity: 0.5 },
                    ]}
                  >
                    <Text
                      style={[
                        styles.activeBadgeText,
                        { color: t.active ? "#15803d" : "#6b7280" },
                      ]}
                    >
                      {t.active ? "활성" : "비활성"}
                    </Text>
                  </Pressable>
                  {/* Required badge */}
                  <View
                    style={[
                      styles.requiredBadge,
                      t.required ? styles.requiredBadgeOn : styles.requiredBadgeOff,
                    ]}
                  >
                    <Text
                      style={[
                        styles.requiredBadgeText,
                        { color: t.required ? "#ff5442" : "#6b8db5" },
                      ]}
                    >
                      {t.required ? "필수" : "선택"}
                    </Text>
                  </View>
                </View>
              </View>
              <Pressable
                onPress={() => setEditingId(editingId === t.id ? null : t.id)}
                style={styles.editBtn}
              >
                <Text style={styles.editBtnText}>{editingId === t.id ? "취소" : "수정"}</Text>
              </Pressable>
            </View>

            {editingId === t.id && (
              <EditTermForm
                term={t}
                onUpdated={(updated) => {
                  setTerms((prev) => prev.map((x) => (x.id === updated.id ? updated : x)));
                  setEditingId(null);
                }}
                onError={setError}
              />
            )}
          </View>
        ))
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
    gap: 12,
  },
  centered: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "#f8fafc",
    gap: 16,
  },
  errorBox: {
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "rgba(255,84,66,0.3)",
    backgroundColor: "rgba(255,84,66,0.1)",
    padding: 12,
  },
  errorText: {
    fontSize: 13,
    color: "#ff5442",
  },
  createToggleRow: {
    alignItems: "flex-end",
  },
  createToggleBtn: {
    paddingHorizontal: 16,
    paddingVertical: 9,
    borderRadius: 12,
    backgroundColor: "#4a9fe5",
  },
  createToggleBtnAlt: {
    backgroundColor: "#d4e6f5",
  },
  createToggleBtnText: {
    fontSize: 13,
    fontWeight: "600",
    color: "#fff",
  },
  emptyText: {
    fontSize: 13,
    color: "#6b8db5",
    textAlign: "center",
    marginTop: 32,
  },
  termCard: {
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "#d4e6f5",
    backgroundColor: "#f0f7ff",
    padding: 14,
    gap: 8,
  },
  termTop: {
    flexDirection: "row",
    alignItems: "flex-start",
    gap: 8,
  },
  termTitleRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    flexWrap: "wrap",
  },
  termTitle: {
    fontSize: 14,
    fontWeight: "600",
    color: "#1e3a5f",
  },
  termVersion: {
    fontSize: 11,
    color: "#94a3b8",
  },
  termDesc: {
    fontSize: 12,
    color: "#6b8db5",
    marginTop: 3,
  },
  termMeta: {
    flexDirection: "row",
    gap: 10,
    marginTop: 4,
    alignItems: "center",
  },
  termUrl: {
    fontSize: 11,
    color: "#4a9fe5",
    flex: 1,
  },
  termDate: {
    fontSize: 11,
    color: "#94a3b8",
  },
  termBadges: {
    flexDirection: "row",
    gap: 6,
    marginTop: 6,
  },
  activeBadge: {
    paddingHorizontal: 8,
    paddingVertical: 3,
    borderRadius: 50,
  },
  activeBadgeOn: {
    backgroundColor: "#dcfce7",
  },
  activeBadgeOff: {
    backgroundColor: "#f3f4f6",
  },
  activeBadgeText: {
    fontSize: 10,
    fontWeight: "600",
  },
  requiredBadge: {
    paddingHorizontal: 8,
    paddingVertical: 3,
    borderRadius: 50,
  },
  requiredBadgeOn: {
    backgroundColor: "rgba(255,84,66,0.1)",
  },
  requiredBadgeOff: {
    backgroundColor: "#d4e6f5",
  },
  requiredBadgeText: {
    fontSize: 10,
    fontWeight: "600",
  },
  editBtn: {
    paddingHorizontal: 2,
  },
  editBtnText: {
    fontSize: 12,
    color: "#6b8db5",
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
