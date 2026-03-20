import { useEffect, useState } from "react";
import { useNavigate, Link, useSearchParams } from "react-router-dom";
import { Header } from "@/components/header";
import { useAuth } from "@/contexts/auth-context";
import { UserLevelCard } from "@/components/user-level-card";
import { InterestSection } from "@/components/interest-section";
import { ChannelPreferencesSection } from "@/components/channel-preferences-section";
import { Footer } from "@/components/footer";
import {
  getCoinHistory,
  getSubscriptions,
  deleteAccount,
  type CoinTransaction,
} from "@/lib/api";

type Tab = "points" | "settings";

function formatDate(iso: string) {
  const d = new Date(iso);
  const y = d.getFullYear();
  const mo = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  const h = String(d.getHours()).padStart(2, "0");
  const mi = String(d.getMinutes()).padStart(2, "0");
  return `${y}.${mo}.${day} ${h}:${mi}`;
}

const COIN_TYPE_LABELS: Record<string, string> = {
  signup_bonus: "가입 보너스",
  admin_set: "포인트 조정",
  admin_adjust: "포인트 조정",
};

function getCoinLabel(tx: CoinTransaction): string {
  return COIN_TYPE_LABELS[tx.type] ?? tx.description ?? tx.type;
}

// ── Point History Tab ────────────────────────────────────────────────────────

function PointHistoryTab() {
  const [transactions, setTransactions] = useState<CoinTransaction[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getCoinHistory(20, 0)
      .then(({ data, has_more }) => {
        setTransactions(data);
        setHasMore(has_more);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const loadMore = async () => {
    try {
      const { data, has_more } = await getCoinHistory(20, transactions.length);
      setTransactions((prev) => [...prev, ...data]);
      setHasMore(has_more);
    } catch {
      // silently fail — list stays as-is
    }
  };

  if (loading) {
    return <p className="text-sm text-[#231815]/50 py-8 text-center">불러오는 중...</p>;
  }

  if (transactions.length === 0) {
    return <p className="text-sm text-[#231815]/50 py-8 text-center">포인트 내역이 없습니다.</p>;
  }

  return (
    <div className="space-y-4">
      {transactions.map((tx) => {
        const content = (
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0 flex-1">
              <h3 className="text-base font-bold text-[#231815] leading-snug truncate">
                {getCoinLabel(tx)}
              </h3>
              <p className="text-sm text-[#231815]/60 mt-1">
                {formatDate(tx.created_at)}
              </p>
            </div>
            {tx.amount > 0 && (
              <span className="inline-flex items-center justify-center px-3 h-7 rounded-full bg-[#43b9d6] border border-[#231815] text-sm font-bold text-[#231815] whitespace-nowrap flex-shrink-0">
                +{tx.amount}포인트
              </span>
            )}
            {tx.amount < 0 && (
              <span className="inline-flex items-center justify-center px-3 h-7 rounded-full bg-[#e8e8e8] border border-[#231815] text-sm font-bold text-[#231815] whitespace-nowrap flex-shrink-0">
                {tx.amount}포인트
              </span>
            )}
          </div>
        );

        return tx.link_id ? (
          <Link
            key={tx.id}
            to={`/topic/${tx.link_id}`}
            className="block border-l-[3px] border-[#43b9d6] pl-5 py-2 hover:bg-[#43b9d6]/5 transition-colors rounded-r-lg"
          >
            {content}
          </Link>
        ) : (
          <div
            key={tx.id}
            className="border-l-[3px] border-[#43b9d6] pl-5 py-2"
          >
            {content}
          </div>
        );
      })}

      {hasMore && (
        <button
          onClick={loadMore}
          className="w-full py-3 text-sm font-medium text-[#008fb2] hover:text-[#006d8a] transition-colors"
        >
          더 보기
        </button>
      )}
    </div>
  );
}

// ── Settings Tab ─────────────────────────────────────────────────────────────

function SettingsTab() {
  const [subscriptions, setSubscriptions] = useState<string[]>([]);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    getSubscriptions().then(setSubscriptions).catch(() => {});
  }, []);

  const handleDeleteAccount = async () => {
    const confirmed = window.confirm(
      "정말로 계정을 삭제하시겠습니까?\n\n" +
      "삭제된 계정은 복구할 수 없으며, 보유 중인 포인트과 모든 데이터가 영구적으로 삭제됩니다."
    );
    if (!confirmed) return;

    setDeleting(true);
    try {
      await deleteAccount();
      window.location.href = "/";
    } catch (e) {
      alert(e instanceof Error ? e.message : "계정 삭제에 실패했습니다");
      setDeleting(false);
    }
  };

  return (
    <div className="space-y-8">
      <InterestSection selected={subscriptions} onChange={setSubscriptions} />
      <ChannelPreferencesSection />

      <div className="pt-4">
        <button
          onClick={handleDeleteAccount}
          disabled={deleting}
          className="text-sm text-[#231815]/60 hover:text-[#231815] underline transition-colors disabled:opacity-50"
        >
          {deleting ? "처리 중..." : "회원 탈퇴"}
        </button>
      </div>
    </div>
  );
}

// ── Page ─────────────────────────────────────────────────────────────────────

export function MypagePage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const initialTab = searchParams.get("tab") === "settings" ? "settings" : "points";
  const [tab, setTab] = useState<Tab>(initialTab);

  useEffect(() => {
    if (!authLoading && !user) navigate("/", { replace: true });
  }, [user, authLoading, navigate]);

  if (authLoading || !user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#fdf9ee]">
        <p className="text-[#231815]/60">로딩 중...</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex flex-col bg-[#fdf9ee]">
      {/* ── Header ── */}
      <Header />

      {/* ── Content ── */}
      <main className="flex-1">
        <div className="max-w-[900px] mx-auto px-6 py-8">
          {/* Level Card */}
          <div className="mb-8">
            <UserLevelCard />
          </div>

          {/* Back to Home */}
          <Link
            to="/"
            className="flex items-center gap-2 mb-6 group"
          >
            <svg
              className="w-8 h-8 text-[#231815] group-hover:opacity-70 transition-opacity"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M19 12H5" />
              <path d="M12 19l-7-7 7-7" />
            </svg>
            <span className="text-2xl font-bold text-[#231815] group-hover:opacity-70 transition-opacity">
              홈으로
            </span>
          </Link>

          {/* Tab Navigation */}
          <div className="flex border-b border-[#dbdade] mb-8">
            <button
              onClick={() => setTab("points")}
              className={`px-6 py-3 text-lg font-medium transition-colors relative ${
                tab === "points"
                  ? "text-[#008fb2]"
                  : "text-[#231815] hover:text-[#008fb2]/70"
              }`}
            >
              포인트 내역
              {tab === "points" && (
                <div className="absolute bottom-0 left-0 right-0 h-[3px] bg-[#008fb2]" />
              )}
            </button>
            <button
              onClick={() => setTab("settings")}
              className={`px-6 py-3 text-lg font-medium transition-colors relative ${
                tab === "settings"
                  ? "text-[#008fb2]"
                  : "text-[#231815] hover:text-[#008fb2]/70"
              }`}
            >
              내 정보
              {tab === "settings" && (
                <div className="absolute bottom-0 left-0 right-0 h-[3px] bg-[#008fb2]" />
              )}
            </button>
          </div>

          {/* Tab Content */}
          {tab === "points" ? <PointHistoryTab /> : <SettingsTab />}
        </div>
      </main>

      {/* ── Footer ── */}
      <Footer />
    </div>
  );
}
