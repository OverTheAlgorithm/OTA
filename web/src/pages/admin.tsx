import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import {
  triggerCollection,
  getBrainCategories,
  createBrainCategory,
  updateBrainCategory,
  deleteBrainCategory,
  type BrainCategory,
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
  const [newForm, setNewForm] = useState({ key: "", emoji: "", label: "", accent_color: "#9b8bb4", display_order: 0 });
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
      setNewForm({ key: "", emoji: "", label: "", accent_color: "#9b8bb4", display_order: 0 });
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "생성 실패");
    }
  };

  if (loading) {
    return <p className="text-sm text-[#9b8bb4]">불러오는 중...</p>;
  }

  return (
    <div className="space-y-3">
      {error && (
        <div className="rounded-xl border border-red-900 bg-[#0f0a19] p-3 text-sm text-red-400">
          {error}
        </div>
      )}

      {categories.map((bc) => (
        <div key={bc.key} className="rounded-xl border border-[#2d1f42] bg-[#0f0a19] p-4">
          {editingKey === bc.key ? (
            <div className="space-y-2">
              <div className="flex gap-2">
                <input
                  value={editForm.emoji}
                  onChange={(e) => setEditForm({ ...editForm, emoji: e.target.value })}
                  className="w-12 bg-[#1a1229] border border-[#2d1f42] rounded px-2 py-1 text-sm text-[#f5f0ff]"
                  placeholder="emoji"
                />
                <input
                  value={editForm.label}
                  onChange={(e) => setEditForm({ ...editForm, label: e.target.value })}
                  className="flex-1 bg-[#1a1229] border border-[#2d1f42] rounded px-2 py-1 text-sm text-[#f5f0ff]"
                  placeholder="label"
                />
              </div>
              <div className="flex gap-2">
                <input
                  value={editForm.accent_color}
                  onChange={(e) => setEditForm({ ...editForm, accent_color: e.target.value })}
                  className="w-24 bg-[#1a1229] border border-[#2d1f42] rounded px-2 py-1 text-sm text-[#f5f0ff]"
                  placeholder="color"
                />
                <input
                  type="number"
                  value={editForm.display_order}
                  onChange={(e) => setEditForm({ ...editForm, display_order: parseInt(e.target.value) || 0 })}
                  className="w-16 bg-[#1a1229] border border-[#2d1f42] rounded px-2 py-1 text-sm text-[#f5f0ff]"
                  placeholder="순서"
                />
                <button onClick={saveEdit} className="px-3 py-1 rounded bg-[#5ba4d9]/20 text-[#5ba4d9] text-xs font-semibold hover:bg-[#5ba4d9]/30 transition-colors">
                  저장
                </button>
                <button onClick={() => setEditingKey(null)} className="px-3 py-1 rounded bg-[#2d1f42] text-[#9b8bb4] text-xs hover:text-[#f5f0ff] transition-colors">
                  취소
                </button>
              </div>
            </div>
          ) : (
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <span className="text-lg">{bc.emoji}</span>
                <div>
                  <p className="text-sm font-semibold text-[#f5f0ff]">{bc.label}</p>
                  <p className="text-xs text-[#9b8bb4]">
                    key: <code className="text-[#5ba4d9]">{bc.key}</code>
                    {" · "}순서: {bc.display_order}
                    {" · "}
                    <span style={{ color: bc.accent_color }}>■</span> {bc.accent_color}
                  </p>
                </div>
              </div>
              <div className="flex gap-2">
                <button
                  onClick={() => startEdit(bc)}
                  className="px-2.5 py-1 rounded text-xs text-[#9b8bb4] hover:text-[#f5f0ff] hover:bg-[#2d1f42] transition-colors"
                >
                  수정
                </button>
                <button
                  onClick={() => handleDelete(bc.key)}
                  className="px-2.5 py-1 rounded text-xs text-red-400/60 hover:text-red-400 hover:bg-red-900/20 transition-colors"
                >
                  삭제
                </button>
              </div>
            </div>
          )}
        </div>
      ))}

      {showNew ? (
        <div className="rounded-xl border border-[#5ba4d9]/30 bg-[#0f0a19] p-4 space-y-2">
          <div className="flex gap-2">
            <input
              value={newForm.key}
              onChange={(e) => setNewForm({ ...newForm, key: e.target.value })}
              className="w-28 bg-[#1a1229] border border-[#2d1f42] rounded px-2 py-1 text-sm text-[#f5f0ff]"
              placeholder="key"
            />
            <input
              value={newForm.emoji}
              onChange={(e) => setNewForm({ ...newForm, emoji: e.target.value })}
              className="w-12 bg-[#1a1229] border border-[#2d1f42] rounded px-2 py-1 text-sm text-[#f5f0ff]"
              placeholder="emoji"
            />
            <input
              value={newForm.label}
              onChange={(e) => setNewForm({ ...newForm, label: e.target.value })}
              className="flex-1 bg-[#1a1229] border border-[#2d1f42] rounded px-2 py-1 text-sm text-[#f5f0ff]"
              placeholder="label"
            />
          </div>
          <div className="flex gap-2">
            <input
              value={newForm.accent_color}
              onChange={(e) => setNewForm({ ...newForm, accent_color: e.target.value })}
              className="w-24 bg-[#1a1229] border border-[#2d1f42] rounded px-2 py-1 text-sm text-[#f5f0ff]"
              placeholder="color"
            />
            <input
              type="number"
              value={newForm.display_order}
              onChange={(e) => setNewForm({ ...newForm, display_order: parseInt(e.target.value) || 0 })}
              className="w-16 bg-[#1a1229] border border-[#2d1f42] rounded px-2 py-1 text-sm text-[#f5f0ff]"
              placeholder="순서"
            />
            <button onClick={handleCreate} className="px-3 py-1 rounded bg-[#5ba4d9]/20 text-[#5ba4d9] text-xs font-semibold hover:bg-[#5ba4d9]/30 transition-colors">
              추가
            </button>
            <button onClick={() => setShowNew(false)} className="px-3 py-1 rounded bg-[#2d1f42] text-[#9b8bb4] text-xs hover:text-[#f5f0ff] transition-colors">
              취소
            </button>
          </div>
        </div>
      ) : (
        <button
          onClick={() => setShowNew(true)}
          className="w-full py-2.5 rounded-xl border border-dashed border-[#2d1f42] text-sm text-[#9b8bb4] hover:text-[#f5f0ff] hover:border-[#9b8bb4] transition-colors"
        >
          + 새 카테고리 추가
        </button>
      )}
    </div>
  );
}

