import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { getActiveTerms, completeSignup, type Term } from "@/lib/api";
import { useAuth } from "@/contexts/auth-context";
import { LOGIN_REDIRECT_KEY } from "@/components/kakao-login-button";

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
        const sorted = [...list].sort((a, b) => {
          if (a.required !== b.required) return a.required ? -1 : 1;
          return new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
        });
        setTerms(sorted);
        setLoading(false);
      })
      .catch(() => {
        setError("약관 목록을 불러올 수 없습니다");
        setLoading(false);
      });
  }, [signupKey, navigate]);

  const requiredIds = terms.filter((t) => t.required).map((t) => t.id);
  const allRequiredAgreed = requiredIds.every((id) => agreed.has(id));

  const toggleTerm = (id: string) => {
    setAgreed((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleSubmit = async () => {
    if (!signupKey || !allRequiredAgreed) return;
    setSubmitting(true);
    setError(null);
    try {
      await completeSignup(signupKey, Array.from(agreed));
      await refreshUser();
      const redirectPath = localStorage.getItem(LOGIN_REDIRECT_KEY);
      localStorage.removeItem(LOGIN_REDIRECT_KEY);
      navigate(redirectPath || "/", { replace: true });
    } catch (e) {
      setError(e instanceof Error ? e.message : "회원가입에 실패했습니다");
      setSubmitting(false);
    }
  };

  const termUrl = (term: Term): string | null => {
    if (!term.url) return null;
    return term.url.match(/^https?:\/\//) ? term.url : `https://${term.url}`;
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
      <div className="w-full max-w-md bg-white rounded-[20px] border-[2.5px] border-[#231815] px-8 py-10 space-y-8">
        <h1 className="text-3xl font-semibold text-[#231815] tracking-tight text-center">
          위즈레터 시작하기
        </h1>

        {error && (
          <div className="rounded-xl border border-[#ff5442]/30 bg-[#ff5442]/10 p-4 text-sm text-[#ff5442]">
            {error}
          </div>
        )}

        <div className="space-y-5 px-2 md:px-4">
          {terms.map((term) => {
            const url = termUrl(term);
            return (
              <label
                key={term.id}
                className="flex items-start gap-3 cursor-pointer"
              >
                <input
                  type="checkbox"
                  checked={agreed.has(term.id)}
                  onChange={() => toggleTerm(term.id)}
                  className="w-[22px] h-[22px] rounded-md border-[#a8a8a8] accent-[#231815] mt-0.5 flex-shrink-0"
                />
                <div className="flex flex-col">
                  <span className="text-sm text-[#231815] leading-snug">
                    {url ? (
                      <a
                        href={url}
                        target="_blank"
                        rel="noopener noreferrer"
                        onClick={(e) => e.stopPropagation()}
                        className="underline hover:opacity-70"
                      >
                        {term.title}
                      </a>
                    ) : (
                      term.title
                    )}{" "}
                    동의{" "}
                    <span className="text-[#525252]">
                      ({term.required ? "필수" : "선택"})
                    </span>
                  </span>
                  {term.description && (
                    <span className="text-xs text-[#525252] mt-0.5">
                      {term.description}
                    </span>
                  )}
                </div>
              </label>
            );
          })}
        </div>

        <div className="space-y-3">
          <button
            onClick={handleSubmit}
            disabled={!allRequiredAgreed || submitting}
            className="w-full py-3.5 rounded-xl font-semibold text-white transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            style={{ background: "var(--color-button-primary)" }}
          >
            {submitting ? "처리 중..." : "동의하고 가입하기"}
          </button>

          <button
            onClick={() => navigate("/")}
            className="w-full py-3.5 rounded-xl font-semibold text-[#525252] border border-[#a8a8a8] bg-white hover:bg-[#f5f5f5] transition-colors"
          >
            돌아가기
          </button>
        </div>

        {!allRequiredAgreed && terms.length > 0 && (
          <p className="text-xs text-center text-[#ff5442]">
            필수 약관에 모두 동의해주세요.
          </p>
        )}
      </div>
    </div>
  );
}
