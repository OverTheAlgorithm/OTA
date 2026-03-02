import { useEffect, useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { InterestSection } from "@/components/interest-section";
import { ChannelPreferencesSection } from "@/components/channel-preferences-section";
import { HistorySection } from "@/components/history-section";
import { getSubscriptions, getUserLevel, type LevelInfo } from "@/lib/api";
import { SendBriefingButton } from "@/components/send-briefing-button";
import { LevelCard } from "@/components/level-card";

export function HomePage() {
  const { user, loading, logout } = useAuth();
  const navigate = useNavigate();

  const [subscriptions, setSubscriptions] = useState<string[]>([]);
  const [levelInfo, setLevelInfo] = useState<LevelInfo | null>(null);

  useEffect(() => {
    if (!loading && !user) navigate("/", { replace: true });
  }, [user, loading, navigate]);

  useEffect(() => {
    if (!user) return;
    getSubscriptions().then(setSubscriptions).catch(() => {});
    getUserLevel().then(setLevelInfo).catch(() => {});
  }, [user]);

  if (loading || !user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#0f0a19]">
        <p className="text-[#9b8bb4]">로딩 중...</p>
      </div>
    );
  }

  const displayName = user.nickname || user.email || "사용자";

  const handleLogout = async () => {
    await logout();
    navigate("/", { replace: true });
  };

  const nextBriefing = () => {
    const now = new Date();
    const kst = new Date(now.toLocaleString("en-US", { timeZone: "Asia/Seoul" }));
    const next = new Date(kst);
    next.setHours(7, 0, 0, 0);
    if (kst >= next) next.setDate(next.getDate() + 1);
    const diff = next.getTime() - kst.getTime();
    const h = Math.floor(diff / 3600000);
    const m = Math.floor((diff % 3600000) / 60000);
    return h > 0 ? `${h}시간 ${m}분 후` : `${m}분 후`;
  };

  return (
    <div className="min-h-screen bg-[#0f0a19] text-[#f5f0ff] flex flex-col">
      <header className="sticky top-0 z-10 border-b border-[#2d1f42] bg-[#0f0a19]/90 backdrop-blur-lg">
        <div className="max-w-2xl mx-auto px-6 h-16 flex items-center justify-between">
          <img src="/OTA_logo.png" alt="OTA" className="w-[63px] h-[42px]" />
          <div className="flex items-center gap-3">
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
              <span className="text-sm text-[#9b8bb4] hidden sm:block">{displayName}</span>
            </div>
            {user.role === "admin" && (
              <Link
                to="/admin"
                className="text-sm text-[#9b8bb4] hover:text-[#f5f0ff] transition-colors"
              >
                관리자 페이지
              </Link>
            )}
            <button
              onClick={handleLogout}
              className="text-sm text-[#9b8bb4] hover:text-[#f5f0ff] transition-colors"
            >
              로그아웃
            </button>
          </div>
        </div>
      </header>

      <main className="flex-1 max-w-2xl w-full mx-auto px-6 py-8 space-y-6">
        {levelInfo ? (
          <LevelCard level={levelInfo} />
        ) : (
          <div className="rounded-2xl bg-gradient-to-br from-[#1a1229] to-[#1e1530] border border-[#2d1f42] px-6 py-5 flex items-center justify-between">
            <div>
              <p className="text-sm text-[#9b8bb4]">안녕하세요</p>
              <h1 className="text-lg font-bold text-[#f5f0ff] mt-0.5">{displayName}님</h1>
            </div>
            <div className="text-right">
              <p className="text-xs text-[#9b8bb4]">다음 브리핑</p>
              <p className="text-sm font-semibold text-[#e84d3d] mt-0.5">{nextBriefing()}</p>
            </div>
          </div>
        )}

        <InterestSection selected={subscriptions} onChange={setSubscriptions} />
        <ChannelPreferencesSection />
        <SendBriefingButton />
        <HistorySection subscriptions={subscriptions} />
      </main>

      <footer className="border-t border-[#2d1f42] py-6 px-6 mt-4">
        <div className="max-w-2xl mx-auto flex flex-col sm:flex-row justify-between items-center gap-3">
          <img src="/OTA_logo.png" alt="OTA" className="h-5 opacity-50" />
          <p className="text-xs text-[#9b8bb4]">
            &copy; 2026 Over the Algorithm. All rights reserved.
          </p>
        </div>
      </footer>
    </div>
  );
}
