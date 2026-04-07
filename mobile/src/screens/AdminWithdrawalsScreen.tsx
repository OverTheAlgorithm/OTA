// Ported from: web/src/pages/admin-withdrawals.tsx
import React, { useCallback, useEffect, useState } from "react";
import {
  View,
  Text,
  ScrollView,
  FlatList,
  Pressable,
  TextInput,
  ActivityIndicator,
  Modal,
  StyleSheet,
} from "react-native";
import { useNavigation } from "@react-navigation/native";
import { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { useAuth } from "../contexts/auth-context";
import { api } from "../lib/api";
import type {
  WithdrawalListItem,
  WithdrawalDetail,
} from "../../../packages/shared/src/types";
import { formatDateTime } from "../../../packages/shared/src/utils";

type NavProp = NativeStackNavigationProp<Record<string, object | undefined>>;

const STATUS_LABEL: Record<string, string> = {
  pending: "대기",
  approved: "승인",
  rejected: "거절",
  cancelled: "취소",
};

const STATUS_COLOR: Record<string, { bg: string; text: string; border: string }> = {
  pending:   { bg: "rgba(229,165,74,0.1)",   text: "#e5a54a", border: "rgba(229,165,74,0.3)" },
  approved:  { bg: "#dcfce7",                 text: "#15803d", border: "#86efac" },
  rejected:  { bg: "rgba(255,84,66,0.1)",    text: "#ff5442", border: "rgba(255,84,66,0.3)" },
  cancelled: { bg: "rgba(107,141,181,0.1)",  text: "#6b8db5", border: "rgba(107,141,181,0.3)" },
};

const FILTERS = [
  { value: "", label: "전체" },
  { value: "pending", label: "대기" },
  { value: "approved", label: "승인" },
  { value: "rejected", label: "거절" },
  { value: "cancelled", label: "취소" },
];

const LIMIT = 20;

// --- NoteModal ---

function NoteModal({
  title,
  onConfirm,
  onCancel,
}: {
  title: string;
  onConfirm: (note: string) => void;
  onCancel: () => void;
}) {
  const [note, setNote] = useState("");
  const trimmed = note.trim();

  return (
    <Modal transparent animationType="fade" onRequestClose={onCancel}>
      <View style={modal.overlay}>
        <View style={modal.box}>
          <Text style={modal.title}>{title}</Text>
          <TextInput
            value={note}
            onChangeText={setNote}
            style={modal.input}
            placeholder="비고를 입력하세요 (필수)"
            multiline
            autoFocus
          />
          <View style={modal.btnRow}>
            <Pressable style={modal.cancelBtn} onPress={onCancel}>
              <Text style={modal.cancelText}>취소</Text>
            </Pressable>
            <Pressable
              style={[modal.confirmBtn, !trimmed && modal.btnDisabled]}
              onPress={() => onConfirm(trimmed)}
              disabled={!trimmed}
            >
              <Text style={modal.confirmText}>확인</Text>
            </Pressable>
          </View>
        </View>
      </View>
    </Modal>
  );
}

const modal = StyleSheet.create({
  overlay: {
    flex: 1,
    backgroundColor: "rgba(0,0,0,0.4)",
    alignItems: "center",
    justifyContent: "center",
    padding: 20,
  },
  box: {
    backgroundColor: "#fff",
    borderRadius: 16,
    padding: 20,
    width: "100%",
    gap: 12,
  },
  title: {
    fontSize: 16,
    fontWeight: "600",
    color: "#1e3a5f",
  },
  input: {
    borderWidth: 1,
    borderColor: "#d4e6f5",
    borderRadius: 10,
    paddingHorizontal: 12,
    paddingVertical: 9,
    fontSize: 13,
    color: "#1e3a5f",
    minHeight: 80,
    textAlignVertical: "top",
  },
  btnRow: {
    flexDirection: "row",
    justifyContent: "flex-end",
    gap: 8,
  },
  cancelBtn: {
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 8,
  },
  cancelText: {
    fontSize: 13,
    color: "#6b8db5",
  },
  confirmBtn: {
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 8,
    backgroundColor: "#4a9fe5",
  },
  confirmText: {
    fontSize: 13,
    fontWeight: "600",
    color: "#fff",
  },
  btnDisabled: {
    opacity: 0.5,
  },
});

// --- DetailPanel ---

function DetailPanel({
  detail,
  adminId,
  onClose,
  onRefresh,
}: {
  detail: WithdrawalDetail;
  adminId: string;
  onClose: () => void;
  onRefresh: () => void;
}) {
  const [actionModal, setActionModal] = useState<"approve" | "reject" | null>(null);
  const [editingTransitionId, setEditingTransitionId] = useState<string | null>(null);
  const [editNote, setEditNote] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const handleAction = async (action: "approve" | "reject", note: string) => {
    setBusy(true);
    setError(null);
    try {
      if (action === "approve") {
        await api.approveWithdrawal(detail.id, note);
      } else {
        await api.rejectWithdrawal(detail.id, note);
      }
      setActionModal(null);
      onRefresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "작업 실패");
    } finally {
      setBusy(false);
    }
  };

  const handleEditNote = async (transitionId: string) => {
    const trimmed = editNote.trim();
    if (!trimmed) return;
    setBusy(true);
    setError(null);
    try {
      await api.updateTransitionNote(transitionId, trimmed);
      setEditingTransitionId(null);
      onRefresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "수정 실패");
    } finally {
      setBusy(false);
    }
  };

  const statusColors = STATUS_COLOR[detail.current_status] ?? STATUS_COLOR.pending;

  return (
    <View style={styles.detailPanel}>
      <View style={styles.detailHeader}>
        <Text style={styles.detailTitle}>출금 상세</Text>
        <Pressable onPress={onClose}>
          <Text style={styles.closeBtn}>닫기</Text>
        </Pressable>
      </View>

      <View style={styles.detailGrid}>
        <View style={styles.detailRow}>
          <Text style={styles.detailLabel}>금액</Text>
          <Text style={styles.detailValue}>{detail.amount.toLocaleString()}원</Text>
        </View>
        <View style={styles.detailRow}>
          <Text style={styles.detailLabel}>상태</Text>
          <View
            style={[
              styles.statusBadge,
              { backgroundColor: statusColors.bg, borderColor: statusColors.border },
            ]}
          >
            <Text style={[styles.statusText, { color: statusColors.text }]}>
              {STATUS_LABEL[detail.current_status]}
            </Text>
          </View>
        </View>
        <View style={styles.detailRow}>
          <Text style={styles.detailLabel}>은행</Text>
          <Text style={styles.detailValue}>{detail.bank_name}</Text>
        </View>
        <View style={styles.detailRow}>
          <Text style={styles.detailLabel}>계좌</Text>
          <Text style={styles.detailValue}>{detail.account_number}</Text>
        </View>
        <View style={styles.detailRow}>
          <Text style={styles.detailLabel}>예금주</Text>
          <Text style={styles.detailValue}>{detail.account_holder}</Text>
        </View>
        <View style={styles.detailRow}>
          <Text style={styles.detailLabel}>신청일</Text>
          <Text style={styles.detailValue}>{formatDateTime(detail.created_at)}</Text>
        </View>
      </View>

      {error && <Text style={styles.errorText}>{error}</Text>}

      {detail.current_status === "pending" && (
        <View style={styles.actionRow}>
          <Pressable
            style={[styles.approveBtn, busy && styles.btnDisabled]}
            onPress={() => setActionModal("approve")}
            disabled={busy}
          >
            <Text style={styles.approveBtnText}>승인</Text>
          </Pressable>
          <Pressable
            style={[styles.rejectBtn, busy && styles.btnDisabled]}
            onPress={() => setActionModal("reject")}
            disabled={busy}
          >
            <Text style={styles.rejectBtnText}>거절</Text>
          </Pressable>
        </View>
      )}

      {/* Transitions */}
      <Text style={styles.transitionsTitle}>상태 이력</Text>
      {(detail.transitions ?? []).map((t) => {
        const tc = STATUS_COLOR[t.status] ?? STATUS_COLOR.pending;
        return (
          <View key={t.id} style={styles.transitionCard}>
            <View style={styles.transitionHeader}>
              <View style={styles.transitionLeft}>
                <View
                  style={[
                    styles.statusBadge,
                    { backgroundColor: tc.bg, borderColor: tc.border },
                  ]}
                >
                  <Text style={[styles.statusText, { color: tc.text }]}>
                    {STATUS_LABEL[t.status]}
                  </Text>
                </View>
                {t.actor_name ? (
                  <Text style={styles.actorName}>{t.actor_name}</Text>
                ) : null}
              </View>
              <Text style={styles.transitionDate}>{formatDateTime(t.created_at)}</Text>
            </View>

            {editingTransitionId === t.id ? (
              <View style={styles.editNoteRow}>
                <TextInput
                  value={editNote}
                  onChangeText={setEditNote}
                  style={[styles.noteInput, { flex: 1 }]}
                  autoFocus
                />
                <Pressable
                  style={[styles.saveNoteBtn, (busy || !editNote.trim()) && styles.btnDisabled]}
                  onPress={() => handleEditNote(t.id)}
                  disabled={busy || !editNote.trim()}
                >
                  <Text style={styles.saveNoteBtnText}>저장</Text>
                </Pressable>
                <Pressable
                  onPress={() => setEditingTransitionId(null)}
                  style={styles.cancelNoteBtn}
                >
                  <Text style={styles.cancelNoteText}>취소</Text>
                </Pressable>
              </View>
            ) : t.note ? (
              <View style={styles.noteRow}>
                <Text style={styles.noteText}>{t.note}</Text>
                {t.actor_id === adminId && (
                  <Pressable
                    onPress={() => {
                      setEditingTransitionId(t.id);
                      setEditNote(t.note);
                    }}
                  >
                    <Text style={styles.editNoteLink}>수정</Text>
                  </Pressable>
                )}
              </View>
            ) : null}

            {t.updated_at !== t.created_at && (
              <Text style={styles.updatedAt}>수정: {formatDateTime(t.updated_at)}</Text>
            )}
          </View>
        );
      })}

      {actionModal && (
        <NoteModal
          title={actionModal === "approve" ? "승인 비고 입력" : "거절 사유 입력"}
          onConfirm={(note) => handleAction(actionModal, note)}
          onCancel={() => setActionModal(null)}
        />
      )}
    </View>
  );
}

