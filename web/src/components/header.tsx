import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";

export function Header() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = async () => {
    await logout();
    navigate("/", { replace: true });
  };

  return (
    <header className="sticky top-0 z-40 bg-[#fdf9ee] border-b-[3px] border-[#231815]">
      <div className="max-w-[1200px] mx-auto px-6 h-[65px] flex items-center justify-between">
        <Link to="/" className="flex items-center">
          <img
            src="/wl-logo.png"
            alt="WizLetter"
            className="w-[140px] md:w-[200px] object-contain"
          />
        </Link>
        <div className="flex items-center gap-3">
          {user ? (
            <>
              {user.role === "admin" && (
                <Link
                  to="/admin"
                  className="hidden md:inline text-base font-medium text-[#231815] hover:opacity-70 transition-opacity"
                >
                  관리자
                </Link>
              )}
              <button
                onClick={handleLogout}
                className="hidden md:inline text-base font-medium text-[#231815] hover:opacity-70 transition-opacity"
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
            <Link
              to="/"
              className="inline-flex items-center justify-center px-5 h-9 rounded-full bg-[#43b9d6] border-[2px] border-[#231815] text-sm font-medium text-[#231815] hover:opacity-80 transition-opacity"
            >
              로그인
            </Link>
          )}
        </div>
      </div>
    </header>
  );
}
