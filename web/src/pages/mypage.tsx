import { useEffect, useState, useCallback } from "react";
import { useNavigate, Link } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { LevelCard } from "@/components/level-card";
import {
  getCoinHistory,
  getUserLevel,
  deleteAccount,
  type CoinTransaction,
  type LevelInfo,
} from "@/lib/api";

const TYPE_LABEL: Record<string, string> = {
  topic_earn: "주제 열람",
  signup_bonus: "회원가입 보너스",
  promotion: "프로모션",
  admin_adjustment: "관리자 조정",
  withdrawal: "출금 신청",
  withdrawal_refund: "출금 환불",
};

const TYPE_COLOR: Record<string, string> = {
  topic_earn: "text-green-600",
  signup_bonus: "text-green-600",
  promotion: "text-green-600",
  admin_adjustment: "text-[#4a9fe5]",
  withdrawal: "text-[#ff5442]",
  withdrawal_refund: "text-[#e5a54a]",
};

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("ko-KR", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function MypagePage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();

  const [levelInfo, setLevelInfo] = useState<LevelInfo | null>(null);
  const [transactions, setTransactions] = useState<CoinTransaction[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [lv, hist] = await Promise.all([
        getUserLevel(),
        getCoinHistory(20, 0),
      ]);
      setLevelInfo(lv);
      setTransactions(hist.data);
      setHasMore(hist.has_more);
    } catch {
      // silently fail on initial load
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!authLoading && !user) navigate("/", { replace: true });
  }, [user, authLoading, navigate]);

  useEffect(() => {
    if (user) loadData();
  }, [user, loadData]);

  const loadMore = async () => {
    const { data, has_more } = await getCoinHistory(20, transactions.length);
    setTransactions((prev) => [...prev, ...data]);
    setHasMore(has_more);
  };

  const [deleting, setDeleting] = useState(false);

  const handleDeleteAccount = async () => {
    const confirmed = window.confirm(
      "정말로 계정을 삭제하시겠습니까?\n\n" +
      "삭제된 계정은 복구할 수 없으며, 보유 중인 코인과 모든 데이터가 영구적으로 삭제됩니다."
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

  if (authLoading || !user || loading) {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ backgroundColor: "var(--color-bg)" }}>
        <p className="text-[#6b8db5]">로딩 중...</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex flex-col" style={{ backgroundColor: "var(--color-bg)", color: "var(--color-fg)" }}>
      <header
        className="sticky top-0 z-10 border-b bg-opacity-90 backdrop-blur-lg"
        style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-bg)" }}
      >
        <div className="max-w-2xl mx-auto px-6 h-16 flex items-center justify-between">
          <button onClick={() => navigate("/home")} className="text-sm text-[#6b8db5] hover:text-[#1e3a5f] transition-colors">
            &larr; 홈으로
          </button>
          <h1 className="text-lg font-bold text-[#1e3a5f]">마이페이지</h1>
          <div className="w-16" />
        </div>
      </header>

      <main className="flex-1 max-w-2xl w-full mx-auto px-6 py-8 space-y-6">
        {levelInfo && <LevelCard level={levelInfo} />}

        {/* withdrawal link */}
        <Link
          to="/withdrawal"
          className="block w-full text-center py-3 rounded-xl font-semibold text-sm transition-colors border border-[#4a9fe5]/30 bg-[#4a9fe5]/10 text-[#4a9fe5] hover:bg-[#4a9fe5]/20"
        >
          출금하기
        </Link>

        {/* coin history */}
        <section className="rounded-2xl border border-[#d4e6f5] bg-[#f0f7ff] p-6 space-y-4">
          <h2 className="text-lg font-semibold text-[#1e3a5f]">코인 획득 내역</h2>
          {transactions.length === 0 ? (
            <p className="text-sm text-[#6b8db5]">코인 내역이 없습니다.</p>
          ) : (
            <div className="space-y-2">
              {transactions.map((tx) => {
                const inner = (
                  <>
                    <div className="min-w-0 flex-1">
                      <p className="text-sm text-[#1e3a5f] truncate">
                        {tx.description || TYPE_LABEL[tx.type] || tx.type}
                      </p>
                      <p className="text-xs text-[#6b8db5]">{formatDate(tx.created_at)}</p>
                    </div>
                    <span className={`text-sm font-bold whitespace-nowrap ml-3 ${tx.amount > 0 ? (TYPE_COLOR[tx.type] || "text-green-600") : "text-[#ff5442]"}`}>
                      {tx.amount > 0 ? "+" : ""}{tx.amount.toLocaleString()}
                    </span>
                  </>
                );
                return tx.link_id ? (
                  <Link
                    key={tx.id}
                    to={`/topic/${tx.link_id}`}
                    className="flex items-center justify-between rounded-xl border border-[#d4e6f5] bg-white px-4 py-3 hover:bg-[#f0f7ff] transition-colors"
                  >
                    {inner}
                  </Link>
                ) : (
                  <div key={tx.id} className="flex items-center justify-between rounded-xl border border-[#d4e6f5] bg-white px-4 py-3">
                    {inner}
                  </div>
                );
              })}
              {hasMore && (
                <button
                  onClick={loadMore}
                  className="w-full py-2 text-sm text-[#6b8db5] hover:text-[#1e3a5f] transition-colors"
                >
                  더 보기
                </button>
              )}
            </div>
          )}
        </section>

        {/* 회원 탈퇴 */}
        <div className="pt-4">
          <button
            onClick={handleDeleteAccount}
            disabled={deleting}
            className="text-sm text-[#ff5442]/70 hover:text-[#ff5442] transition-colors disabled:opacity-50"
          >
            {deleting ? "처리 중..." : "회원 탈퇴"}
          </button>
        </div>
      </main>
    </div>
  );
}
