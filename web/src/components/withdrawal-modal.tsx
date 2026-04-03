import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { requestWithdrawal, type WithdrawalInfo } from "@/lib/api";

interface Props {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  preCheckInfo: WithdrawalInfo;
}

export function WithdrawalModal({ open, onClose, onSuccess, preCheckInfo }: Props) {
  const unit = preCheckInfo.withdrawal_unit_amount;
  const maxWithdrawable = Math.floor(preCheckInfo.current_balance / unit) * unit;

  const [amount, setAmount] = useState(0);
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) return;
    setAmount(0);
    setError("");
    setSubmitting(false);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [open, onClose]);

  if (!open) return null;

  const isAmountValid =
    amount > 0 && amount <= maxWithdrawable && amount % unit === 0;

  const handleAmountChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = parseInt(e.target.value, 10);
    setError("");
    if (isNaN(val)) {
      setAmount(0);
      return;
    }
    if (val > maxWithdrawable) {
      setAmount(maxWithdrawable);
      setError(`최대 출금 가능 금액은 ${maxWithdrawable.toLocaleString()}P 입니다.`);
      return;
    }
    if (val % unit !== 0) {
      setAmount(val);
      setError(`출금 금액은 ${unit.toLocaleString()}P 단위로 입력해주세요.`);
      return;
    }
    setAmount(val);
  };

  const handleSubmit = async () => {
    if (!isAmountValid || submitting) return;
    setSubmitting(true);
    setError("");
    try {
      await requestWithdrawal(amount);
      onSuccess();
    } catch (e) {
      setError(e instanceof Error ? e.message : "출금 신청에 실패했습니다.");
    } finally {
      setSubmitting(false);
    }
  };

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="bg-white rounded-[22px] max-w-[480px] w-[90%] mx-auto p-8 relative border-[2px] border-[#231815]">
        <button
          onClick={onClose}
          className="absolute top-4 right-4 w-10 h-10 flex items-center justify-center text-[#231815]/50 hover:text-[#231815] transition-colors"
          aria-label="닫기"
        >
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
          >
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>

        <h2 className="text-2xl font-bold text-[#231815] mb-6">출금 요청</h2>

        {/* Balance display */}
        <div className="mb-6">
          <p className="text-sm text-[#231815]/60 mb-1">현재 보유 포인트</p>
          <p className="text-3xl font-bold text-[#231815]">
            {preCheckInfo.current_balance.toLocaleString()} P
          </p>
        </div>

        {/* Amount input */}
        <div className="mb-2">
          <label className="block text-sm font-semibold text-[#231815] mb-2">
            출금할 금액
          </label>
          <div className="flex items-center gap-2">
            <input
              type="number"
              step={unit}
              min={unit}
              max={maxWithdrawable}
              value={amount === 0 ? "" : amount}
              onChange={handleAmountChange}
              placeholder={`${unit.toLocaleString()}P 단위`}
              className="w-full h-12 px-4 border-[2px] border-[#231815] rounded-xl text-lg text-right focus:outline-none focus:ring-2 focus:ring-[#43b9d6]"
            />
            <span className="text-lg font-semibold text-[#231815] shrink-0">P</span>
          </div>
        </div>

        {/* Remaining balance */}
        {isAmountValid && (
          <p className="text-sm text-[#231815]/60 mb-2">
            출금 후 잔액: {(preCheckInfo.current_balance - amount).toLocaleString()} P
          </p>
        )}

        {/* Error */}
        {error && (
          <p className="text-sm text-[#ff5442] mb-2">{error}</p>
        )}

        {/* Submit button */}
        <button
          onClick={handleSubmit}
          disabled={!isAmountValid || submitting}
          className="mt-4 w-full h-[50px] rounded-full bg-[#43b9d6] border-[2px] border-[#231815] text-[#231815] font-semibold text-lg transition-opacity disabled:opacity-40"
        >
          {submitting ? "요청 중..." : "출금 요청하기"}
        </button>
      </div>
    </div>,
    document.body
  );
}
