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
  getWithdrawalHistory,
  getBankAccount,
  getSubscriptions,
  deleteAccount,
  updatePenName,
  updateNickname,
  hasRoleAtLeast,
  type CoinTransaction,
  type WithdrawalDetail,
  type BankAccount,
} from "@/lib/api";
import { BankAccountModal } from "@/components/bank-account-modal";
import { LoadingState } from "@/components/spinner";
import { formatDateTime } from "@/lib/utils";

type Tab = "points" | "withdrawals" | "settings";

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
    return <LoadingState inline label="불러오는 중" className="text-[#231815]/50 py-8" />;
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
                {formatDateTime(tx.created_at)}
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

// ── Withdrawal History Tab ───────────────────────────────────────────────────

const STATUS_LABELS: Record<string, string> = {
  pending: "처리중",
  approved: "완료",
  rejected: "거절",
  cancelled: "취소",
};

const STATUS_STYLES: Record<string, string> = {
  pending: "bg-[#ffefc6] border-[#e0b830] text-[#8a6d00]",
  approved: "bg-[#d4f5e2] border-[#2ea55e] text-[#1a6b3a]",
  rejected: "bg-[#ffe0dc] border-[#e04b3a] text-[#9c2b1e]",
  cancelled: "bg-[#e8e8e8] border-[#999] text-[#636363]",
};

