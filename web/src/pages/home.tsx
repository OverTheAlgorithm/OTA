import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";

export function HomePage() {
  const { user, loading, logout } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (!loading && !user) {
      navigate("/", { replace: true });
    }
  }, [user, loading, navigate]);

  if (loading || !user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#0f0a19]">
        <p className="text-[#9b8bb4]">로딩 중...</p>
      </div>
    );
  }

  const handleLogout = async () => {
    await logout();
    navigate("/", { replace: true });
  };

  const displayName = user.nickname || user.email || "사용자";

  return (
    <div className="min-h-screen bg-[#0f0a19] text-[#f5f0ff]">
      {/* Header */}
      <header className="border-b border-[#2d1f42] bg-[#0f0a19]/90 backdrop-blur-lg sticky top-0 z-10">
        <div className="max-w-4xl mx-auto px-6 h-16 flex items-center justify-between">
          <img src="/OTA_logo.png" alt="OTA" className="h-7" />

          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              {user.profile_image ? (
                <img
                  src={user.profile_image}
                  alt=""
                  className="w-8 h-8 rounded-full ring-1 ring-[#2d1f42]"
                />
              ) : (
                <div className="w-8 h-8 rounded-full bg-[#2d1f42] flex items-center justify-center text-xs text-[#9b8bb4]">
                  {displayName[0]}
                </div>
              )}
              <span className="text-sm text-[#9b8bb4] hidden sm:block">
                {displayName}
              </span>
            </div>
            <button
              onClick={handleLogout}
              className="text-sm text-[#9b8bb4] hover:text-[#f5f0ff] transition-colors"
            >
              로그아웃
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-4xl mx-auto px-6 py-16">
        {/* Welcome */}
        <div className="text-center mb-12">
          <h2 className="text-3xl md:text-4xl font-bold">
            반갑습니다,{" "}
            <span className="bg-gradient-to-r from-[#e84d3d] to-[#f0923b] bg-clip-text text-transparent">
              {displayName}
            </span>
            님
          </h2>
          <p className="mt-3 text-[#9b8bb4]">
            매일 아침 7시, 가장 뜨거운 맥락을 전해드릴게요.
          </p>
        </div>

        {/* Status card */}
        <div className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] p-8 flex flex-col items-center text-center gap-4">
          <div className="w-14 h-14 rounded-2xl bg-[#e84d3d]/10 flex items-center justify-center">
            <svg
              className="w-7 h-7 text-[#e84d3d]"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <circle cx="12" cy="12" r="10" />
              <path d="M12 6v6l4 2" />
            </svg>
          </div>
          <div>
            <p className="font-semibold text-[#f5f0ff]">다음 맥락 브리핑</p>
            <p className="text-sm text-[#9b8bb4] mt-1">
              내일 아침 7시 (KST) 카카오톡 · 이메일로 전달됩니다
            </p>
          </div>
          <div className="w-full h-px bg-[#2d1f42]" />
          <p className="text-sm text-[#9b8bb4]">
            알고리즘 너머, 오늘 세상에서 가장 뜨거웠던 이야기를 내일 아침 만나보세요.
          </p>
        </div>
      </main>
    </div>
  );
}
