import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { Header } from "@/components/header";
import { useAuth } from "@/contexts/auth-context";
import {
  getBankAccount,
  saveBankAccount,
  getWithdrawalInfo,
  requestWithdrawal,
  getWithdrawalHistory,
  cancelWithdrawal,
  getUserLevel,
  type BankAccount,
  type WithdrawalDetail,
  type WithdrawalInfo,
  type LevelInfo,
} from "@/lib/api";

const STATUS_LABEL: Record<string, string> = {
  pending: "대기",
  approved: "승인",
  rejected: "거절",
  cancelled: "취소",
};

const STATUS_COLOR: Record<string, string> = {
  pending: "text-[#e5a54a] bg-[#e5a54a]/10 border-[#e5a54a]/30",
  approved: "text-green-600 bg-green-100 border-green-300",
  rejected: "text-[#ff5442] bg-[#ff5442]/10 border-[#ff5442]/30",
  cancelled: "text-[#6b8db5] bg-[#6b8db5]/10 border-[#6b8db5]/30",
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

function BankAccountForm({
  initial,
  onSaved,
}: {
  initial: BankAccount | null;
  onSaved: () => void;
}) {
  const [bankName, setBankName] = useState(initial?.bank_name ?? "");
  const [accountNumber, setAccountNumber] = useState(initial?.account_number ?? "");
  const [accountHolder, setAccountHolder] = useState(initial?.account_holder ?? "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      await saveBankAccount({
        bank_name: bankName.trim(),
        account_number: accountNumber.trim(),
        account_holder: accountHolder.trim(),
      });
      onSaved();
    } catch (e) {
      setError(e instanceof Error ? e.message : "저장 실패");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div>
          <label className="text-xs text-[#6b8db5] mb-1 block">은행명</label>
          <input
            value={bankName}
            onChange={(e) => setBankName(e.target.value)}
            className="w-full bg-white border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
            placeholder="예: 카카오뱅크"
          />
        </div>
        <div>
          <label className="text-xs text-[#6b8db5] mb-1 block">계좌번호</label>
          <input
            value={accountNumber}
            onChange={(e) => setAccountNumber(e.target.value)}
            className="w-full bg-white border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
            placeholder="'-' 없이 입력"
          />
        </div>
        <div>
          <label className="text-xs text-[#6b8db5] mb-1 block">예금주</label>
          <input
            value={accountHolder}
            onChange={(e) => setAccountHolder(e.target.value)}
            className="w-full bg-white border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
            placeholder="홍길동"
          />
        </div>
      </div>
      {error && <p className="text-sm text-[#ff5442]">{error}</p>}
      <button
        onClick={handleSave}
        disabled={saving || !bankName.trim() || !accountNumber.trim() || !accountHolder.trim()}
        className="px-4 py-2 rounded-lg text-sm font-semibold transition-colors disabled:opacity-50"
        style={{ background: "var(--color-button-primary)", color: "white" }}
      >
        {saving ? "저장 중..." : initial ? "계좌 수정" : "계좌 등록"}
      </button>
    </div>
  );
}

