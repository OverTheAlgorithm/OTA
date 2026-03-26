import { useState } from "react";
import { Link, useNavigate, useLocation } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { LoginModal } from "./login-modal";
import { SubscriptionNudgeBanner } from "./subscription-nudge-banner";

export function Header() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [loginOpen, setLoginOpen] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);

  const handleLogout = async () => {
    await logout();
    navigate("/", { replace: true });
  };

  return (
    <>
      <header className="sticky top-0 z-40 bg-[#fdf9ee] border-b-[3px] border-[#231815]">
        <div className="max-w-[1200px] mx-auto px-6 h-[65px] flex items-center justify-between">
          <Link to="/" className="flex items-center">
            <img
              src="/wl-logo.png"
              alt="WizLetter"
              className="w-[140px] md:w-[200px] object-contain"
            />
          </Link>
          {/* Center nav (desktop) */}
          <nav className="hidden md:flex items-center gap-8">
            <Link
              to="/latest"
              className="text-base font-medium text-[#231815] hover:opacity-70 transition-opacity"
            >
              최신 소식 보기
            </Link>
            <Link
              to="/allnews"
              className="text-base font-medium text-[#231815] hover:opacity-70 transition-opacity"
            >
              모든 소식 보기
            </Link>
          </nav>

          {/* Right actions (desktop) + hamburger (mobile) */}
          <div className="flex items-center gap-3">
            {/* Desktop actions */}
            <div className="hidden md:flex items-center gap-3">
              {user ? (
                <>
                  {user.role === "admin" && (
                    <Link
                      to="/admin"
                      className="text-base font-medium text-[#231815] hover:opacity-70 transition-opacity"
                    >
                      관리자
                    </Link>
                  )}
                  <button
                    onClick={handleLogout}
                    className="text-base font-medium text-[#231815] hover:opacity-70 transition-opacity"
                  >
                    로그아웃
                  </button>
                  <Link
                    to="/mypage"
                    className="inline-flex items-center justify-center px-5 h-9 rounded-full bg-[#43b9d6] border-[2px] border-[#231815] text-sm font-medium text-[#231815] hover:opacity-80 transition-opacity"
                  >
                    마이페이지
                  </Link>
                </>
              ) : (
                <button
                  onClick={() => setLoginOpen(true)}
                  className="inline-flex items-center justify-center px-5 h-9 rounded-full bg-[#43b9d6] border-[2px] border-[#231815] text-sm font-medium text-[#231815] hover:opacity-80 transition-opacity"
                >
                  로그인
                </button>
              )}
            </div>

            {/* Hamburger (mobile) */}
            <button
              onClick={() => setMenuOpen((v) => !v)}
              className="md:hidden flex flex-col justify-center items-center w-9 h-9 gap-[5px]"
              aria-label="메뉴"
            >
              <span className="block w-5 h-[2px] bg-[#231815]" />
              <span className="block w-5 h-[2px] bg-[#231815]" />
              <span className="block w-5 h-[2px] bg-[#231815]" />
            </button>
          </div>
        </div>

        {/* Mobile menu */}
        {menuOpen && (
          <div className="md:hidden border-t-[2px] border-[#231815] bg-[#fdf9ee] px-6 py-4 space-y-3">
            <Link
              to="/latest"
              className="block text-base font-medium text-[#231815]"
              onClick={() => setMenuOpen(false)}
            >
              최신 소식 보기
            </Link>
            <Link
              to="/allnews"
              className="block text-base font-medium text-[#231815]"
              onClick={() => setMenuOpen(false)}
            >
              모든 소식 보기
            </Link>
            {user ? (
              <>
                {user.role === "admin" && (
                  <Link
                    to="/admin"
                    className="block text-base font-medium text-[#231815]"
                    onClick={() => setMenuOpen(false)}
                  >
                    관리자
                  </Link>
                )}
                <Link
                  to="/mypage"
                  className="block text-base font-medium text-[#231815]"
                  onClick={() => setMenuOpen(false)}
                >
                  마이페이지
                </Link>
                <button
                  onClick={() => { setMenuOpen(false); handleLogout(); }}
                  className="block text-base font-medium text-[#231815]"
                >
                  로그아웃
                </button>
              </>
            ) : (
              <button
                onClick={() => { setMenuOpen(false); setLoginOpen(true); }}
                className="block text-base font-medium text-[#231815]"
              >
                로그인
              </button>
            )}
          </div>
        )}
      </header>

      <SubscriptionNudgeBanner />

      <LoginModal
        open={loginOpen}
        onClose={() => setLoginOpen(false)}
        redirectPath={location.pathname + location.search}
      />
    </>
  );
}
