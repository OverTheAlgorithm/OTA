import { useEffect, useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { Footer } from "@/components/footer";
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
      <div className="min-h-screen flex items-center justify-center bg-white">
        <p className="text-[#6b8db5]">로딩 중...</p>
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
    <div
      className="min-h-screen flex flex-col"
      style={{
        backgroundColor: "var(--color-bg)",
        color: "var(--color-fg)"
      }}
    >
      <header
        className="sticky top-0 z-10 border-b bg-opacity-90 backdrop-blur-lg"
        style={{
          borderColor: "var(--color-border)",
          backgroundColor: "var(--color-bg)"
        }}
      >
        <div className="max-w-2xl mx-auto px-6 h-16 flex items-center justify-between">
          <img src="/OTA_logo.png" alt="OTA" className="w-[63px] h-[42px]" />
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2">
              {user.profile_image ? (
                <img
                  src={user.profile_image}
                  alt=""
                  className="w-8 h-8 rounded-full ring-1 ring-[#d4e6f5]"
                />
              ) : (
                <div className="w-8 h-8 rounded-full bg-[#e8f4fd] flex items-center justify-center text-xs text-[#4a9fe5]">
                  {displayName[0]}
                </div>
              )}
              <span className="text-sm text-[#6b8db5] hidden sm:block">{displayName}</span>
            </div>
            {user.role === "admin" && (
              <Link
                to="/admin"
                className="text-sm text-[#6b8db5] hover:text-[#1e3a5f] transition-colors"
              >
                관리자 페이지
              </Link>
            )}
            <button
              onClick={handleLogout}
              className="text-sm text-[#6b8db5] hover:text-[#1e3a5f] transition-colors"
            >
              로그아웃
            </button>
          </div>
        </div>
      </header>

      <main className="flex-1 max-w-2xl w-full mx-auto px-6 py-8 space-y-6">
        {levelInfo ? (
          <div className="space-y-3">
            <LevelCard level={levelInfo} />
            <Link
              to="/mypage"
              className="block w-full text-center py-3 rounded-xl font-semibold text-sm transition-colors border border-[#4a9fe5]/30 bg-[#4a9fe5]/10 text-[#4a9fe5] hover:bg-[#4a9fe5]/20"
            >
              마이페이지
            </Link>
          </div>
        ) : (
          <div className="rounded-2xl bg-gradient-to-br from-[#f0f7ff] to-[#e8f4fd] border border-[#d4e6f5] px-6 py-5 flex items-center justify-between">
            <div>
              <p className="text-sm text-[#6b8db5]">안녕하세요</p>
              <h1 className="text-lg font-bold text-[#1e3a5f] mt-0.5">{displayName}님</h1>
            </div>
            <div className="text-right">
              <p className="text-xs text-[#6b8db5]">다음 브리핑</p>
              <p className="text-sm font-semibold text-[#ff5442] mt-0.5">{nextBriefing()}</p>
            </div>
          </div>
        )}

        <InterestSection selected={subscriptions} onChange={setSubscriptions} />
        <ChannelPreferencesSection />
        <SendBriefingButton />
        <HistorySection subscriptions={subscriptions} />
      </main>

      <Footer compact />
    </div>
  );
}
