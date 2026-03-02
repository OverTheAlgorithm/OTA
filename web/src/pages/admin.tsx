import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { useTheme } from "@/contexts/theme-context";
import {
  triggerCollection,
  sendTestEmail,
  getBrainCategories,
  createBrainCategory,
  updateBrainCategory,
  deleteBrainCategory,
  type BrainCategory,
  type TestEmailResult,
} from "@/lib/api";

type CollectState =
  | { status: "idle" }
  | { status: "running" }
  | { status: "requested" }
  | { status: "error"; message: string };

function BrainCategoryManager() {
  const [categories, setCategories] = useState<BrainCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editForm, setEditForm] = useState({ emoji: "", label: "", accent_color: "", display_order: 0 });
  const [showNew, setShowNew] = useState(false);
  const [newForm, setNewForm] = useState({ key: "", emoji: "", label: "", accent_color: "#6b8db5", display_order: 0 });
  const [error, setError] = useState<string | null>(null);

  const load = () => {
    setLoading(true);
    getBrainCategories()
      .then(setCategories)
      .catch(() => setError("불러오기 실패"))
      .finally(() => setLoading(false));
  };

  useEffect(() => { load(); }, []);

  const startEdit = (bc: BrainCategory) => {
    setEditingKey(bc.key);
    setEditForm({ emoji: bc.emoji, label: bc.label, accent_color: bc.accent_color, display_order: bc.display_order });
  };

  const saveEdit = async () => {
    if (!editingKey) return;
    setError(null);
    try {
      await updateBrainCategory(editingKey, editForm);
      setEditingKey(null);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "수정 실패");
    }
  };

  const handleDelete = async (key: string) => {
    if (!confirm(`"${key}" 카테고리를 삭제하시겠습니까?`)) return;
    setError(null);
    try {
      await deleteBrainCategory(key);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "삭제 실패");
    }
  };

  const handleCreate = async () => {
    if (!newForm.key || !newForm.emoji || !newForm.label) {
      setError("key, emoji, label은 필수입니다");
      return;
    }
    setError(null);
    try {
      await createBrainCategory(newForm);
      setShowNew(false);
      setNewForm({ key: "", emoji: "", label: "", accent_color: "#6b8db5", display_order: 0 });
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "생성 실패");
    }
  };

  if (loading) {
    return <p className="text-sm text-[#6b8db5]">불러오는 중...</p>;
  }

  return (
    <div className="space-y-3">
      {error && (
        <div className="rounded-xl border border-[#ff5442]/30 bg-[#ff5442]/10 p-3 text-sm text-[#ff5442]">
          {error}
        </div>
      )}

      {categories.map((bc) => (
        <div key={bc.key} className="rounded-xl border border-[#d4e6f5] bg-[#f0f7ff] p-4">
          {editingKey === bc.key ? (
            <div className="space-y-2">
              <div className="flex gap-2">
                <input
                  value={editForm.emoji}
                  onChange={(e) => setEditForm({ ...editForm, emoji: e.target.value })}
                  className="w-12 bg-white border border-[#d4e6f5] rounded px-2 py-1 text-sm text-[#1e3a5f]"
                  placeholder="emoji"
                />
                <input
                  value={editForm.label}
                  onChange={(e) => setEditForm({ ...editForm, label: e.target.value })}
                  className="flex-1 bg-white border border-[#d4e6f5] rounded px-2 py-1 text-sm text-[#1e3a5f]"
                  placeholder="label"
                />
              </div>
              <div className="flex gap-2">
                <input
                  value={editForm.accent_color}
                  onChange={(e) => setEditForm({ ...editForm, accent_color: e.target.value })}
                  className="w-24 bg-white border border-[#d4e6f5] rounded px-2 py-1 text-sm text-[#1e3a5f]"
                  placeholder="color"
                />
                <input
                  type="number"
                  value={editForm.display_order}
                  onChange={(e) => setEditForm({ ...editForm, display_order: parseInt(e.target.value) || 0 })}
                  className="w-16 bg-white border border-[#d4e6f5] rounded px-2 py-1 text-sm text-[#1e3a5f]"
                  placeholder="순서"
                />
                <button onClick={saveEdit} className="px-3 py-1 rounded bg-[#4a9fe5]/20 text-[#4a9fe5] text-xs font-semibold hover:bg-[#4a9fe5]/30 transition-colors">
                  저장
                </button>
                <button onClick={() => setEditingKey(null)} className="px-3 py-1 rounded bg-[#d4e6f5] text-[#6b8db5] text-xs hover:text-[#1e3a5f] transition-colors">
                  취소
                </button>
              </div>
            </div>
          ) : (
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <span className="text-lg">{bc.emoji}</span>
                <div>
                  <p className="text-sm font-semibold text-[#1e3a5f]">{bc.label}</p>
                  <p className="text-xs text-[#6b8db5]">
                    key: <code className="text-[#4a9fe5]">{bc.key}</code>
                    {" · "}순서: {bc.display_order}
                    {" · "}
                    <span style={{ color: bc.accent_color }}>■</span> {bc.accent_color}
                  </p>
                </div>
              </div>
              <div className="flex gap-2">
                <button
                  onClick={() => startEdit(bc)}
                  className="px-2.5 py-1 rounded text-xs text-[#6b8db5] hover:text-[#1e3a5f] hover:bg-[#d4e6f5] transition-colors"
                >
                  수정
                </button>
                <button
                  onClick={() => handleDelete(bc.key)}
                  className="px-2.5 py-1 rounded text-xs text-[#ff5442]/60 hover:text-[#ff5442] hover:bg-[#ff5442]/10 transition-colors"
                >
                  삭제
                </button>
              </div>
            </div>
          )}
        </div>
      ))}

      {showNew ? (
        <div className="rounded-xl border border-[#4a9fe5]/30 bg-white p-4 space-y-2">
          <div className="flex gap-2">
            <input
              value={newForm.key}
              onChange={(e) => setNewForm({ ...newForm, key: e.target.value })}
              className="w-28 bg-[#f0f7ff] border border-[#d4e6f5] rounded px-2 py-1 text-sm text-[#1e3a5f]"
              placeholder="key"
            />
            <input
              value={newForm.emoji}
              onChange={(e) => setNewForm({ ...newForm, emoji: e.target.value })}
              className="w-12 bg-[#f0f7ff] border border-[#d4e6f5] rounded px-2 py-1 text-sm text-[#1e3a5f]"
              placeholder="emoji"
            />
            <input
              value={newForm.label}
              onChange={(e) => setNewForm({ ...newForm, label: e.target.value })}
              className="flex-1 bg-[#f0f7ff] border border-[#d4e6f5] rounded px-2 py-1 text-sm text-[#1e3a5f]"
              placeholder="label"
            />
          </div>
          <div className="flex gap-2">
            <input
              value={newForm.accent_color}
              onChange={(e) => setNewForm({ ...newForm, accent_color: e.target.value })}
              className="w-24 bg-[#f0f7ff] border border-[#d4e6f5] rounded px-2 py-1 text-sm text-[#1e3a5f]"
              placeholder="color"
            />
            <input
              type="number"
              value={newForm.display_order}
              onChange={(e) => setNewForm({ ...newForm, display_order: parseInt(e.target.value) || 0 })}
              className="w-16 bg-[#f0f7ff] border border-[#d4e6f5] rounded px-2 py-1 text-sm text-[#1e3a5f]"
              placeholder="순서"
            />
            <button onClick={handleCreate} className="px-3 py-1 rounded bg-[#4a9fe5]/20 text-[#4a9fe5] text-xs font-semibold hover:bg-[#4a9fe5]/30 transition-colors">
              추가
            </button>
            <button onClick={() => setShowNew(false)} className="px-3 py-1 rounded bg-[#d4e6f5] text-[#6b8db5] text-xs hover:text-[#1e3a5f] transition-colors">
              취소
            </button>
          </div>
        </div>
      ) : (
        <button
          onClick={() => setShowNew(true)}
          className="w-full py-2.5 rounded-xl border border-dashed border-[#d4e6f5] text-sm text-[#6b8db5] hover:text-[#1e3a5f] hover:border-[#6b8db5] transition-colors"
        >
          + 새 카테고리 추가
        </button>
      )}
    </div>
  );
}

