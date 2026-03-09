import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { getAdminTerms, createTerm, updateTermActive, type Term } from "@/lib/api";

export function AdminTermsPage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();

  const [terms, setTerms] = useState<Term[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [togglingId, setTogglingId] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading) return;
    if (!user) { navigate("/", { replace: true }); return; }
    if (user.role !== "admin") { navigate("/home", { replace: true }); return; }
    loadTerms();
  }, [user, authLoading, navigate]);

  const loadTerms = () => {
    setLoading(true);
    setError(null);
    getAdminTerms()
      .then(setTerms)
      .catch((e) => setError(e instanceof Error ? e.message : "불러오기 실패"))
      .finally(() => setLoading(false));
  };

  const handleToggleActive = async (term: Term) => {
    setTogglingId(term.id);
    try {
      await updateTermActive(term.id, !term.active);
      setTerms((prev) =>
        prev.map((t) => (t.id === term.id ? { ...t, active: !t.active } : t))
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "상태 변경 실패");
    } finally {
      setTogglingId(null);
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
      <div className="max-w-3xl mx-auto space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">이용 약관 관리</h1>
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

        {/* Create form toggle */}
        <div className="flex justify-end">
          <button
            onClick={() => setShowCreate(!showCreate)}
            className="px-4 py-2 rounded-xl font-semibold text-sm text-white transition-colors"
            style={{ background: "var(--color-button-primary)" }}
          >
            {showCreate ? "취소" : "새 약관 만들기"}
          </button>
        </div>

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
          <p className="text-sm text-[#6b8db5]">불러오는 중...</p>
        ) : terms.length === 0 ? (
          <p className="text-sm text-[#6b8db5]">등록된 약관이 없습니다.</p>
        ) : (
          <div className="space-y-3">
            {terms.map((t) => (
              <div
                key={t.id}
                className="rounded-xl border border-[#d4e6f5] bg-[#f0f7ff] p-4"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-sm font-semibold text-[#1e3a5f]">
                        {t.title}
                      </span>
                      <span className="text-xs text-[#94a3b8]">
                        v{t.version}
                      </span>
                      <button
                        onClick={() => handleToggleActive(t)}
                        disabled={togglingId === t.id}
                        className={`text-[10px] px-1.5 py-0.5 rounded-full font-semibold cursor-pointer transition-colors disabled:opacity-50 ${
                          t.active
                            ? "bg-green-100 text-green-700 hover:bg-green-200"
                            : "bg-gray-100 text-gray-500 hover:bg-gray-200"
                        }`}
                      >
                        {t.active ? "활성" : "비활성"}
                      </button>
                      <span
                        className={`text-[10px] px-1.5 py-0.5 rounded-full font-semibold ${
                          t.required
                            ? "bg-[#ff5442]/10 text-[#ff5442]"
                            : "bg-[#d4e6f5] text-[#6b8db5]"
                        }`}
                      >
                        {t.required ? "필수" : "선택"}
                      </span>
                    </div>
                    {t.description && (
                      <p className="text-xs text-[#6b8db5] mt-1">
                        {t.description}
                      </p>
                    )}
                    <div className="flex items-center gap-3 mt-1.5 text-xs text-[#94a3b8]">
                      <a
                        href={t.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-[#4a9fe5] hover:underline"
                      >
                        약관 링크
                      </a>
                      <span>
                        {new Date(t.created_at).toLocaleDateString("ko-KR")}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

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
    if (!form.title || !form.url || !form.version) {
      setError("제목, URL, 버전은 필수입니다");
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      await createTerm(form);
      onCreated();
    } catch (e) {
      setError(e instanceof Error ? e.message : "생성 실패");
      setSubmitting(false);
    }
  };

  return (
    <div className="rounded-xl border border-[#4a9fe5]/30 bg-white p-5 space-y-3">
      <h3 className="text-sm font-semibold text-[#1e3a5f]">새 약관 만들기</h3>

      {error && (
        <p className="text-xs text-[#ff5442]">{error}</p>
      )}

      <input
        value={form.title}
        onChange={(e) => setForm({ ...form, title: e.target.value })}
        placeholder="제목 (예: 개인정보 처리방침)"
        className="w-full bg-[#f0f7ff] border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
      />
      <textarea
        value={form.description}
        onChange={(e) => setForm({ ...form, description: e.target.value })}
        placeholder="설명 (선택)"
        rows={2}
        className="w-full bg-[#f0f7ff] border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
      />
      <input
        value={form.url}
        onChange={(e) => setForm({ ...form, url: e.target.value })}
        placeholder="약관 전문 URL (예: https://notion.so/...)"
        className="w-full bg-[#f0f7ff] border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
      />
      <input
        value={form.version}
        onChange={(e) => setForm({ ...form, version: e.target.value })}
        placeholder="버전 (예: 1 또는 1.2)"
        className="w-full bg-[#f0f7ff] border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
      />

      <div className="flex items-center gap-6">
        <label className="flex items-center gap-2 cursor-pointer">
          <input
            type="checkbox"
            checked={form.active}
            onChange={(e) => setForm({ ...form, active: e.target.checked })}
            className="w-4 h-4 rounded accent-[#4a9fe5]"
          />
          <span className="text-sm text-[#1e3a5f]">활성</span>
        </label>
        <label className="flex items-center gap-2 cursor-pointer">
          <input
            type="checkbox"
            checked={form.required}
            onChange={(e) => setForm({ ...form, required: e.target.checked })}
            className="w-4 h-4 rounded accent-[#4a9fe5]"
          />
          <span className="text-sm text-[#1e3a5f]">필수</span>
        </label>
      </div>

      <div className="flex gap-2">
        <button
          onClick={handleSubmit}
          disabled={submitting}
          className="px-4 py-2 rounded-lg bg-[#4a9fe5]/20 text-[#4a9fe5] text-sm font-semibold hover:bg-[#4a9fe5]/30 transition-colors disabled:opacity-50"
        >
          {submitting ? "생성 중..." : "생성"}
        </button>
      </div>
    </div>
  );
}
