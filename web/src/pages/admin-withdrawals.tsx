import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import {
  getAdminWithdrawals,
  getAdminWithdrawalDetail,
  approveWithdrawal,
  rejectWithdrawal,
  updateTransitionNote,
  type WithdrawalListItem,
  type WithdrawalDetail,
} from "@/lib/api";
import { formatDateTime } from "@/lib/utils";

const STATUS_LABEL: Record<string, string> = {
  pending: "대기",
  approved: "승인",
  rejected: "거절",
  cancelled: "취소",
};

const STATUS_COLOR: Record<string, string> = {
  pending: "text-[#e5a54a] bg-[#e5a54a]/10 border-[#e5a54a]/30",
  approved: "text-green-600 bg-green-100 border-green-300",
  rejected: "text-[#ff5442] bg-[#ff5442]/10 border-[#ff5442]/30",
  cancelled: "text-[#6b8db5] bg-[#6b8db5]/10 border-[#6b8db5]/30",
};

const FILTERS = [
  { value: "", label: "전체" },
  { value: "pending", label: "대기" },
  { value: "approved", label: "승인" },
  { value: "rejected", label: "거절" },
  { value: "cancelled", label: "취소" },
];

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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white rounded-2xl shadow-xl p-6 w-full max-w-md mx-4 space-y-4">
        <h3 className="text-lg font-semibold text-[#1e3a5f]">{title}</h3>
        <textarea
          value={note}
          onChange={(e) => setNote(e.target.value)}
          rows={3}
          className="w-full border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f] resize-none"
          placeholder="비고를 입력하세요 (필수)"
          autoFocus
        />
        <div className="flex justify-end gap-2">
          <button
            onClick={onCancel}
            className="px-4 py-2 rounded-lg text-sm text-[#6b8db5] hover:text-[#1e3a5f] transition-colors"
          >
            취소
          </button>
          <button
            onClick={() => onConfirm(trimmed)}
            disabled={!trimmed}
            className="px-4 py-2 rounded-lg text-sm font-semibold transition-colors disabled:opacity-50"
            style={{ background: "var(--color-button-primary)", color: "white" }}
          >
            확인
          </button>
        </div>
      </div>
    </div>
  );
}

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
  const [modal, setModal] = useState<"approve" | "reject" | null>(null);
  const [editingTransitionId, setEditingTransitionId] = useState<string | null>(null);
  const [editNote, setEditNote] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const handleAction = async (action: "approve" | "reject", note: string) => {
    setBusy(true);
    setError(null);
    try {
      if (action === "approve") {
        await approveWithdrawal(detail.id, note);
      } else {
        await rejectWithdrawal(detail.id, note);
      }
      setModal(null);
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
      await updateTransitionNote(transitionId, trimmed);
      setEditingTransitionId(null);
      onRefresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "수정 실패");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="rounded-2xl border border-[#4a9fe5]/30 bg-white p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold text-[#1e3a5f]">출금 상세</h3>
        <button onClick={onClose} className="text-sm text-[#6b8db5] hover:text-[#1e3a5f]">닫기</button>
      </div>

      <div className="grid grid-cols-2 gap-2 text-sm">
        <div><span className="text-[#6b8db5]">금액:</span> {detail.amount.toLocaleString()}원</div>
        <div><span className="text-[#6b8db5]">상태:</span> {STATUS_LABEL[detail.current_status]}</div>
        <div><span className="text-[#6b8db5]">은행:</span> {detail.bank_name}</div>
        <div><span className="text-[#6b8db5]">계좌:</span> {detail.account_number}</div>
        <div><span className="text-[#6b8db5]">예금주:</span> {detail.account_holder}</div>
        <div><span className="text-[#6b8db5]">신청일:</span> {formatDateTime(detail.created_at)}</div>
      </div>

      {error && <p className="text-sm text-[#ff5442]">{error}</p>}

      {detail.current_status === "pending" && (
        <div className="flex gap-2">
          <button
            onClick={() => setModal("approve")}
            disabled={busy}
            className="px-4 py-2 rounded-lg text-sm font-semibold bg-green-100 text-green-700 hover:bg-green-200 transition-colors disabled:opacity-50"
          >
            승인
          </button>
          <button
            onClick={() => setModal("reject")}
            disabled={busy}
            className="px-4 py-2 rounded-lg text-sm font-semibold bg-[#ff5442]/10 text-[#ff5442] hover:bg-[#ff5442]/20 transition-colors disabled:opacity-50"
          >
            거절
          </button>
        </div>
      )}

      {/* Transitions */}
      <div className="space-y-2">
        <h4 className="text-sm font-semibold text-[#6b8db5]">상태 이력</h4>
        {detail.transitions?.map((t) => (
          <div key={t.id} className="rounded-lg border border-[#d4e6f5] bg-[#f0f7ff] p-3 text-sm space-y-1">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className={`text-xs px-2 py-0.5 rounded-full border ${STATUS_COLOR[t.status]}`}>
                  {STATUS_LABEL[t.status]}
                </span>
                {t.actor_name && <span className="text-xs text-[#6b8db5]">{t.actor_name}</span>}
              </div>
              <span className="text-xs text-[#6b8db5]">{formatDateTime(t.created_at)}</span>
            </div>
            {editingTransitionId === t.id ? (
              <div className="flex gap-2 mt-1">
                <input
                  value={editNote}
                  onChange={(e) => setEditNote(e.target.value)}
                  className="flex-1 bg-white border border-[#d4e6f5] rounded px-2 py-1 text-sm text-[#1e3a5f]"
                  autoFocus
                />
                <button
                  onClick={() => handleEditNote(t.id)}
                  disabled={busy || !editNote.trim()}
                  className="px-2 py-1 rounded text-xs font-semibold bg-[#4a9fe5]/20 text-[#4a9fe5] disabled:opacity-50"
                >
                  저장
                </button>
                <button
                  onClick={() => setEditingTransitionId(null)}
                  className="px-2 py-1 rounded text-xs text-[#6b8db5]"
                >
                  취소
                </button>
              </div>
            ) : (
              t.note && (
                <div className="flex items-start gap-2">
                  <p className="text-[#1e3a5f] flex-1">{t.note}</p>
                  {t.actor_id === adminId && (
                    <button
                      onClick={() => { setEditingTransitionId(t.id); setEditNote(t.note); }}
                      className="text-xs text-[#6b8db5] hover:text-[#1e3a5f] shrink-0"
                    >
                      수정
                    </button>
                  )}
                </div>
              )
            )}
            {t.updated_at !== t.created_at && (
              <p className="text-xs text-[#6b8db5]">수정: {formatDateTime(t.updated_at)}</p>
            )}
          </div>
        ))}
      </div>

      {modal && (
        <NoteModal
          title={modal === "approve" ? "승인 비고 입력" : "거절 사유 입력"}
          onConfirm={(note) => handleAction(modal, note)}
          onCancel={() => setModal(null)}
        />
      )}
    </div>
  );
}

