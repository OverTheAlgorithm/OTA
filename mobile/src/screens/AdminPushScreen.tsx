// New feature — push notification admin management
// No web source; this IS the source of truth (mobile-first admin feature)
import React, { useCallback, useEffect, useState } from "react";
import {
  View,
  Text,
  FlatList,
  Pressable,
  TextInput,
  ActivityIndicator,
  Alert,
  Modal,
  ScrollView,
  StyleSheet,
  Platform,
} from "react-native";
import { useNavigation } from "@react-navigation/native";
import { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { useAuth } from "../contexts/auth-context";
import { api } from "../lib/api";
import type {
  ScheduledPush,
  CreateScheduledPushRequest,
  UpdateScheduledPushRequest,
} from "../../../packages/shared/src/push-admin";

type NavProp = NativeStackNavigationProp<Record<string, object | undefined>>;

const STATUS_LABELS: Record<ScheduledPush["status"], string> = {
  pending: "대기중",
  sent: "전송됨",
  failed: "실패",
  cancelled: "취소됨",
};

type StatusBadgeStyle = {
  bg: string;
  text: string;
};

const STATUS_BADGE: Record<ScheduledPush["status"], StatusBadgeStyle> = {
  pending: { bg: "#fef9c3", text: "#854d0e" },
  sent: { bg: "#dcfce7", text: "#15803d" },
  failed: { bg: "#fee2e2", text: "#b91c1c" },
  cancelled: { bg: "#f3f4f6", text: "#6b7280" },
};

type FormState = {
  title: string;
  body: string;
  link: string;
  scheduled_at: string; // ISO string or empty
};

const emptyForm: FormState = { title: "", body: "", link: "", scheduled_at: "" };

// --- PushForm Modal ---

function PushFormModal({
  visible,
  initial,
  onSubmit,
  onCancel,
  submitting,
  error,
  title: modalTitle,
}: {
  visible: boolean;
  initial: FormState;
  onSubmit: (form: FormState) => void;
  onCancel: () => void;
  submitting: boolean;
  error: string | null;
  title: string;
}) {
  const [form, setForm] = useState<FormState>(initial);

  useEffect(() => {
    if (visible) setForm(initial);
  }, [visible, initial]);

  return (
    <Modal visible={visible} animationType="slide" transparent onRequestClose={onCancel}>
      <View style={modalStyles.overlay}>
        <View style={modalStyles.sheet}>
          <ScrollView contentContainerStyle={modalStyles.content} keyboardShouldPersistTaps="handled">
            <Text style={modalStyles.title}>{modalTitle}</Text>

            {error ? <Text style={modalStyles.error}>{error}</Text> : null}

            <Text style={modalStyles.label}>제목 (필수)</Text>
            <TextInput
              value={form.title}
              onChangeText={(v) => setForm({ ...form, title: v })}
              placeholder="제목 (최대 100자)"
              maxLength={100}
              style={modalStyles.input}
            />

            <Text style={modalStyles.label}>본문 (필수)</Text>
            <TextInput
              value={form.body}
              onChangeText={(v) => setForm({ ...form, body: v })}
              placeholder="본문 (최대 500자)"
              maxLength={500}
              multiline
              style={[modalStyles.input, { minHeight: 80 }]}
            />

            <Text style={modalStyles.label}>링크 URL (선택)</Text>
            <TextInput
              value={form.link}
              onChangeText={(v) => setForm({ ...form, link: v })}
              placeholder="https://..."
              autoCapitalize="none"
              keyboardType="url"
              style={modalStyles.input}
            />

            <Text style={modalStyles.label}>예약 전송 시각 (선택, ISO 형식)</Text>
            <TextInput
              value={form.scheduled_at}
              onChangeText={(v) => setForm({ ...form, scheduled_at: v })}
              placeholder="2026-04-08T07:00:00+09:00"
              autoCapitalize="none"
              style={modalStyles.input}
            />
            <Text style={modalStyles.hint}>미입력시 예약 없이 즉시 전송 버튼으로만 전송 가능</Text>

            <View style={modalStyles.actions}>
              <Pressable
                style={[modalStyles.submitBtn, submitting && modalStyles.btnDisabled]}
                onPress={() => onSubmit(form)}
                disabled={submitting}
              >
                <Text style={modalStyles.submitBtnText}>{submitting ? "저장 중..." : "저장"}</Text>
              </Pressable>
              <Pressable style={modalStyles.cancelBtn} onPress={onCancel} disabled={submitting}>
                <Text style={modalStyles.cancelBtnText}>취소</Text>
              </Pressable>
            </View>
          </ScrollView>
        </View>
      </View>
    </Modal>
  );
}

const modalStyles = StyleSheet.create({
  overlay: {
    flex: 1,
    backgroundColor: "rgba(0,0,0,0.4)",
    justifyContent: "flex-end",
  },
  sheet: {
    backgroundColor: "#fff",
    borderTopLeftRadius: 20,
    borderTopRightRadius: 20,
    maxHeight: "90%",
  },
  content: {
    padding: 20,
    gap: 8,
    paddingBottom: Platform.OS === "ios" ? 40 : 24,
  },
  title: {
    fontSize: 16,
    fontWeight: "700",
    color: "#1e3a5f",
    marginBottom: 4,
  },
  error: {
    fontSize: 12,
    color: "#ff5442",
  },
  label: {
    fontSize: 12,
    color: "#6b8db5",
    marginTop: 4,
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
  hint: {
    fontSize: 11,
    color: "#94a3b8",
  },
  actions: {
    flexDirection: "row",
    gap: 10,
    marginTop: 8,
  },
  submitBtn: {
    flex: 1,
    paddingVertical: 12,
    borderRadius: 12,
    backgroundColor: "#4a9fe5",
    alignItems: "center",
  },
  submitBtnText: {
    color: "#fff",
    fontWeight: "600",
    fontSize: 14,
  },
  cancelBtn: {
    flex: 1,
    paddingVertical: 12,
    borderRadius: 12,
    backgroundColor: "#d4e6f5",
    alignItems: "center",
  },
  cancelBtnText: {
    color: "#6b8db5",
    fontWeight: "600",
    fontSize: 14,
  },
  btnDisabled: {
    opacity: 0.5,
  },
});

// --- AdminPushScreen ---

export function AdminPushScreen() {
  const { user, loading: authLoading } = useAuth();
  const navigation = useNavigation<NavProp>();

  const [pushes, setPushes] = useState<ScheduledPush[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const [editingPush, setEditingPush] = useState<ScheduledPush | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const [actioningId, setActioningId] = useState<string | null>(null);

  const loadPushes = useCallback(() => {
    setError(null);
    api
      .listScheduledPushes()
      .then(setPushes)
      .catch((e) => setError(e instanceof Error ? e.message : "불러오기 실패"))
      .finally(() => {
        setLoading(false);
        setRefreshing(false);
      });
  }, []);

  useEffect(() => {
    if (!authLoading && user?.role === "admin") {
      loadPushes();
    }
  }, [user, authLoading, loadPushes]);

  const handleRefresh = () => {
    setRefreshing(true);
    loadPushes();
  };

  const handleCreate = async (form: FormState) => {
    if (!form.title.trim() || !form.body.trim()) {
      setCreateError("제목과 본문은 필수입니다");
      return;
    }
    setCreateError(null);
    setCreating(true);
    try {
      const req: CreateScheduledPushRequest = {
        title: form.title.trim(),
        body: form.body.trim(),
        link: form.link.trim() || undefined,
        scheduled_at: form.scheduled_at.trim() || undefined,
      };
      await api.createScheduledPush(req);
      setShowCreate(false);
      loadPushes();
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : "생성 실패");
    } finally {
      setCreating(false);
    }
  };

  const handleUpdate = async (form: FormState) => {
    if (!editingPush) return;
    if (!form.title.trim() || !form.body.trim()) {
      setSaveError("제목과 본문은 필수입니다");
      return;
    }
    setSaveError(null);
    setSaving(true);
    try {
      const req: UpdateScheduledPushRequest = {
        title: form.title.trim(),
        body: form.body.trim(),
        link: form.link.trim() || undefined,
        scheduled_at: form.scheduled_at.trim() || undefined,
      };
      await api.updateScheduledPush(editingPush.id, req);
      setEditingPush(null);
      loadPushes();
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : "수정 실패");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = (id: string) => {
    Alert.alert("취소 확인", "이 푸시 알림을 취소하시겠습니까?", [
      { text: "아니요", style: "cancel" },
      {
        text: "취소",
        style: "destructive",
        onPress: async () => {
          setActioningId(id);
          try {
            await api.deleteScheduledPush(id);
            loadPushes();
          } catch (e) {
            setError(e instanceof Error ? e.message : "취소 실패");
          } finally {
            setActioningId(null);
          }
        },
      },
    ]);
  };

  const handleExecute = (id: string) => {
    Alert.alert("즉시 전송", "지금 즉시 전송하시겠습니까?", [
      { text: "취소", style: "cancel" },
      {
        text: "전송",
        onPress: async () => {
          setActioningId(id);
          try {
            await api.executeScheduledPush(id);
            loadPushes();
          } catch (e) {
            setError(e instanceof Error ? e.message : "전송 실패");
          } finally {
            setActioningId(null);
          }
        },
      },
    ]);
  };

  const editInitialForm = editingPush
    ? {
        title: editingPush.title,
        body: editingPush.body,
        link: editingPush.link,
        scheduled_at: editingPush.scheduled_at
          ? new Date(editingPush.scheduled_at).toISOString()
          : "",
      }
    : emptyForm;

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
    <View style={styles.container}>
      {/* Create modal */}
      <PushFormModal
        visible={showCreate}
        initial={emptyForm}
        onSubmit={handleCreate}
        onCancel={() => { setShowCreate(false); setCreateError(null); }}
        submitting={creating}
        error={createError}
        title="새 푸시 알림 만들기"
      />

      {/* Edit modal */}
      <PushFormModal
        visible={editingPush !== null}
        initial={editInitialForm}
        onSubmit={handleUpdate}
        onCancel={() => { setEditingPush(null); setSaveError(null); }}
        submitting={saving}
        error={saveError}
        title="푸시 알림 수정"
      />

      {/* Header */}
      <View style={styles.header}>
        <Text style={styles.headerTitle}>푸시 알림 관리</Text>
        <Pressable style={styles.createBtn} onPress={() => setShowCreate(true)}>
          <Text style={styles.createBtnText}>+ 새 알림</Text>
        </Pressable>
      </View>

      {error ? (
        <View style={styles.errorBox}>
          <Text style={styles.errorText}>{error}</Text>
        </View>
      ) : null}

      {loading ? (
        <View style={styles.centered}>
          <ActivityIndicator color="#4a9fe5" />
        </View>
      ) : (
        <FlatList
          data={pushes}
          keyExtractor={(item) => item.id}
          contentContainerStyle={styles.listContent}
          refreshing={refreshing}
          onRefresh={handleRefresh}
          ListEmptyComponent={
            <Text style={styles.emptyText}>등록된 푸시 알림이 없습니다.</Text>
          }
          renderItem={({ item: p }) => (
            <View style={styles.card}>
              <View style={styles.cardTop}>
                <View style={{ flex: 1 }}>
                  <View style={styles.cardTitleRow}>
                    <Text style={styles.cardTitle} numberOfLines={1}>{p.title}</Text>
                    <View
                      style={[
                        styles.badge,
                        { backgroundColor: STATUS_BADGE[p.status].bg },
                      ]}
                    >
                      <Text style={[styles.badgeText, { color: STATUS_BADGE[p.status].text }]}>
                        {STATUS_LABELS[p.status]}
                      </Text>
                    </View>
                  </View>
                  <Text style={styles.cardBody} numberOfLines={2}>{p.body}</Text>
                  {p.link ? (
                    <Text style={styles.cardLink} numberOfLines={1}>{p.link}</Text>
                  ) : null}
                  <View style={styles.cardMeta}>
                    {p.scheduled_at ? (
                      <Text style={styles.metaText}>
                        예약: {new Date(p.scheduled_at).toLocaleString("ko-KR")}
                      </Text>
                    ) : null}
                    {p.sent_at ? (
                      <Text style={styles.metaText}>
                        전송: {new Date(p.sent_at).toLocaleString("ko-KR")}
                      </Text>
                    ) : null}
                    <Text style={styles.metaText}>
                      생성: {new Date(p.created_at).toLocaleDateString("ko-KR")}
                    </Text>
                  </View>
                  {p.error_message ? (
                    <Text style={styles.cardError}>{p.error_message}</Text>
                  ) : null}
                </View>
              </View>

              {p.status === "pending" && (
                <View style={styles.cardActions}>
                  <Pressable
                    style={styles.actionBtn}
                    onPress={() => setEditingPush(p)}
                    disabled={actioningId === p.id}
                  >
                    <Text style={styles.actionBtnText}>수정</Text>
                  </Pressable>
                  <Pressable
                    style={[styles.actionBtn, styles.actionBtnGreen]}
                    onPress={() => handleExecute(p.id)}
                    disabled={actioningId === p.id}
                  >
                    <Text style={[styles.actionBtnText, { color: "#15803d" }]}>
                      {actioningId === p.id ? "..." : "즉시 전송"}
                    </Text>
                  </Pressable>
                  <Pressable
                    style={[styles.actionBtn, styles.actionBtnRed]}
                    onPress={() => handleDelete(p.id)}
                    disabled={actioningId === p.id}
                  >
                    <Text style={[styles.actionBtnText, { color: "#ff5442" }]}>취소</Text>
                  </Pressable>
                </View>
              )}
            </View>
          )}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "#f8fafc",
  },
  centered: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "#f8fafc",
    gap: 16,
  },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: 16,
    paddingVertical: 12,
    borderBottomWidth: 1,
    borderBottomColor: "#d4e6f5",
    backgroundColor: "#fff",
  },
  headerTitle: {
    fontSize: 16,
    fontWeight: "700",
    color: "#1e3a5f",
  },
  createBtn: {
    paddingHorizontal: 14,
    paddingVertical: 8,
    borderRadius: 10,
    backgroundColor: "#4a9fe5",
  },
  createBtnText: {
    color: "#fff",
    fontWeight: "600",
    fontSize: 13,
  },
  listContent: {
    padding: 16,
    gap: 12,
    paddingBottom: 40,
  },
  emptyText: {
    fontSize: 13,
    color: "#6b8db5",
    textAlign: "center",
    marginTop: 40,
  },
  errorBox: {
    marginHorizontal: 16,
    marginTop: 12,
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
  card: {
    borderRadius: 14,
    borderWidth: 1,
    borderColor: "#d4e6f5",
    backgroundColor: "#f0f7ff",
    padding: 14,
    gap: 10,
  },
  cardTop: {
    flexDirection: "row",
    gap: 8,
  },
  cardTitleRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    flexWrap: "wrap",
    marginBottom: 4,
  },
  cardTitle: {
    fontSize: 14,
    fontWeight: "600",
    color: "#1e3a5f",
    flex: 1,
  },
  badge: {
    paddingHorizontal: 8,
    paddingVertical: 3,
    borderRadius: 50,
  },
  badgeText: {
    fontSize: 10,
    fontWeight: "600",
  },
  cardBody: {
    fontSize: 12,
    color: "#6b8db5",
    lineHeight: 17,
  },
  cardLink: {
    fontSize: 11,
    color: "#4a9fe5",
    marginTop: 2,
  },
  cardMeta: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 8,
    marginTop: 6,
  },
  metaText: {
    fontSize: 11,
    color: "#94a3b8",
  },
  cardError: {
    fontSize: 11,
    color: "#ff5442",
    marginTop: 4,
  },
  cardActions: {
    flexDirection: "row",
    gap: 8,
    borderTopWidth: 1,
    borderTopColor: "#d4e6f5",
    paddingTop: 10,
  },
  actionBtn: {
    paddingHorizontal: 12,
    paddingVertical: 7,
    borderRadius: 8,
    backgroundColor: "#d4e6f5",
  },
  actionBtnGreen: {
    backgroundColor: "#dcfce7",
  },
  actionBtnRed: {
    backgroundColor: "rgba(255,84,66,0.1)",
  },
  actionBtnText: {
    fontSize: 12,
    fontWeight: "600",
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
