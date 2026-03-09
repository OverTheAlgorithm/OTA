import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { getActiveTerms, completeSignup, type Term } from "@/lib/api";
import { useAuth } from "@/contexts/auth-context";

export function TermsConsentPage() {
  const [searchParams] = useSearchParams();
  const signupKey = searchParams.get("signup_key");
  const navigate = useNavigate();
  const { refreshUser } = useAuth();

  const [terms, setTerms] = useState<Term[]>([]);
  const [agreed, setAgreed] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!signupKey) {
      navigate("/", { replace: true });
      return;
    }
    getActiveTerms()
      .then((list) => {
        setTerms(list);
        setLoading(false);
      })
      .catch(() => {
        setError("약관 목록을 불러올 수 없습니다");
        setLoading(false);
      });
  }, [signupKey, navigate]);

  const requiredIds = terms.filter((t) => t.required).map((t) => t.id);
  const allRequiredAgreed = requiredIds.every((id) => agreed.has(id));
  const allAgreed = terms.length > 0 && terms.every((t) => agreed.has(t.id));

  const toggleTerm = (id: string) => {
    setAgreed((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const toggleAll = () => {
    if (allAgreed) {
      setAgreed(new Set());
    } else {
      setAgreed(new Set(terms.map((t) => t.id)));
    }
  };

  const handleSubmit = async () => {
    if (!signupKey || !allRequiredAgreed) return;
    setSubmitting(true);
    setError(null);
    try {
      await completeSignup(signupKey, Array.from(agreed));
      await refreshUser();
      navigate("/home", { replace: true });
    } catch (e) {
      setError(e instanceof Error ? e.message : "회원가입에 실패했습니다");
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-white">
        <p className="text-[#6b8db5]">불러오는 중...</p>
      </div>
    );
  }

  return (
    <div
      className="min-h-screen flex items-center justify-center p-4"
      style={{ backgroundColor: "var(--color-bg)" }}
    >
      <div className="w-full max-w-md space-y-6">
        <div className="text-center space-y-2">
          <h1 className="text-2xl font-bold text-[#1e3a5f]">
            서비스 이용 약관
          </h1>
          <p className="text-sm text-[#6b8db5]">
            서비스 이용을 위해 아래 약관에 동의해주세요.
          </p>
        </div>

        {error && (
          <div className="rounded-xl border border-[#ff5442]/30 bg-[#ff5442]/10 p-4 text-sm text-[#ff5442]">
            {error}
          </div>
        )}

        <div className="rounded-2xl border border-[#d4e6f5] bg-[#f0f7ff] overflow-hidden">
          {/* Toggle all */}
          <label className="flex items-center gap-3 px-5 py-4 border-b border-[#d4e6f5] cursor-pointer hover:bg-[#e8f4fd] transition-colors">
            <input
              type="checkbox"
              checked={allAgreed}
              onChange={toggleAll}
              className="w-5 h-5 rounded accent-[#4a9fe5]"
            />
            <span className="text-sm font-semibold text-[#1e3a5f]">
              전체 동의
            </span>
          </label>

          {/* Individual terms */}
          {terms.map((term) => (
            <label
              key={term.id}
              className="flex items-start gap-3 px-5 py-3.5 border-b border-[#d4e6f5] last:border-b-0 cursor-pointer hover:bg-[#e8f4fd] transition-colors"
            >
              <input
                type="checkbox"
                checked={agreed.has(term.id)}
                onChange={() => toggleTerm(term.id)}
                className="w-5 h-5 rounded accent-[#4a9fe5] mt-0.5 flex-shrink-0"
              />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm text-[#1e3a5f]">
                    {term.title}
                  </span>
                  <span className="text-[10px] text-[#94a3b8]">
                    v{term.version}
                  </span>
                  <span
                    className={`text-[10px] px-1.5 py-0.5 rounded-full font-semibold ${
                      term.required
                        ? "bg-[#ff5442]/10 text-[#ff5442]"
                        : "bg-[#d4e6f5] text-[#6b8db5]"
                    }`}
                  >
                    {term.required ? "필수" : "선택"}
                  </span>
                </div>
                {term.description && (
                  <p className="text-xs text-[#6b8db5] mt-0.5">
                    {term.description}
                  </p>
                )}
                <a
                  href={term.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  onClick={(e) => e.stopPropagation()}
                  className="text-xs text-[#4a9fe5] hover:underline mt-0.5 inline-block"
                >
                  전문 보기
                </a>
              </div>
            </label>
          ))}
        </div>

        <button
          onClick={handleSubmit}
          disabled={!allRequiredAgreed || submitting}
          className="w-full py-3.5 rounded-xl font-semibold text-white transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          style={{ background: "var(--color-button-primary)" }}
        >
          {submitting ? "처리 중..." : "동의하고 가입하기"}
        </button>

        {!allRequiredAgreed && terms.length > 0 && (
          <p className="text-xs text-center text-[#ff5442]">
            필수 약관에 모두 동의해주세요.
          </p>
        )}
      </div>
    </div>
  );
}