// --- AdminWithdrawalsScreen ---

export function AdminWithdrawalsScreen() {
  const { user, loading: authLoading } = useAuth();
  const navigation = useNavigation<NavProp>();

  const [items, setItems] = useState<WithdrawalListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const [selectedDetail, setSelectedDetail] = useState<WithdrawalDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadList = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.getAdminWithdrawals(filter, LIMIT, page * LIMIT);
      setItems(result.data);
      setTotal(result.total);
    } catch {
      setError("목록 불러오기 실패");
    } finally {
      setLoading(false);
    }
  }, [filter, page]);

  useEffect(() => {
    if (user?.role === "admin") loadList();
  }, [user, loadList]);

  const openDetail = async (id: string) => {
    try {
      const detail = await api.getAdminWithdrawalDetail(id);
      setSelectedDetail(detail);
    } catch {
      setError("상세 조회 실패");
    }
  };

  const refreshDetail = async () => {
    if (!selectedDetail) return;
    try {
      const detail = await api.getAdminWithdrawalDetail(selectedDetail.id);
      setSelectedDetail(detail);
      await loadList();
    } catch {
      // ignore
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

  const totalPages = Math.ceil(total / LIMIT);

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {/* Filters */}
      <ScrollView horizontal showsHorizontalScrollIndicator={false}>
        <View style={styles.filterRow}>
          {FILTERS.map((f) => (
            <Pressable
              key={f.value}
              style={[styles.filterBtn, filter === f.value && styles.filterBtnActive]}
              onPress={() => {
                setFilter(f.value);
                setPage(0);
              }}
            >
              <Text
                style={[
                  styles.filterBtnText,
                  filter === f.value && styles.filterBtnTextActive,
                ]}
              >
                {f.label}
              </Text>
            </Pressable>
          ))}
          <Text style={styles.totalText}>총 {total}건</Text>
        </View>
      </ScrollView>

      {error && (
        <View style={styles.errorBox}>
          <Text style={styles.errorText}>{error}</Text>
        </View>
      )}

      {/* Detail Panel */}
      {selectedDetail && (
        <DetailPanel
          detail={selectedDetail}
          adminId={user.id}
          onClose={() => setSelectedDetail(null)}
          onRefresh={refreshDetail}
        />
      )}

      {/* List */}
      {loading ? (
        <ActivityIndicator color="#4a9fe5" style={{ marginTop: 32 }} />
      ) : items.length === 0 ? (
        <Text style={styles.emptyText}>출금 내역이 없습니다.</Text>
      ) : (
        items.map((item) => {
          const sc = STATUS_COLOR[item.current_status] ?? STATUS_COLOR.pending;
          return (
            <View key={item.id} style={styles.listCard}>
              <View style={styles.listCardTop}>
                <View style={{ flex: 1 }}>
                  <Text style={styles.listUser}>{item.user_nickname || "-"}</Text>
                  <Text style={styles.listEmail}>{item.user_email || "-"}</Text>
                </View>
                <View style={[styles.statusBadge, { backgroundColor: sc.bg, borderColor: sc.border }]}>
                  <Text style={[styles.statusText, { color: sc.text }]}>
                    {STATUS_LABEL[item.current_status]}
                  </Text>
                </View>
              </View>
              <View style={styles.listCardMid}>
                <Text style={styles.listAmount}>{item.amount.toLocaleString()}원</Text>
                <Text style={styles.listBank}>
                  {item.bank_name} {item.account_number} ({item.account_holder})
                </Text>
              </View>
              <View style={styles.listCardBottom}>
                <Text style={styles.listDate}>{formatDateTime(item.created_at)}</Text>
                <Pressable onPress={() => openDetail(item.id)}>
                  <Text style={styles.viewLink}>보기</Text>
                </Pressable>
              </View>
            </View>
          );
        })
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <View style={styles.paginationRow}>
          <Pressable
            style={[styles.pageBtn, page === 0 && styles.btnDisabled]}
            onPress={() => setPage((p) => Math.max(0, p - 1))}
            disabled={page === 0}
          >
            <Text style={styles.pageBtnText}>이전</Text>
          </Pressable>
          <Text style={styles.pageInfo}>
            {page + 1} / {totalPages}
          </Text>
          <Pressable
            style={[styles.pageBtn, page >= totalPages - 1 && styles.btnDisabled]}
            onPress={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
            disabled={page >= totalPages - 1}
          >
            <Text style={styles.pageBtnText}>다음</Text>
          </Pressable>
        </View>
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
  filterRow: {
    flexDirection: "row",
    gap: 8,
    alignItems: "center",
  },
  filterBtn: {
    paddingHorizontal: 14,
    paddingVertical: 7,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: "#d4e6f5",
    backgroundColor: "#f0f7ff",
  },
  filterBtnActive: {
    backgroundColor: "#4a9fe5",
    borderColor: "#4a9fe5",
  },
  filterBtnText: {
    fontSize: 13,
    color: "#6b8db5",
  },
  filterBtnTextActive: {
    color: "#fff",
    fontWeight: "600",
  },
  totalText: {
    fontSize: 12,
    color: "#6b8db5",
    marginLeft: 8,
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
  emptyText: {
    fontSize: 13,
    color: "#6b8db5",
    textAlign: "center",
    marginTop: 32,
  },
  listCard: {
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "#d4e6f5",
    backgroundColor: "#fff",
    padding: 14,
    gap: 8,
  },
  listCardTop: {
    flexDirection: "row",
    alignItems: "flex-start",
    gap: 8,
  },
  listUser: {
    fontSize: 14,
    fontWeight: "600",
    color: "#1e3a5f",
  },
  listEmail: {
    fontSize: 11,
    color: "#6b8db5",
    marginTop: 2,
  },
  listCardMid: {
    gap: 2,
  },
  listAmount: {
    fontSize: 16,
    fontWeight: "700",
    color: "#1e3a5f",
  },
  listBank: {
    fontSize: 12,
    color: "#6b8db5",
  },
  listCardBottom: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
  },
  listDate: {
    fontSize: 11,
    color: "#6b8db5",
  },
  viewLink: {
    fontSize: 13,
    color: "#4a9fe5",
    fontWeight: "600",
  },
  statusBadge: {
    paddingHorizontal: 8,
    paddingVertical: 3,
    borderRadius: 50,
    borderWidth: 1,
    alignSelf: "flex-start",
  },
  statusText: {
    fontSize: 11,
    fontWeight: "600",
  },
  paginationRow: {
    flexDirection: "row",
    justifyContent: "center",
    alignItems: "center",
    gap: 12,
    marginTop: 8,
  },
  pageBtn: {
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: "#d4e6f5",
    backgroundColor: "#f0f7ff",
  },
  pageBtnText: {
    fontSize: 13,
    color: "#6b8db5",
  },
  pageInfo: {
    fontSize: 13,
    color: "#6b8db5",
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
  // DetailPanel
  detailPanel: {
    borderRadius: 16,
    borderWidth: 1,
    borderColor: "rgba(74,159,229,0.3)",
    backgroundColor: "#fff",
    padding: 16,
    gap: 10,
  },
  detailHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  detailTitle: {
    fontSize: 16,
    fontWeight: "600",
    color: "#1e3a5f",
  },
  closeBtn: {
    fontSize: 13,
    color: "#6b8db5",
  },
  detailGrid: {
    gap: 6,
  },
  detailRow: {
    flexDirection: "row",
    gap: 8,
    alignItems: "center",
  },
  detailLabel: {
    fontSize: 13,
    color: "#6b8db5",
    width: 50,
  },
  detailValue: {
    fontSize: 13,
    color: "#1e3a5f",
    flex: 1,
  },
  actionRow: {
    flexDirection: "row",
    gap: 8,
  },
  approveBtn: {
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 8,
    backgroundColor: "#dcfce7",
  },
  approveBtnText: {
    fontSize: 13,
    fontWeight: "600",
    color: "#15803d",
  },
  rejectBtn: {
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 8,
    backgroundColor: "rgba(255,84,66,0.1)",
  },
  rejectBtnText: {
    fontSize: 13,
    fontWeight: "600",
    color: "#ff5442",
  },
  transitionsTitle: {
    fontSize: 13,
    fontWeight: "600",
    color: "#6b8db5",
    marginTop: 4,
  },
  transitionCard: {
    borderRadius: 8,
    borderWidth: 1,
    borderColor: "#d4e6f5",
    backgroundColor: "#f0f7ff",
    padding: 10,
    gap: 6,
  },
  transitionHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  transitionLeft: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  actorName: {
    fontSize: 11,
    color: "#6b8db5",
  },
  transitionDate: {
    fontSize: 11,
    color: "#6b8db5",
  },
  noteRow: {
    flexDirection: "row",
    alignItems: "flex-start",
    gap: 8,
  },
  noteText: {
    fontSize: 13,
    color: "#1e3a5f",
    flex: 1,
  },
  editNoteLink: {
    fontSize: 11,
    color: "#6b8db5",
  },
  editNoteRow: {
    flexDirection: "row",
    gap: 6,
    alignItems: "center",
  },
  noteInput: {
    backgroundColor: "#fff",
    borderWidth: 1,
    borderColor: "#d4e6f5",
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 5,
    fontSize: 12,
    color: "#1e3a5f",
  },
  saveNoteBtn: {
    paddingHorizontal: 10,
    paddingVertical: 5,
    borderRadius: 6,
    backgroundColor: "rgba(74,159,229,0.2)",
  },
  saveNoteBtnText: {
    fontSize: 11,
    fontWeight: "600",
    color: "#4a9fe5",
  },
  cancelNoteBtn: {
    paddingHorizontal: 8,
    paddingVertical: 5,
  },
  cancelNoteText: {
    fontSize: 11,
    color: "#6b8db5",
  },
  updatedAt: {
    fontSize: 10,
    color: "#6b8db5",
  },
});
