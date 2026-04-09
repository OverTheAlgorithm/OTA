import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  getActiveTerms,
  completeSignup,
  sendVerificationCode,
  verifyEmailCode,
  type Term,
} from "@/lib/api";
import { useAuth } from "@/contexts/auth-context";
import { LOGIN_REDIRECT_KEY } from "@/components/kakao-login-button";
import { LoadingState } from "@/components/spinner";

type ModalStep = "none" | "email-input" | "code-input" | "skip-nudge";

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

  // Email verification modal state
  const [modalStep, setModalStep] = useState<ModalStep>("none");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [modalLoading, setModalLoading] = useState(false);
  const [modalError, setModalError] = useState("");

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

  const goToFinalDestination = () => {
    const redirectPath = localStorage.getItem(LOGIN_REDIRECT_KEY);
    localStorage.removeItem(LOGIN_REDIRECT_KEY);
    navigate(redirectPath || "/latest", { replace: true, state: { fromLogin: true } });
  };

  const handleSubmit = async () => {
    if (!signupKey || !allRequiredAgreed) return;
    setSubmitting(true);
    setError(null);
    try {
      await completeSignup(signupKey, Array.from(agreed));
      await refreshUser();
      setModalStep("email-input");
    } catch (e) {
      setError(e instanceof Error ? e.message : "회원가입에 실패했습니다");
      setSubmitting(false);
    }
  };

  const handleSendCode = async () => {
    const emailRegex = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/;
    if (!emailRegex.test(email)) {
      setModalError("올바른 이메일 형식을 입력해주세요");
      return;
    }
    setModalLoading(true);
    setModalError("");
    try {
      await sendVerificationCode(email);
      setModalStep("code-input");
    } catch (err) {
      setModalError(
        err instanceof Error ? err.message : "인증 코드 전송에 실패했습니다"
      );
    } finally {
      setModalLoading(false);
    }
  };

  const handleVerifyCode = async () => {
    if (code.length !== 6) {
      setModalError("6자리 인증 코드를 입력해주세요");
      return;
    }
    setModalLoading(true);
    setModalError("");
    try {
      await verifyEmailCode(code);
      await refreshUser();
      goToFinalDestination();
    } catch (err) {
      setModalError(
        err instanceof Error ? err.message : "인증 코드 확인에 실패했습니다"
      );
    } finally {
      setModalLoading(false);
    }
  };

  const termUrl = (term: Term): string | null => {
    if (!term.url) return null;
    return term.url.match(/^https?:\/\//) ? term.url : `https://${term.url}`;
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-white">
        <LoadingState label="불러오는 중" className="text-[#6b8db5]" />
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

      {/* Email verification modals */}
      {modalStep !== "none" && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          {/* Modal 1: Email Input */}
          {modalStep === "email-input" && (
            <div className="relative w-full max-w-[500px] bg-white rounded-[30px] px-10 py-12 md:px-14 md:py-16">
              <button
                onClick={() => setModalStep("skip-nudge")}
                className="absolute top-6 right-6 w-10 h-10 flex items-center justify-center rounded-full hover:bg-[#f5f5f5] transition-colors"
              >
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#231815" strokeWidth="2.5" strokeLinecap="round">
                  <path d="M18 6L6 18M6 6l12 12" />
                </svg>
              </button>

              <h2 className="text-4xl md:text-5xl font-semibold text-[#231815] tracking-tight leading-tight mb-8">
                이메일 인증하기
              </h2>

              <div className="space-y-6">
                <div>
                  <label htmlFor="signup-email" className="block text-sm font-medium text-[#231815] mb-2">
                    이메일 주소
                  </label>
                  <input
                    id="signup-email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="example@email.com"
                    className="w-full px-6 py-4 bg-white border border-[#bdc4cd] rounded-xl text-[#231815] text-base placeholder-[#96a0ad] focus:outline-none focus:border-[#43b9d6] transition-colors"
                    disabled={modalLoading}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") handleSendCode();
                    }}
                  />
                </div>

                {modalError && (
                  <p className="text-sm text-[#ff5442]">{modalError}</p>
                )}

                <button
                  onClick={handleSendCode}
                  disabled={modalLoading}
                  className="w-full py-4 rounded-full text-lg font-semibold bg-[#43b9d6] text-[#231815] border-[2px] border-[#231815] hover:brightness-110 disabled:opacity-50 transition-all"
                >
                  {modalLoading ? "전송 중..." : "인증번호 보내기"}
                </button>

                <button
                  onClick={() => setModalStep("skip-nudge")}
                  className="block mx-auto text-sm font-medium text-[#3d3d3d] hover:underline"
                >
                  건너뛰기
                </button>
              </div>
            </div>
          )}

          {/* Modal 2: Code Verification */}
          {modalStep === "code-input" && (
            <div className="relative w-full max-w-[500px] bg-white rounded-[30px] px-10 py-12 md:px-14 md:py-16">
              <button
                onClick={goToFinalDestination}
                className="absolute top-6 right-6 w-10 h-10 flex items-center justify-center rounded-full hover:bg-[#f5f5f5] transition-colors"
              >
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#231815" strokeWidth="2.5" strokeLinecap="round">
                  <path d="M18 6L6 18M6 6l12 12" />
                </svg>
              </button>

              <h2 className="text-4xl md:text-5xl font-semibold text-[#231815] tracking-tight leading-tight mb-4">
                이메일 인증하기
              </h2>

              <p className="text-[15px] text-[#231815] mb-8">
                {email}로 인증 코드를 전송했습니다
              </p>

              <div className="space-y-6">
                <div>
                  <label htmlFor="signup-code" className="block text-sm font-medium text-[#231815] mb-2">
                    인증 코드 (6자리)
                  </label>
                  <input
                    id="signup-code"
                    type="text"
                    value={code}
                    onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                    placeholder="000000"
                    className="w-full px-6 py-4 bg-white border border-[#bdc4cd] rounded-xl text-[#231815] text-base placeholder-[#96a0ad] focus:outline-none focus:border-[#43b9d6] transition-colors"
                    disabled={modalLoading}
                    maxLength={6}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") handleVerifyCode();
                    }}
                  />
                </div>

                {modalError && (
                  <p className="text-sm text-[#ff5442]">{modalError}</p>
                )}

                <div className="space-y-3">
                  <button
                    onClick={handleVerifyCode}
                    disabled={modalLoading || code.length !== 6}
                    className="w-full py-4 rounded-full text-lg font-semibold bg-[#43b9d6] text-[#231815] border-[2px] border-[#231815] hover:brightness-110 disabled:opacity-50 transition-all"
                  >
                    {modalLoading ? "확인 중..." : "인증 완료"}
                  </button>

                  <button
                    onClick={() => {
                      setModalStep("email-input");
                      setCode("");
                      setModalError("");
                    }}
                    disabled={modalLoading}
                    className="w-full py-4 rounded-full text-lg font-semibold bg-white text-[#231815] border-[2px] border-[#231815] hover:bg-[#f5f5f5] disabled:opacity-50 transition-all"
                  >
                    이메일 다시 입력하기
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Skip Nudge (Frame 1040) */}
          {modalStep === "skip-nudge" && (
            <div className="relative w-full max-w-[820px] bg-white rounded-[40px] border border-black/10 px-10 py-12 md:px-16 md:py-14">
              <button
                onClick={goToFinalDestination}
                className="absolute top-6 right-7 w-10 h-10 flex items-center justify-center rounded-full hover:bg-[#f5f5f5] transition-colors"
              >
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="#231815" strokeWidth="2.5" strokeLinecap="round">
                  <path d="M18 6L6 18M6 6l12 12" />
                </svg>
              </button>

              <div className="space-y-5 pr-8">
                <h2 className="text-[28px] md:text-[36px] font-semibold text-[#000] leading-[1.25]">
                  잠깐!{" "}
                  <br />
                  이메일을 인증하지 않으면{" "}
                  <br />
                  매일 아침 최신 소식을 받을 수 없어요!
                </h2>

                <p className="text-[15px] md:text-[18px] text-[#000] leading-[1.5]">
                  위즈레터를 구독하시면 복잡한 소식을 간결하게 요약해서,{" "}
                  <br className="hidden md:block" />
                  매일 아침 7시에 보내드립니다.
                  <br />
                  이메일 인증을 완료하고 슬기로운 아침을 시작해 보세요!
                </p>
              </div>

              <div className="flex gap-4 mt-10">
                <button
                  onClick={goToFinalDestination}
                  className="flex-1 py-4 rounded-full text-lg font-semibold bg-white text-[#231815] border-[2px] border-[#231815] hover:bg-[#f5f5f5] transition-all"
                >
                  닫기
                </button>
                <button
                  onClick={() => {
                    setModalStep("email-input");
                    setModalError("");
                  }}
                  className="flex-1 py-4 rounded-full text-lg font-semibold bg-[#43b9d6] text-[#231815] border-[2px] border-[#231815] hover:brightness-110 transition-all"
                >
                  이메일 인증하기
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