export function AdminPage() {
  const { user, loading } = useAuth();
  const navigate = useNavigate();
  const [collectState, setCollectState] = useState<CollectState>({ status: "idle" });

  useEffect(() => {
    if (loading) return;
    if (!user) { navigate("/", { replace: true }); return; }
    if (user.role !== "admin") { navigate("/home", { replace: true }); return; }
  }, [user, loading, navigate]);

  if (loading || !user || user.role !== "admin") {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#0f0a19]">
        <p className="text-[#9b8bb4]">로딩 중...</p>
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
    <div className="min-h-screen bg-[#0f0a19] text-[#f5f0ff] p-8">
      <div className="max-w-xl mx-auto space-y-8">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">관리자 페이지</h1>
          <button
            onClick={() => navigate("/home")}
            className="text-sm text-[#9b8bb4] hover:text-[#f5f0ff] transition-colors"
          >
            ← 홈으로
          </button>
        </div>

        <section className="rounded-2xl border border-[#2d1f42] bg-[#1a1229] p-6 space-y-4">
          <h2 className="text-lg font-semibold">데이터 수집</h2>
          <p className="text-sm text-[#9b8bb4]">
            AI를 통해 오늘의 한국 트렌드를 즉시 수집합니다. 이 작업은 1시간까지 소요될 수 있습니다.
          </p>

          <button
            onClick={handleCollect}
            disabled={isDisabled}
            className="w-full py-3 rounded-xl font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            style={{ background: "#2d1f42", color: "#f5f0ff" }}
          >
            {collectState.status === "running" && "수집 중..."}
            {collectState.status === "requested" && "수집 요청 완료"}
            {(collectState.status === "idle" || collectState.status === "error") && "수집 실행"}
          </button>

          {collectState.status === "requested" && (
            <p className="text-xs text-[#9b8bb4]">작업 완료시 슬랙으로 통지합니다.</p>
          )}

          {collectState.status === "error" && (
            <div className="rounded-xl border border-red-900 bg-[#0f0a19] p-4 text-sm">
              <p className="text-red-400 font-semibold">수집 실패</p>
              <p className="text-[#9b8bb4] mt-1">{collectState.message}</p>
            </div>
          )}
        </section>

        <section className="rounded-2xl border border-[#2d1f42] bg-[#1a1229] p-6 space-y-4">
          <h2 className="text-lg font-semibold">Brain Category 관리</h2>
          <p className="text-sm text-[#9b8bb4]">
            각 토픽에 부여되는 행동 지침 라벨입니다. AI가 수집 시 이 목록에서 선택합니다.
          </p>
          <BrainCategoryManager />
        </section>
      </div>
    </div>
  );
}