export function WithdrawalPage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();

  const [bankAccount, setBankAccount] = useState<BankAccount | null>(null);
  const [info, setInfo] = useState<WithdrawalInfo | null>(null);
  const [levelInfo, setLevelInfo] = useState<LevelInfo | null>(null);
  const [history, setHistory] = useState<WithdrawalDetail[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [amount, setAmount] = useState("");
  const [requestError, setRequestError] = useState<string | null>(null);
  const [requesting, setRequesting] = useState(false);
  const [cancellingId, setCancellingId] = useState<string | null>(null);

  const pendingAmount = history
    .filter((w) => w.current_status === "pending")
    .reduce((sum, w) => sum + w.amount, 0);

  const availableCoins = (levelInfo?.total_coins ?? 0) - pendingAmount;

  const loadAll = useCallback(async () => {
    setLoading(true);
    try {
      const [ba, wi, lv, hist] = await Promise.all([
        getBankAccount(),
        getWithdrawalInfo(),
        getUserLevel(),
        getWithdrawalHistory(20, 0),
      ]);
      setBankAccount(ba);
      setInfo(wi);
      setLevelInfo(lv);
      setHistory(hist.data);
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
    if (user) loadAll();
  }, [user, loadAll]);

  const handleRequest = async () => {
    const numAmount = parseInt(amount, 10);
    if (!numAmount || numAmount <= 0) return;
    setRequesting(true);
    setRequestError(null);
    try {
      await requestWithdrawal(numAmount);
      setAmount("");
      await loadAll();
    } catch (e) {
      setRequestError(e instanceof Error ? e.message : "출금 신청 실패");
    } finally {
      setRequesting(false);
    }
  };

  const handleCancel = async (id: string) => {
    if (!confirm("출금 신청을 취소하시겠습니까? 코인이 복구됩니다.")) return;
    setCancellingId(id);
    try {
      await cancelWithdrawal(id);
      await loadAll();
    } catch (e) {
      alert(e instanceof Error ? e.message : "취소 실패");
    } finally {
      setCancellingId(null);
    }
  };

  const loadMore = async () => {
    const { data, has_more } = await getWithdrawalHistory(20, history.length);
    setHistory((prev) => [...prev, ...data]);
    setHasMore(has_more);
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
      <Header />

      <main className="flex-1 max-w-2xl w-full mx-auto px-6 py-8 space-y-6">
        {/* 잔액 정보 */}
        <div className="rounded-2xl bg-gradient-to-br from-[#f0f7ff] to-[#e8f4fd] border border-[#d4e6f5] px-6 py-5">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs text-[#6b8db5]">보유 코인</p>
              <p className="text-2xl font-bold text-[#1e3a5f]">
                {(levelInfo?.total_coins ?? 0).toLocaleString()}
              </p>
            </div>
            <div className="text-right">
              {pendingAmount > 0 && (
                <>
                  <p className="text-xs text-[#e5a54a]">출금 대기</p>
                  <p className="text-lg font-semibold text-[#e5a54a]">
                    -{pendingAmount.toLocaleString()}
                  </p>
                </>
              )}
              <p className="text-xs text-[#6b8db5] mt-1">출금 가능</p>
              <p className="text-lg font-semibold text-[#1e3a5f]">
                {availableCoins.toLocaleString()}
              </p>
            </div>
          </div>
        </div>

        {/* 계좌 등록 */}
        <section className="rounded-2xl border border-[#d4e6f5] bg-[#f0f7ff] p-6 space-y-4">
          <h2 className="text-lg font-semibold text-[#1e3a5f]">계좌 정보</h2>
          {bankAccount ? (
            <div className="space-y-3">
              <div className="text-sm text-[#1e3a5f]">
                <span className="text-[#6b8db5]">은행:</span> {bankAccount.bank_name}
                {" · "}
                <span className="text-[#6b8db5]">계좌:</span> {bankAccount.account_number}
                {" · "}
                <span className="text-[#6b8db5]">예금주:</span> {bankAccount.account_holder}
              </div>
              <BankAccountForm initial={bankAccount} onSaved={loadAll} />
            </div>
          ) : (
            <>
              <p className="text-sm text-[#6b8db5]">출금 받을 계좌를 먼저 등록해주세요.</p>
              <BankAccountForm initial={null} onSaved={loadAll} />
            </>
          )}
        </section>

        {/* 출금 신청 */}
        {bankAccount && (
          <section className="rounded-2xl border border-[#d4e6f5] bg-[#f0f7ff] p-6 space-y-4">
            <h2 className="text-lg font-semibold text-[#1e3a5f]">출금 신청</h2>
            <p className="text-sm text-[#6b8db5]">
              최소 출금 금액: {(info?.min_withdrawal_amount ?? 0).toLocaleString()}원
              {" · "}출금 가능: {availableCoins.toLocaleString()}원
            </p>
            <div className="flex gap-3">
              <input
                type="number"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder="출금 금액"
                min={info?.min_withdrawal_amount ?? 1}
                max={availableCoins}
                className="flex-1 bg-white border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
              />
              <button
                onClick={handleRequest}
                disabled={requesting || !amount || parseInt(amount) <= 0}
                className="px-5 py-2 rounded-lg text-sm font-semibold transition-colors disabled:opacity-50"
                style={{ background: "var(--color-button-primary)", color: "white" }}
              >
                {requesting ? "신청 중..." : "출금 신청"}
              </button>
            </div>
            {requestError && <p className="text-sm text-[#ff5442]">{requestError}</p>}
          </section>
        )}

        {/* 출금 내역 */}
        <section className="rounded-2xl border border-[#d4e6f5] bg-[#f0f7ff] p-6 space-y-4">
          <h2 className="text-lg font-semibold text-[#1e3a5f]">출금 내역</h2>
          {history.length === 0 ? (
            <p className="text-sm text-[#6b8db5]">출금 내역이 없습니다.</p>
          ) : (
            <div className="space-y-3">
              {history.map((w) => (
                <div key={w.id} className="rounded-xl border border-[#d4e6f5] bg-white p-4 space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="text-lg font-bold text-[#1e3a5f]">
                        {w.amount.toLocaleString()}원
                      </span>
                      <span className={`text-xs px-2 py-0.5 rounded-full border ${STATUS_COLOR[w.current_status]}`}>
                        {STATUS_LABEL[w.current_status]}
                      </span>
                    </div>
                    {w.current_status === "pending" && (
                      <button
                        onClick={() => handleCancel(w.id)}
                        disabled={cancellingId === w.id}
                        className="text-xs text-[#ff5442] hover:text-[#ff5442]/80 transition-colors disabled:opacity-50"
                      >
                        {cancellingId === w.id ? "취소 중..." : "취소"}
                      </button>
                    )}
                  </div>
                  <p className="text-xs text-[#6b8db5]">
                    {w.bank_name} {w.account_number} · {formatDate(w.created_at)}
                  </p>
                  {/* 거절 사유 표시 */}
                  {w.transitions
                    ?.filter((t) => t.status === "rejected" && t.note)
                    .map((t) => (
                      <p key={t.id} className="text-xs text-[#ff5442] bg-[#ff5442]/5 rounded-lg px-3 py-2">
                        거절 사유: {t.note}
                      </p>
                    ))}
                </div>
              ))}
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
      </main>
    </div>
  );
}