export function AdminWithdrawalsPage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();

  const [items, setItems] = useState<WithdrawalListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const [selectedDetail, setSelectedDetail] = useState<WithdrawalDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const limit = 20;

  const loadList = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await getAdminWithdrawals(filter, limit, page * limit);
      setItems(result.data);
      setTotal(result.total);
    } catch {
      setError("목록 불러오기 실패");
    } finally {
      setLoading(false);
    }
  }, [filter, page]);

  useEffect(() => {
    if (!authLoading && (!user || user.role !== "admin")) {
      navigate("/", { replace: true });
    }
  }, [user, authLoading, navigate]);

  useEffect(() => {
    if (user?.role === "admin") loadList();
  }, [user, loadList]);

  const openDetail = async (id: string) => {
    try {
      const detail = await getAdminWithdrawalDetail(id);
      setSelectedDetail(detail);
    } catch {
      setError("상세 조회 실패");
    }
  };

  const refreshDetail = async () => {
    if (!selectedDetail) return;
    try {
      const detail = await getAdminWithdrawalDetail(selectedDetail.id);
      setSelectedDetail(detail);
      await loadList();
    } catch {
      // ignore
    }
  };

  if (authLoading || !user || user.role !== "admin") {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ backgroundColor: "var(--color-bg)" }}>
        <p className="text-[#6b8db5]">로딩 중...</p>
      </div>
    );
  }

  const totalPages = Math.ceil(total / limit);

  return (
    <div className="min-h-screen p-8" style={{ backgroundColor: "var(--color-bg)", color: "var(--color-fg)" }}>
      <div className="max-w-4xl mx-auto space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold text-[#1e3a5f]">출금 관리</h1>
          <button
            onClick={() => navigate("/admin")}
            className="text-sm text-[#6b8db5] hover:text-[#1e3a5f] transition-colors"
          >
            ← 관리자 페이지
          </button>
        </div>

        {/* Filters */}
        <div className="flex gap-2 flex-wrap">
          {FILTERS.map((f) => (
            <button
              key={f.value}
              onClick={() => { setFilter(f.value); setPage(0); }}
              className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                filter === f.value
                  ? "bg-[#4a9fe5] text-white"
                  : "bg-[#f0f7ff] text-[#6b8db5] hover:text-[#1e3a5f] border border-[#d4e6f5]"
              }`}
            >
              {f.label}
            </button>
          ))}
          <span className="ml-auto text-sm text-[#6b8db5] self-center">총 {total}건</span>
        </div>

        {error && (
          <div className="rounded-xl border border-[#ff5442]/30 bg-[#ff5442]/10 p-3 text-sm text-[#ff5442]">
            {error}
          </div>
        )}

        {/* Detail panel */}
        {selectedDetail && (
          <DetailPanel
            detail={selectedDetail}
            adminId={user.id}
            onClose={() => setSelectedDetail(null)}
            onRefresh={refreshDetail}
          />
        )}

        {/* Table */}
        {loading ? (
          <p className="text-sm text-[#6b8db5]">불러오는 중...</p>
        ) : items.length === 0 ? (
          <p className="text-sm text-[#6b8db5]">출금 내역이 없습니다.</p>
        ) : (
          <div className="overflow-x-auto rounded-2xl border border-[#d4e6f5]">
            <table className="w-full text-sm">
              <thead className="bg-[#f0f7ff]">
                <tr>
                  <th className="text-left px-4 py-3 text-[#6b8db5] font-medium">날짜</th>
                  <th className="text-left px-4 py-3 text-[#6b8db5] font-medium">신청자</th>
                  <th className="text-right px-4 py-3 text-[#6b8db5] font-medium">금액</th>
                  <th className="text-left px-4 py-3 text-[#6b8db5] font-medium">은행/계좌</th>
                  <th className="text-center px-4 py-3 text-[#6b8db5] font-medium">상태</th>
                  <th className="text-center px-4 py-3 text-[#6b8db5] font-medium">상세</th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-[#d4e6f5]">
                {items.map((item) => (
                  <tr key={item.id} className="hover:bg-[#f0f7ff]/50">
                    <td className="px-4 py-3 text-[#1e3a5f] whitespace-nowrap">{formatDateTime(item.created_at)}</td>
                    <td className="px-4 py-3">
                      <p className="text-[#1e3a5f]">{item.user_nickname || "-"}</p>
                      <p className="text-xs text-[#6b8db5]">{item.user_email || "-"}</p>
                    </td>
                    <td className="px-4 py-3 text-right font-semibold text-[#1e3a5f]">
                      {item.amount.toLocaleString()}
                    </td>
                    <td className="px-4 py-3 text-[#6b8db5] text-xs">
                      {item.bank_name} {item.account_number}
                      <br />{item.account_holder}
                    </td>
                    <td className="px-4 py-3 text-center">
                      <span className={`text-xs px-2 py-0.5 rounded-full border ${STATUS_COLOR[item.current_status]}`}>
                        {STATUS_LABEL[item.current_status]}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-center">
                      <button
                        onClick={() => openDetail(item.id)}
                        className="text-xs text-[#4a9fe5] hover:text-[#1e3a5f] transition-colors"
                      >
                        보기
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex justify-center gap-2">
            <button
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              className="px-3 py-1.5 rounded-lg text-sm bg-[#f0f7ff] text-[#6b8db5] hover:text-[#1e3a5f] disabled:opacity-50 border border-[#d4e6f5]"
            >
              이전
            </button>
            <span className="px-3 py-1.5 text-sm text-[#6b8db5]">
              {page + 1} / {totalPages}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={page >= totalPages - 1}
              className="px-3 py-1.5 rounded-lg text-sm bg-[#f0f7ff] text-[#6b8db5] hover:text-[#1e3a5f] disabled:opacity-50 border border-[#d4e6f5]"
            >
              다음
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
