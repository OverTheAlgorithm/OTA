import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { saveBankAccount, type BankAccount } from "@/lib/api";

interface Props {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  existing: BankAccount | null;
}

export function BankAccountModal({ open, onClose, onSuccess, existing }: Props) {
  const [bankName, setBankName] = useState("");
  const [accountNumber, setAccountNumber] = useState("");
  const [accountHolder, setAccountHolder] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) return;
    setBankName(existing?.bank_name ?? "");
    setAccountNumber(existing?.account_number ?? "");
    setAccountHolder(existing?.account_holder ?? "");
    setError("");
    setSubmitting(false);
  }, [open, existing]);

  useEffect(() => {
    if (!open) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [open, onClose]);

  if (!open) return null;

  const isValid =
    bankName.trim() !== "" &&
    accountNumber.trim() !== "" &&
    accountHolder.trim() !== "";

  const handleSubmit = async () => {
    if (!isValid || submitting) return;
    setSubmitting(true);
    setError("");
    try {
      await saveBankAccount({
        bank_name: bankName.trim(),
        account_number: accountNumber.trim(),
        account_holder: accountHolder.trim(),
      });
      onSuccess();
    } catch (e) {
      setError(e instanceof Error ? e.message : "계좌 등록에 실패했습니다.");
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
      <div className="bg-white rounded-[30px] max-w-[520px] w-[90%] mx-auto px-10 py-10 relative">
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

        <h2 className="text-2xl font-bold text-[#231815] mb-3">
          계좌번호 등록/변경
        </h2>

        <p className="text-sm text-[#231815]/70 text-center mb-6 leading-relaxed">
          포인트 교환 시 사용되며, 본인 명의의 계좌만 등록할 수 있습니다.
          <br />
          입력하신 개인 정보는 안전하게 보관됩니다.
        </p>

        <div className="space-y-3 mb-4">
          <input
            type="text"
            value={bankName}
            onChange={(e) => setBankName(e.target.value)}
            placeholder="은행 (예: 신한, 카카오뱅크)"
            className="w-full h-12 px-4 border border-[#bdc4cd] rounded-[10px] text-base focus:outline-none focus:ring-2 focus:ring-[#43b9d6]"
          />
          <input
            type="text"
            value={accountNumber}
            onChange={(e) => setAccountNumber(e.target.value)}
            placeholder="계좌번호"
            className="w-full h-12 px-4 border border-[#bdc4cd] rounded-[10px] text-base focus:outline-none focus:ring-2 focus:ring-[#43b9d6]"
          />
          <input
            type="text"
            value={accountHolder}
            onChange={(e) => setAccountHolder(e.target.value)}
            placeholder="예금주"
            className="w-full h-12 px-4 border border-[#bdc4cd] rounded-[10px] text-base focus:outline-none focus:ring-2 focus:ring-[#43b9d6]"
          />
        </div>

        {error && (
          <p className="text-sm text-[#ff5442] mb-3">{error}</p>
        )}

        <button
          onClick={handleSubmit}
          disabled={!isValid || submitting}
          className="w-full h-[50px] rounded-full bg-[#43b9d6] border-[2px] border-[#231815] text-[#231815] font-semibold text-lg transition-opacity disabled:opacity-40"
        >
          {submitting ? "처리 중..." : "등록"}
        </button>
      </div>
    </div>,
    document.body
  );
}
