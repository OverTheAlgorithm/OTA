import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import {
  listScheduledPushes,
  createScheduledPush,
  updateScheduledPush,
  deleteScheduledPush,
  executeScheduledPush,
} from "@/lib/api";
import type { ScheduledPush, CreateScheduledPushRequest, UpdateScheduledPushRequest } from "@/lib/api";

const STATUS_LABELS: Record<ScheduledPush["status"], string> = {
  pending: "대기중",
  sent: "전송됨",
  failed: "실패",
  cancelled: "취소됨",
};

const STATUS_COLORS: Record<ScheduledPush["status"], string> = {
  pending: "bg-yellow-100 text-yellow-800",
  sent: "bg-green-100 text-green-700",
  failed: "bg-red-100 text-red-700",
  cancelled: "bg-gray-100 text-gray-500",
};

type FormState = {
  title: string;
  body: string;
  link: string;
  scheduled_at: string;
};

const emptyForm: FormState = { title: "", body: "", link: "", scheduled_at: "" };

export function AdminPushPage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();

  const [pushes, setPushes] = useState<ScheduledPush[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState("");

  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState<FormState>(emptyForm);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const [editingId, setEditingId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<FormState>(emptyForm);
  const [saving, setSaving] = useState(false);

  const [actioningId, setActioningId] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading) return;
    if (!user) { navigate("/", { replace: true }); return; }
    if (user.role !== "admin") { navigate("/", { replace: true }); return; }
    loadPushes();
  }, [user, authLoading, navigate, statusFilter]);

  const loadPushes = () => {
    setLoading(true);
    setError(null);
    listScheduledPushes(statusFilter || undefined)
      .then(setPushes)
      .catch((e) => setError(e instanceof Error ? e.message : "불러오기 실패"))
      .finally(() => setLoading(false));
  };

  const handleCreate = async () => {
    if (!createForm.title.trim() || !createForm.body.trim()) {
      setCreateError("제목과 본문은 필수입니다");
      return;
    }
    setCreateError(null);
    setCreating(true);
    try {
      const req: CreateScheduledPushRequest = {
        title: createForm.title.trim(),
        body: createForm.body.trim(),
        link: createForm.link.trim() || undefined,
        scheduled_at: createForm.scheduled_at || undefined,
      };
      await createScheduledPush(req);
      setShowCreate(false);
      setCreateForm(emptyForm);
      loadPushes();
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : "생성 실패");
      setCreating(false);
    }
  };

  const startEdit = (p: ScheduledPush) => {
    setEditingId(p.id);
    setEditForm({
      title: p.title,
      body: p.body,
      link: p.link,
      scheduled_at: p.scheduled_at
        ? new Date(p.scheduled_at).toISOString().slice(0, 16)
        : "",
    });
  };

  const handleUpdate = async (id: string) => {
    if (!editForm.title.trim() || !editForm.body.trim()) return;
    setSaving(true);
    try {
      const req: UpdateScheduledPushRequest = {
        title: editForm.title.trim(),
        body: editForm.body.trim(),
        link: editForm.link.trim() || undefined,
        scheduled_at: editForm.scheduled_at || undefined,
      };
      await updateScheduledPush(id, req);
      setEditingId(null);
      loadPushes();
    } catch (e) {
      setError(e instanceof Error ? e.message : "수정 실패");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("이 푸시 알림을 취소하시겠습니까?")) return;
    setActioningId(id);
    try {
      await deleteScheduledPush(id);
      loadPushes();
    } catch (e) {
      setError(e instanceof Error ? e.message : "삭제 실패");
    } finally {
      setActioningId(null);
    }
  };

  const handleExecute = async (id: string) => {
    if (!confirm("지금 즉시 전송하시겠습니까?")) return;
    setActioningId(id);
    try {
      await executeScheduledPush(id);
      loadPushes();
    } catch (e) {
      setError(e instanceof Error ? e.message : "전송 실패");
    } finally {
      setActioningId(null);
    }
  };

  if (authLoading || !user || user.role !== "admin") {
    return (
      <div className="min-h-screen flex items-center justify-center bg-white">
        <p className="text-[#6b8db5]">로딩 중...</p>
      </div>
    );
  }

  return (
    <div
      className="min-h-screen p-8"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-fg)" }}
    >
      <div className="max-w-4xl mx-auto space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">푸시 알림 관리</h1>
          <button
            onClick={() => navigate("/admin")}
            className="text-sm text-[#6b8db5] hover:text-[#1e3a5f] transition-colors"
          >
            ← 관리자 페이지
          </button>
        </div>

        {error && (
          <div className="rounded-xl border border-[#ff5442]/30 bg-[#ff5442]/10 p-3 text-sm text-[#ff5442]">
            {error}
          </div>
        )}

        {/* Filters + Create button */}
        <div className="flex items-center justify-between gap-4 flex-wrap">
          <div className="flex items-center gap-2">
            <label className="text-sm text-[#6b8db5]">상태 필터:</label>
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="bg-white border border-[#d4e6f5] rounded-lg px-3 py-1.5 text-sm text-[#1e3a5f]"
            >
              <option value="">전체</option>
              <option value="pending">대기중</option>
              <option value="sent">전송됨</option>
              <option value="failed">실패</option>
              <option value="cancelled">취소됨</option>
            </select>
          </div>
          <button
            onClick={() => { setShowCreate(!showCreate); setCreateError(null); }}
            className="px-4 py-2 rounded-xl font-semibold text-sm text-white transition-colors"
            style={{ background: "var(--color-button-primary)" }}
          >
            {showCreate ? "취소" : "새 푸시 알림 만들기"}
          </button>
        </div>

        {/* Create form */}
        {showCreate && (
          <div className="rounded-xl border border-[#4a9fe5]/30 bg-white p-5 space-y-3">
            <h3 className="text-sm font-semibold text-[#1e3a5f]">새 푸시 알림 만들기</h3>
            {createError && <p className="text-xs text-[#ff5442]">{createError}</p>}
            <input
              value={createForm.title}
              onChange={(e) => setCreateForm({ ...createForm, title: e.target.value })}
              placeholder="제목 (필수, 최대 100자)"
              maxLength={100}
              className="w-full bg-[#f0f7ff] border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
            />
            <textarea
              value={createForm.body}
              onChange={(e) => setCreateForm({ ...createForm, body: e.target.value })}
              placeholder="본문 (필수, 최대 500자)"
              maxLength={500}
              rows={3}
              className="w-full bg-[#f0f7ff] border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
            />
            <input
              value={createForm.link}
              onChange={(e) => setCreateForm({ ...createForm, link: e.target.value })}
              placeholder="링크 URL (선택)"
              className="w-full bg-[#f0f7ff] border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
            />
            <div className="space-y-1">
              <label className="text-xs text-[#6b8db5]">예약 전송 시각 (선택 — 미입력시 즉시 전송만 가능)</label>
              <input
                type="datetime-local"
                value={createForm.scheduled_at}
                onChange={(e) => setCreateForm({ ...createForm, scheduled_at: e.target.value })}
                className="w-full bg-[#f0f7ff] border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
              />
            </div>
            <button
              onClick={handleCreate}
              disabled={creating}
              className="px-4 py-2 rounded-lg bg-[#4a9fe5]/20 text-[#4a9fe5] text-sm font-semibold hover:bg-[#4a9fe5]/30 transition-colors disabled:opacity-50"
            >
              {creating ? "생성 중..." : "생성"}
            </button>
          </div>
        )}

        {/* Push list */}
        {loading ? (
          <p className="text-sm text-[#6b8db5]">불러오는 중...</p>
        ) : pushes.length === 0 ? (
          <p className="text-sm text-[#6b8db5]">등록된 푸시 알림이 없습니다.</p>
        ) : (
          <div className="space-y-3">
            {pushes.map((p) => (
              <div key={p.id} className="rounded-xl border border-[#d4e6f5] bg-[#f0f7ff] p-4 space-y-3">
                {editingId === p.id ? (
                  <div className="space-y-2">
                    <input
                      value={editForm.title}
                      onChange={(e) => setEditForm({ ...editForm, title: e.target.value })}
                      placeholder="제목"
                      maxLength={100}
                      className="w-full bg-white border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
                    />
                    <textarea
                      value={editForm.body}
                      onChange={(e) => setEditForm({ ...editForm, body: e.target.value })}
                      placeholder="본문"
                      maxLength={500}
                      rows={2}
                      className="w-full bg-white border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
                    />
                    <input
                      value={editForm.link}
                      onChange={(e) => setEditForm({ ...editForm, link: e.target.value })}
                      placeholder="링크 URL (선택)"
                      className="w-full bg-white border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
                    />
                    <input
                      type="datetime-local"
                      value={editForm.scheduled_at}
                      onChange={(e) => setEditForm({ ...editForm, scheduled_at: e.target.value })}
                      className="w-full bg-white border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
                    />
                    <div className="flex gap-2">
                      <button
                        onClick={() => handleUpdate(p.id)}
                        disabled={saving}
                        className="px-3 py-1.5 rounded-lg bg-[#4a9fe5]/20 text-[#4a9fe5] text-sm font-semibold hover:bg-[#4a9fe5]/30 disabled:opacity-50"
                      >
                        {saving ? "저장 중..." : "저장"}
                      </button>
                      <button
                        onClick={() => setEditingId(null)}
                        className="px-3 py-1.5 rounded-lg bg-gray-100 text-gray-500 text-sm hover:bg-gray-200"
                      >
                        취소
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex-1 min-w-0 space-y-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-semibold text-[#1e3a5f]">{p.title}</span>
                        <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-semibold ${STATUS_COLORS[p.status]}`}>
                          {STATUS_LABELS[p.status]}
                        </span>
                      </div>
                      <p className="text-xs text-[#6b8db5]">{p.body}</p>
                      {p.link && (
                        <p className="text-xs text-[#4a9fe5] truncate">{p.link}</p>
                      )}
                      <div className="flex items-center gap-3 text-xs text-[#94a3b8]">
                        {p.scheduled_at && (
                          <span>예약: {new Date(p.scheduled_at).toLocaleString("ko-KR")}</span>
                        )}
                        {p.sent_at && (
                          <span>전송: {new Date(p.sent_at).toLocaleString("ko-KR")}</span>
                        )}
                        <span>생성: {new Date(p.created_at).toLocaleDateString("ko-KR")}</span>
                      </div>
                      {p.error_message && (
                        <p className="text-xs text-[#ff5442]">오류: {p.error_message}</p>
                      )}
                    </div>
                    {p.status === "pending" && (
                      <div className="flex gap-1.5 flex-shrink-0">
                        <button
                          onClick={() => startEdit(p)}
                          className="text-xs px-2 py-1 rounded text-[#6b8db5] hover:text-[#1e3a5f] hover:bg-[#d4e6f5] transition-colors"
                        >
                          수정
                        </button>
                        <button
                          onClick={() => handleExecute(p.id)}
                          disabled={actioningId === p.id}
                          className="text-xs px-2 py-1 rounded bg-green-100 text-green-700 hover:bg-green-200 transition-colors disabled:opacity-50"
                        >
                          즉시 전송
                        </button>
                        <button
                          onClick={() => handleDelete(p.id)}
                          disabled={actioningId === p.id}
                          className="text-xs px-2 py-1 rounded bg-[#ff5442]/10 text-[#ff5442] hover:bg-[#ff5442]/20 transition-colors disabled:opacity-50"
                        >
                          취소
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
