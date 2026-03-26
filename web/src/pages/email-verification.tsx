import { useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import {
  sendVerificationCode,
  verifyEmailCode,
  getDeliveryChannels,
  updateDeliveryChannels,
} from "@/lib/api";
import { Header } from "@/components/header";

export function EmailVerificationPage() {
  const { refreshUser } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const autoSubscribe = searchParams.get("auto_subscribe") === "true";

  const [step, setStep] = useState<"email" | "code">("email");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSendCode = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    const emailRegex = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/;
    if (!emailRegex.test(email)) {
      setError("올바른 이메일 형식을 입력해주세요");
      return;
    }

    setLoading(true);
    try {
      await sendVerificationCode(email);
      setStep("code");
    } catch (err) {
      setError(err instanceof Error ? err.message : "인증 코드 전송에 실패했습니다");
    } finally {
      setLoading(false);
    }
  };

  const handleVerifyCode = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (code.length !== 6) {
      setError("6자리 인증 코드를 입력해주세요");
      return;
    }

    setLoading(true);
    try {
      await verifyEmailCode(code);
      await refreshUser();
      if (autoSubscribe) {
        const existing = await getDeliveryChannels();
        const hasEmail = existing.some((ch) => ch.channel === "email");
        const merged = hasEmail
          ? existing.map((ch) =>
              ch.channel === "email" ? { ...ch, enabled: true } : ch
            )
          : [...existing, { channel: "email", enabled: true }];
        await updateDeliveryChannels(merged);
        navigate("/mypage", { replace: true });
      } else {
        navigate("/mypage?tab=settings", { replace: true });
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "인증 코드 확인에 실패했습니다");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex flex-col bg-[#fdf9ee]">
      <Header />

      <main className="flex-1 flex items-center justify-center px-4 py-12">
        <div className="relative w-full max-w-[500px] bg-white rounded-[30px] px-10 py-12 md:px-14 md:py-16">
          {/* Close button */}
          <button
            onClick={() => navigate(-1)}
            className="absolute top-6 right-6 w-10 h-10 flex items-center justify-center rounded-full hover:bg-[#f5f5f5] transition-colors"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#231815" strokeWidth="2.5" strokeLinecap="round">
              <path d="M18 6L6 18M6 6l12 12" />
            </svg>
          </button>

          {/* Title */}
          <h1 className="text-4xl md:text-5xl font-semibold text-[#231815] tracking-tight leading-tight mb-8">
            이메일 인증하기
          </h1>

          {/* Step 1: Email Input */}
          {step === "email" && (
            <form onSubmit={handleSendCode} className="space-y-6">
              <div>
                <label htmlFor="email" className="block text-sm font-medium text-[#231815] mb-2">
                  이메일 주소
                </label>
                <input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="example@email.com"
                  className="w-full px-6 py-4 bg-white border border-[#bdc4cd] rounded-xl text-[#231815] text-base placeholder-[#96a0ad] focus:outline-none focus:border-[#43b9d6] transition-colors"
                  disabled={loading}
                  required
                />
              </div>

              {error && (
                <p className="text-sm text-[#ff5442]">{error}</p>
              )}

              <button
                type="submit"
                disabled={loading}
                className="w-full py-4 rounded-full text-lg font-semibold bg-[#43b9d6] text-[#231815] border-[2px] border-[#231815] hover:brightness-110 disabled:opacity-50 transition-all"
              >
                {loading ? "전송 중..." : "인증번호 보내기"}
              </button>
            </form>
          )}

          {/* Step 2: Code Verification */}
          {step === "code" && (
            <form onSubmit={handleVerifyCode} className="space-y-6">
              <p className="text-[15px] text-[#231815]">
                {email}로 인증 코드를 전송했습니다
              </p>

              <div>
                <label htmlFor="code" className="block text-sm font-medium text-[#231815] mb-2">
                  인증 코드 (6자리)
                </label>
                <input
                  id="code"
                  type="text"
                  value={code}
                  onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                  placeholder="000000"
                  className="w-full px-6 py-4 bg-white border border-[#bdc4cd] rounded-xl text-[#231815] text-base placeholder-[#96a0ad] focus:outline-none focus:border-[#43b9d6] transition-colors"
                  disabled={loading}
                  maxLength={6}
                  required
                />
              </div>

              {error && (
                <p className="text-sm text-[#ff5442]">{error}</p>
              )}

              <div className="space-y-3">
                <button
                  type="submit"
                  disabled={loading || code.length !== 6}
                  className="w-full py-4 rounded-full text-lg font-semibold bg-[#43b9d6] text-[#231815] border-[2px] border-[#231815] hover:brightness-110 disabled:opacity-50 transition-all"
                >
                  {loading ? "확인 중..." : "인증 완료"}
                </button>

                <button
                  type="button"
                  onClick={() => {
                    setStep("email");
                    setCode("");
                    setError("");
                  }}
                  disabled={loading}
                  className="w-full py-4 rounded-full text-lg font-semibold bg-white text-[#231815] border-[2px] border-[#231815] hover:bg-[#f5f5f5] disabled:opacity-50 transition-all"
                >
                  이메일 다시 입력하기
                </button>
              </div>
            </form>
          )}
        </div>
      </main>
    </div>
  );
}