export function AdminPage() {
  const { user, loading } = useAuth();
  const { toggleTheme } = useTheme();
  const navigate = useNavigate();
  const [collectState, setCollectState] = useState<CollectState>({ status: "idle" });

  type TestEmailState =
    | { status: "idle" }
    | { status: "sending" }
    | { status: "done"; result: TestEmailResult }
    | { status: "error"; message: string };
  const [testEmailState, setTestEmailState] = useState<TestEmailState>({ status: "idle" });

  const handleTestEmail = async () => {
    setTestEmailState({ status: "sending" });
    try {
      const result = await sendTestEmail();
      setTestEmailState({ status: "done", result });
    } catch (e) {
      setTestEmailState({ status: "error", message: e instanceof Error ? e.message : "알 수 없는 오류" });
    }
  };

  useEffect(() => {
    if (loading) return;
    if (!user) { navigate("/", { replace: true }); return; }
    if (user.role !== "admin") { navigate("/home", { replace: true }); return; }
  }, [user, loading, navigate]);

  if (loading || !user || user.role !== "admin") {
    return (
      <div className="min-h-screen flex items-center justify-center bg-white">
        <p className="text-[#6b8db5]">로딩 중...</p>
      </div>
    );
  }

  const handleCollect = async () => {
    setCollectState({ status: "running" });
    try {
      await triggerCollection();
      setCollectState({ status: "requested" });
    } catch (e) {
      setCollectState({ status: "error", message: e instanceof Error ? e.message : "알 수 없는 오류" });
    }
  };

  const isDisabled = collectState.status === "running" || collectState.status === "requested";

  return (
    <div
      className="min-h-screen p-8"
      style={{
        backgroundColor: "var(--color-bg)",
        color: "var(--color-fg)"
      }}
    >
      <div className="max-w-xl mx-auto space-y-8">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">관리자 페이지</h1>
          <div className="flex items-center gap-3">
            <button
              onClick={toggleTheme}
              className="text-sm text-[#6b8db5] hover:text-[#1e3a5f] transition-colors"
              title="테마 전환"
            >
              🌙
            </button>
            <button
              onClick={() => navigate("/home")}
              className="text-sm text-[#6b8db5] hover:text-[#1e3a5f] transition-colors"
            >
              ← 홈으로
            </button>
          </div>
        </div>

        <section className="rounded-2xl border border-[#d4e6f5] bg-[#f0f7ff] p-6 space-y-4">
          <h2 className="text-lg font-semibold">데이터 수집</h2>
          <p className="text-sm text-[#6b8db5]">
            AI를 통해 오늘의 한국 트렌드를 즉시 수집합니다. 이 작업은 1시간까지 소요될 수 있습니다.
          </p>

          <button
            onClick={handleCollect}
            disabled={isDisabled}
            className="w-full py-3 rounded-xl font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            style={{ background: "var(--color-button-primary)", color: "white" }}
          >
            {collectState.status === "running" && "수집 중..."}
            {collectState.status === "requested" && "수집 요청 완료"}
            {(collectState.status === "idle" || collectState.status === "error") && "수집 실행"}
          </button>

          {collectState.status === "requested" && (
            <p className="text-xs text-[#6b8db5]">작업 완료시 슬랙으로 통지합니다.</p>
          )}

          {collectState.status === "error" && (
            <div className="rounded-xl border border-[#ff5442]/30 bg-[#ff5442]/10 p-4 text-sm">
              <p className="text-[#ff5442] font-semibold">수집 실패</p>
              <p className="text-[#6b8db5] mt-1">{collectState.message}</p>
            </div>
          )}
        </section>

        <section className="rounded-2xl border border-[#d4e6f5] bg-[#f0f7ff] p-6 space-y-4">
          <h2 className="text-lg font-semibold">테스트 이메일</h2>
          <p className="text-sm text-[#6b8db5]">
            최신 브리핑을 내 이메일로 즉시 전송합니다. 이메일 채널이 활성화되어 있어야 합니다.
          </p>

          <button
            onClick={handleTestEmail}
            disabled={testEmailState.status === "sending"}
            className="w-full py-3 rounded-xl font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            style={{ background: "var(--color-button-primary)", color: "white" }}
          >
            {testEmailState.status === "sending" ? "전송 중..." : "테스트 이메일 전송하기"}
          </button>

          {testEmailState.status === "done" && (
            <div className="rounded-xl border border-green-300 bg-green-100 p-4 text-sm space-y-1">
              {testEmailState.result.success_count > 0 && (
                <p className="text-green-700 font-semibold">✓ 이메일이 전송됐습니다</p>
              )}
              {testEmailState.result.skipped_count > 0 && (
                <p className="text-[#6b8db5]">이미 전송된 브리핑입니다 (중복 방지)</p>
              )}
              {testEmailState.result.failure_count > 0 && (
                <p className="text-[#ff5442]">전송 실패 — {Object.values(testEmailState.result.errors).join(", ")}</p>
              )}
            </div>
          )}

          {testEmailState.status === "error" && (
            <div className="rounded-xl border border-[#ff5442]/30 bg-[#ff5442]/10 p-4 text-sm">
              <p className="text-[#ff5442] font-semibold">전송 실패</p>
              <p className="text-[#6b8db5] mt-1">{testEmailState.message}</p>
            </div>
          )}
        </section>

        <section className="rounded-2xl border border-[#d4e6f5] bg-[#f0f7ff] p-6 space-y-4">
          <h2 className="text-lg font-semibold">Brain Category 관리</h2>
          <p className="text-sm text-[#6b8db5]">
            각 토픽에 부여되는 행동 지침 라벨입니다. AI가 수집 시 이 목록에서 선택합니다.
          </p>
          <BrainCategoryManager />
        </section>
      </div>
    </div>
  );
}
