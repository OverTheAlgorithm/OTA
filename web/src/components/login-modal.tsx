import { KakaoLoginButton } from "./kakao-login-button";

interface LoginModalProps {
  open: boolean;
  onClose: () => void;
  redirectPath?: string;
  error?: boolean;
}

export function LoginModal({ open, onClose, redirectPath, error }: LoginModalProps) {
  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm px-4"
      onClick={onClose}
    >
      <div
        className="relative w-full max-w-sm bg-[#fdf9ee] border-[3px] border-[#231815] rounded-2xl p-8 flex flex-col items-center gap-6"
        onClick={(e) => e.stopPropagation()}
      >
        <button
          onClick={onClose}
          className="absolute top-4 right-4 text-[#231815]/50 hover:text-[#231815] transition-colors"
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
          >
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>

        <img
          src="/wl-logo-square.png"
          alt="WizLetter"
          className="w-[80px] md:w-[100px] object-contain"
        />

        <div className="text-center">
          <h2 className="text-xl font-bold text-[#231815]">
            무료로 구독하기
          </h2>
          <p className="mt-1 text-sm text-[#231815]/60">
            매일 아침 5분, 세상의 흐름을 읽는 위즈레터
          </p>
        </div>

        {error && (
          <p className="text-sm text-[#ff5442] text-center">
            로그인에 실패했습니다. 다시 시도해주세요.
          </p>
        )}

        <KakaoLoginButton redirectPath={redirectPath} />
      </div>
    </div>
  );
}
