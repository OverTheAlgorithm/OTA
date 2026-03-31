import { KakaoLoginButton } from "@/components/kakao-login-button";

const DISMISS_KEY = "wl_login_prompt_dismiss";

export function isLoginPromptDismissed(): boolean {
  const dismissed = localStorage.getItem(DISMISS_KEY);
  if (!dismissed) return false;
  return Date.now() < Number(dismissed);
}

function dismissForOneDay() {
  const oneDayMs = 24 * 60 * 60 * 1000;
  localStorage.setItem(DISMISS_KEY, String(Date.now() + oneDayMs));
}

interface LoginPromptModalProps {
  redirectPath: string;
  onClose: () => void;
}

export function LoginPromptModal({ redirectPath, onClose }: LoginPromptModalProps) {
  const handleDismissOneDay = () => {
    dismissForOneDay();
    onClose();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm px-4"
      onClick={onClose}
    >
      <div
        className="relative w-full max-w-sm bg-[#fdf9ee] border-[3px] border-[#231815] rounded-2xl p-8 flex flex-col items-center gap-5"
        onClick={(e) => e.stopPropagation()}
      >
        {/* X button */}
        <button
          onClick={onClose}
          className="absolute top-4 right-4 text-[#231815]/60 hover:text-[#231815] transition-colors"
        >
          <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>

        {/* Guide text */}
        <img src="/wl-logo-square.png" alt="WizLetter" className="w-14 h-14 rounded-xl object-contain" />
        <div className="text-center">
          <h2 className="text-lg font-bold text-[#231815]">
            로그인하고 기사를 읽으면<br />포인트를 획득할 수 있어요!
          </h2>
        </div>

        {/* Kakao login */}
        <KakaoLoginButton redirectPath={redirectPath} />

        {/* Dismiss for one day */}
        <button
          onClick={handleDismissOneDay}
          className="text-xs text-[#231815]/40 hover:text-[#231815]/60 transition-colors"
        >
          하루 동안 보지 않기
        </button>
      </div>
    </div>
  );
}
