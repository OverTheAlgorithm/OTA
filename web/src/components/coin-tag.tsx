import type { EarnStatusItem } from "@/lib/api";

export function CoinTag({ status }: { status: EarnStatusItem }) {
  if (status.status === "DUPLICATE") {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/50">
        획득!
      </span>
    );
  }
  if (status.status === "EXPIRED") {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/50">
        획득 기간 경과
      </span>
    );
  }
  if (status.status === "DAILY_LIMIT") {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/50">
        일일 한도
      </span>
    );
  }
  if (status.status === "PENDING" && status.coins > 0) {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#43b9d6]/15 text-[#43b9d6]">
        +{status.coins}포인트
      </span>
    );
  }
  return null;
}
