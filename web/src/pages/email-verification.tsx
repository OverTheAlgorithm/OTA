import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { sendVerificationCode, verifyEmailCode } from "@/lib/api";

export function EmailVerificationPage() {
  const { refreshUser } = useAuth();
  const navigate = useNavigate();

  const [step, setStep] = useState<"email" | "code">("email");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSendCode = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    // Email format validation
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
      navigate("/home", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "인증 코드 확인에 실패했습니다");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-[#0f0a19] px-4">
      <div className="w-full max-w-md space-y-6">
        {/* Header */}
        <div className="text-center">
          <h1 className="text-2xl font-bold text-white mb-2">이메일 인증</h1>
          <p className="text-sm text-[#9b8bb4]">
            {step === "email"
              ? "이메일 주소를 입력하고 인증 코드를 받아주세요"
              : `${email}로 인증 코드를 전송했습니다`}
          </p>
        </div>

        {/* Email Input Step */}
        {step === "email" && (
          <form onSubmit={handleSendCode} className="space-y-4">
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-[#9b8bb4] mb-2">
                이메일 주소
              </label>
              <input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="example@email.com"
                className="w-full px-4 py-3 bg-[#1a1229] border border-[#2d1f42] rounded-lg text-white placeholder-[#6b5b7f] focus:outline-none focus:border-[#5ba4d9] transition-colors"
                disabled={loading}
                required
              />
            </div>

            {error && (
              <div className="p-3 bg-red-500/10 border border-red-500/30 rounded-lg">
                <p className="text-sm text-red-400">{error}</p>
              </div>
            )}

            <button
              type="submit"
              disabled={loading}
              className="w-full py-3 bg-[#5ba4d9] hover:bg-[#4a8fc2] disabled:bg-[#2d1f42] disabled:text-[#6b5b7f] text-white font-medium rounded-lg transition-colors"
            >
              {loading ? "전송 중..." : "인증 코드 전송"}
            </button>
          </form>
        )}

        {/* Code Verification Step */}
        {step === "code" && (
          <form onSubmit={handleVerifyCode} className="space-y-4">
            <div>
              <label htmlFor="code" className="block text-sm font-medium text-[#9b8bb4] mb-2">
                인증 코드 (6자리)
              </label>
              <input
                id="code"
                type="text"
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                placeholder="000000"
                className="w-full px-4 py-3 bg-[#1a1229] border border-[#2d1f42] rounded-lg text-white text-center text-2xl tracking-widest placeholder-[#6b5b7f] focus:outline-none focus:border-[#5ba4d9] transition-colors"
                disabled={loading}
                maxLength={6}
                required
              />
              <p className="text-xs text-[#6b5b7f] mt-2">
                인증 코드는 5분 동안 유효합니다
              </p>
            </div>

            {error && (
              <div className="p-3 bg-red-500/10 border border-red-500/30 rounded-lg">
                <p className="text-sm text-red-400">{error}</p>
              </div>
            )}

            <div className="space-y-2">
              <button
                type="submit"
                disabled={loading || code.length !== 6}
                className="w-full py-3 bg-[#5ba4d9] hover:bg-[#4a8fc2] disabled:bg-[#2d1f42] disabled:text-[#6b5b7f] text-white font-medium rounded-lg transition-colors"
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
                className="w-full py-3 bg-transparent border border-[#2d1f42] hover:border-[#5ba4d9] text-[#9b8bb4] hover:text-white font-medium rounded-lg transition-colors"
              >
                이메일 다시 입력
              </button>
            </div>
          </form>
        )}

        {/* Back to Home */}
        <button
          onClick={() => navigate("/home")}
          className="w-full py-2 text-sm text-[#9b8bb4] hover:text-white transition-colors"
        >
          홈으로 돌아가기
        </button>
      </div>
    </div>
  );
}