function WithdrawalHistoryTab() {
  const [items, setItems] = useState<WithdrawalDetail[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getWithdrawalHistory(20, 0)
      .then(({ data, has_more }) => {
        setItems(data);
        setHasMore(has_more);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const loadMore = async () => {
    try {
      const { data, has_more } = await getWithdrawalHistory(20, items.length);
      setItems((prev) => [...prev, ...data]);
      setHasMore(has_more);
    } catch {
      // silently fail
    }
  };

  if (loading) {
    return <LoadingState inline label="불러오는 중" className="text-[#231815]/50 py-8" />;
  }

  if (items.length === 0) {
    return <p className="text-sm text-[#231815]/50 py-8 text-center">출금 내역이 없습니다.</p>;
  }

  return (
    <div className="space-y-4">
      {items.map((w) => (
        <div
          key={w.id}
          className="border-l-[3px] border-[#43b9d6] pl-5 py-2"
        >
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0 flex-1">
              <p className="text-sm text-[#231815]/60">
                {formatDateTime(w.created_at)}
              </p>
              <h3 className="text-base font-bold text-[#231815] leading-snug mt-1">
                {w.amount.toLocaleString()}포인트 출금
              </h3>
            </div>
            <span
              className={`inline-flex items-center justify-center px-3 h-7 rounded-full border text-sm font-bold whitespace-nowrap flex-shrink-0 ${STATUS_STYLES[w.current_status] ?? ""}`}
            >
              {STATUS_LABELS[w.current_status] ?? w.current_status}
            </span>
          </div>
        </div>
      ))}

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

// ── Pen Name Section (editor+ only) ──────────────────────────────────────────

const PEN_NAME_MIN = 2;
const PEN_NAME_MAX = 32;
const NICKNAME_MIN = 2;
const NICKNAME_MAX = 32;

// Nickname editor. Available to all signed-in users. Submitting flips the
// server-side nickname_state to "custom", which both suppresses the
// first-time comment modal and locks the nickname against future Kakao
// login overwrites.
function NicknameSection({
  initial,
  onSaved,
}: {
  initial: string;
  onSaved: () => Promise<void>;
}) {
  const [value, setValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [savedMsg, setSavedMsg] = useState<string | null>(null);

  const trimmed = value.trim();
  const dirty = trimmed.length > 0 && trimmed !== (initial ?? "").trim();
  const tooShort = trimmed.length > 0 && trimmed.length < NICKNAME_MIN;

  const handleSave = async () => {
    if (saving || !dirty || tooShort) return;
    setSaving(true);
    setError(null);
    setSavedMsg(null);
    try {
      await updateNickname(trimmed);
      await onSaved();
      setSavedMsg("닉네임이 저장되었습니다");
      setValue("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "닉네임 변경에 실패했습니다");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="border-l-[3px] border-[#43b9d6] pl-5">
      <h2 className="text-lg font-bold text-[#231815] mb-1">닉네임</h2>
      <p className="text-xs text-[#231815]/60 mb-3">
        댓글에 표시되는 이름입니다. 현재: <span className="font-medium text-[#231815]">{initial || "닉네임"}</span>
      </p>
      <div className="rounded-[22px] bg-white border border-[#231815] px-5 py-4 space-y-3">
        <input
          type="text"
          value={value}
          onChange={(e) => {
            setValue(e.target.value);
            setSavedMsg(null);
            setError(null);
          }}
          maxLength={NICKNAME_MAX}
          placeholder={initial || "새 닉네임을 입력하세요"}
          className="w-full h-10 px-3 rounded-lg border border-[#231815]/20 focus:border-[#008fb2] focus:outline-none text-sm"
          data-testid="mypage-nickname-input"
        />
        <div className="flex items-center justify-between text-xs">
          <span className="text-[#231815]/60">{trimmed.length}/{NICKNAME_MAX}자</span>
        </div>
        {tooShort && (
          <p className="text-xs text-[#ff5442]">닉네임은 {NICKNAME_MIN}자 이상이어야 합니다.</p>
        )}
        {error && (
          <p className="text-xs text-[#ff5442] bg-[#ff5442]/10 rounded px-2 py-1 border border-[#ff5442]/20">
            {error}
          </p>
        )}
        {savedMsg && <p className="text-xs text-[#2ea55e]">{savedMsg}</p>}
        <button
          type="button"
          onClick={handleSave}
          disabled={saving || !dirty || tooShort}
          className="px-4 h-9 rounded-full border-2 border-[#231815] text-sm font-medium bg-white text-[#231815] hover:bg-[#231815]/5 disabled:opacity-50 disabled:cursor-not-allowed"
          data-testid="mypage-nickname-save"
        >
          {saving ? "저장 중..." : "저장"}
        </button>
      </div>
    </div>
  );
}

function PenNameSection({
  initial,
  fallbackNickname,
  onSaved,
}: {
  initial: string;
  fallbackNickname: string;
  onSaved: () => Promise<void>;
}) {
  const [value, setValue] = useState(initial);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [savedMsg, setSavedMsg] = useState<string | null>(null);

  useEffect(() => {
    setValue(initial);
  }, [initial]);

  const trimmed = value.trim();
  const dirty = trimmed !== (initial ?? "").trim();
  const tooShort = trimmed.length > 0 && trimmed.length < PEN_NAME_MIN;

  const handleSave = async () => {
    if (saving || !dirty || tooShort) return;
    setSaving(true);
    setError(null);
    setSavedMsg(null);
    try {
      await updatePenName(trimmed);
      await onSaved();
      setSavedMsg(trimmed ? "필명이 저장되었습니다" : "필명이 해제되었습니다");
    } catch (e) {
      setError(e instanceof Error ? e.message : "필명 변경에 실패했습니다");
    } finally {
      setSaving(false);
    }
  };

  const previewName = trimmed || fallbackNickname || "닉네임";

  return (
    <div className="border-l-[3px] border-[#43b9d6] pl-5">
      <h2 className="text-lg font-bold text-[#231815] mb-1">필명</h2>
      <p className="text-xs text-[#231815]/60 mb-3">
        에디터 픽에서 작성자 이름으로 표시됩니다. 비워두면 닉네임이 사용됩니다.
      </p>
      <div className="rounded-[22px] bg-white border border-[#231815] px-5 py-4 space-y-3">
        <input
          type="text"
          value={value}
          onChange={(e) => {
            setValue(e.target.value);
            setSavedMsg(null);
            setError(null);
          }}
          maxLength={PEN_NAME_MAX}
          placeholder={fallbackNickname || "필명을 입력하세요"}
          className="w-full h-10 px-3 rounded-lg border border-[#231815]/20 focus:border-[#008fb2] focus:outline-none text-sm"
        />
        <div className="flex items-center justify-between text-xs">
          <span className="text-[#231815]/60">
            표시 이름: <span className="font-medium text-[#231815]">{previewName}</span>
          </span>
          <span className="text-[#231815]/40">
            {trimmed.length}/{PEN_NAME_MAX}자
          </span>
        </div>
        {tooShort && (
          <p className="text-xs text-[#ff5442]">필명은 {PEN_NAME_MIN}자 이상이어야 합니다.</p>
        )}
        {error && (
          <p className="text-xs text-[#ff5442] bg-[#ff5442]/10 rounded px-2 py-1 border border-[#ff5442]/20">
            {error}
          </p>
        )}
        {savedMsg && <p className="text-xs text-[#2ea55e]">{savedMsg}</p>}
        <button
          type="button"
          onClick={handleSave}
          disabled={saving || !dirty || tooShort}
          className="px-4 h-9 rounded-full border-2 border-[#231815] text-sm font-medium bg-white text-[#231815] hover:bg-[#231815]/5 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {saving ? "저장 중..." : "저장"}
        </button>
      </div>
    </div>
  );
}

// ── Settings Tab ─────────────────────────────────────────────────────────────

function SettingsTab() {
  const { user, refreshUser } = useAuth();
  const isEditor = hasRoleAtLeast(user?.role, "editor");
  const [subscriptions, setSubscriptions] = useState<string[]>([]);
  const [bankAccount, setBankAccount] = useState<BankAccount | null>(null);
  const [bankLoading, setBankLoading] = useState(true);
  const [showBankModal, setShowBankModal] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  useEffect(() => {
    getSubscriptions().then(setSubscriptions).catch(() => {});
    getBankAccount()
      .then(setBankAccount)
      .catch(() => {})
      .finally(() => setBankLoading(false));
  }, []);

  const handleBankSuccess = () => {
    setShowBankModal(false);
    getBankAccount().then(setBankAccount).catch(() => {});
  };

  const handleDeleteAccount = async () => {
    setDeleting(true);
    setDeleteError(null);
    try {
      await deleteAccount();
      window.location.href = "/";
    } catch (e) {
      setDeleteError(e instanceof Error ? e.message : "계정 삭제에 실패했습니다");
      setDeleting(false);
    }
  };

  return (
    <div className="space-y-8">
      <InterestSection selected={subscriptions} onChange={setSubscriptions} />
      <ChannelPreferencesSection />

      <NicknameSection
        initial={user?.nickname ?? ""}
        onSaved={refreshUser}
      />

      {isEditor && (
        <PenNameSection
          initial={user?.pen_name ?? ""}
          fallbackNickname={user?.nickname ?? ""}
          onSaved={refreshUser}
        />
      )}

      {/* Bank Account Section */}
      <div className="border-l-[3px] border-[#43b9d6] pl-5">
        <h2 className="text-lg font-bold text-[#231815] mb-3">계좌</h2>
        <div className="flex items-center gap-4 rounded-[22px] bg-white border border-[#231815] px-5 py-4">
          <svg className="w-8 h-8 text-[#231815]/60 flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
            <rect x="2" y="5" width="20" height="14" rx="2" />
            <path d="M2 10h20" />
          </svg>
          <div className="flex-1 min-w-0">
            {bankLoading ? (
              <LoadingState inline label="불러오는 중" className="text-[#231815]/50" />
            ) : bankAccount ? (
              <>
                <p className="text-sm font-medium text-[#231815]">{bankAccount.bank_name}</p>
                <p className="text-sm text-[#231815]/60 mt-0.5">
                  {bankAccount.account_number}
                  <button
                    onClick={() => setShowBankModal(true)}
                    className="ml-2 text-[#008fb2] underline"
                  >
                    변경하기
                  </button>
                </p>
              </>
            ) : (
              <button
                onClick={() => setShowBankModal(true)}
                className="text-sm text-[#008fb2] underline font-medium"
              >
                계좌 등록하기
              </button>
            )}
          </div>
        </div>
      </div>

      <BankAccountModal
        open={showBankModal}
        onClose={() => setShowBankModal(false)}
        onSuccess={handleBankSuccess}
        existing={bankAccount}
      />

      <div className="pt-4">
        <button
          onClick={() => { setDeleteError(null); setShowDeleteModal(true); }}
          disabled={deleting}
          className="text-sm text-[#231815]/60 hover:text-[#231815] underline transition-colors disabled:opacity-50"
        >
          {deleting ? "처리 중..." : "회원 탈퇴"}
        </button>
      </div>

      {/* Delete Account Modal */}
      {showDeleteModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm px-4"
          onClick={() => !deleting && setShowDeleteModal(false)}
        >
          <div
            className="relative w-full max-w-sm bg-white rounded-[30px] px-8 py-10 flex flex-col items-center gap-6"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Close button */}
            <button
              onClick={() => setShowDeleteModal(false)}
              disabled={deleting}
              className="absolute top-5 right-5 text-[#231815]/60 hover:text-[#231815] transition-colors disabled:opacity-50"
            >
              <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="M18 6L6 18M6 6l12 12" />
              </svg>
            </button>

            <h2 className="text-3xl font-semibold text-[#231815] tracking-tight">회원 탈퇴</h2>

            <p className="text-sm text-[#231815] leading-relaxed text-center tracking-tight">
              삭제된 계정은 복구할 수 없으며, 보유 중인 포인트와<br />
              모든 데이터는 삭제 됩니다.<br />
              그래도 탈퇴하시겠습니까?
            </p>

            {deleteError && (
              <p className="text-xs text-[#ff5442] bg-[#ff5442]/10 rounded-lg px-3 py-2 border border-[#ff5442]/20 w-full text-center">
                {deleteError}
              </p>
            )}

            <button
              onClick={handleDeleteAccount}
              disabled={deleting}
              className="w-[70%] py-3.5 rounded-full bg-[#ececec] text-[#636363] text-sm font-semibold hover:bg-[#ff5442] hover:text-white transition-colors disabled:opacity-50"
            >
              {deleting ? "처리 중..." : "탈퇴 하기"}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Page ─────────────────────────────────────────────────────────────────────

export function MypagePage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const tabParam = searchParams.get("tab");
  const initialTab: Tab = tabParam === "settings" ? "settings" : tabParam === "withdrawals" ? "withdrawals" : "points";
  const [tab, setTab] = useState<Tab>(initialTab);

  useEffect(() => {
    if (!authLoading && !user) navigate("/", { replace: true });
  }, [user, authLoading, navigate]);

  if (authLoading || !user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#ffffff]">
        <LoadingState label="로딩 중" className="text-[#231815]/60" />
      </div>
    );
  }

  return (
    <div className="min-h-screen flex flex-col bg-[#ffffff]">
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
              className={`flex-1 px-2 md:px-6 py-3 text-sm md:text-lg font-medium whitespace-nowrap transition-colors relative ${
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
              onClick={() => setTab("withdrawals")}
              className={`flex-1 px-2 md:px-6 py-3 text-sm md:text-lg font-medium whitespace-nowrap transition-colors relative ${
                tab === "withdrawals"
                  ? "text-[#008fb2]"
                  : "text-[#231815] hover:text-[#008fb2]/70"
              }`}
            >
              출금 내역
              {tab === "withdrawals" && (
                <div className="absolute bottom-0 left-0 right-0 h-[3px] bg-[#008fb2]" />
              )}
            </button>
            <button
              onClick={() => setTab("settings")}
              className={`flex-1 px-2 md:px-6 py-3 text-sm md:text-lg font-medium whitespace-nowrap transition-colors relative ${
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
          {tab === "points" && <PointHistoryTab />}
          {tab === "withdrawals" && <WithdrawalHistoryTab />}
          {tab === "settings" && <SettingsTab />}
        </div>
      </main>

      {/* ── Footer ── */}
      <Footer />
    </div>
  );
}
