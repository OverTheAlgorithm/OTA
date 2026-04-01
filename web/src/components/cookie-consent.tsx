import { useState, useEffect } from "react";
import { Link } from "react-router-dom";

const CONSENT_KEY = "wl_cookie_consent";

export function CookieConsent() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const consented = localStorage.getItem(CONSENT_KEY);
    if (!consented) {
      setVisible(true);
    }
  }, []);

  const handleConsent = () => {
    localStorage.setItem(CONSENT_KEY, "true");
    setVisible(false);
  };

  if (!visible) return null;

  return (
    <div className="fixed bottom-0 left-0 right-0 z-50 flex items-center justify-between gap-4 border-t border-[#231815] bg-[#fdf9ee] px-4 py-3 sm:px-6">
      <p className="text-sm text-[#231815]">
        이 웹사이트는 서비스 개선을 위해 쿠키를 사용합니다.{" "}
        <Link
          to="/cookie-policy"
          className="underline underline-offset-2 hover:text-[#43b9d6]"
        >
          자세히 보기
        </Link>
      </p>
      <button
        onClick={handleConsent}
        className="shrink-0 border border-[#231815] bg-[#43b9d6] px-4 py-1.5 text-sm font-medium text-white hover:opacity-90"
      >
        동의
      </button>
    </div>
  );
}
