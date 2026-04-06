import { useEffect } from "react";
import { createPortal } from "react-dom";

interface Props {
  open: boolean;
  message: string;
  onClose: () => void;
}

export function WithdrawalErrorModal({ open, message, onClose }: Props) {
  useEffect(() => {
    if (!open) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [open, onClose]);

  if (!open) return null;

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="bg-white rounded-[30px] max-w-[500px] w-[90%] mx-auto p-10 relative">
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

        <div className="flex justify-center mb-6">
          <img
            src="/wl-piggy.png"
            alt=""
            className="w-[120px] h-[108px] object-contain"
          />
        </div>

        <p className="text-lg text-[#231815] text-center whitespace-pre-line font-normal">
          {message}
        </p>

        <button
          onClick={onClose}
          className="mx-auto mt-8 block w-[300px] max-w-full h-[50px] rounded-full border-[2px] border-[#231815] bg-white text-[#231815] font-semibold text-lg hover:bg-gray-50 transition-colors"
        >
          확인
        </button>
      </div>
    </div>,
    document.body
  );
}
