import type { EarnStatusItem } from "@/lib/api";

export function CoinTag({ status }: { status: EarnStatusItem }) {
  const earnTag = renderEarnTag(status);
  const quizTag = renderQuizTag(status);

  if (!earnTag && !quizTag) return null;

  return (
    <span className="inline-flex items-center gap-1.5 flex-shrink-0">
      {earnTag}
      {quizTag}
    </span>
  );
}

function renderEarnTag(status: EarnStatusItem) {
  if (status.status === "DUPLICATE") {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/50 whitespace-nowrap">
        획득!
      </span>
    );
  }
  if (status.status === "EXPIRED") {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/50 whitespace-nowrap">
        획득 기간 경과
      </span>
    );
  }
  if (status.status === "DAILY_LIMIT") {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/50 whitespace-nowrap">
        일일 한도
      </span>
    );
  }
  if (status.status === "PENDING" && status.coins > 0) {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#43b9d6]/15 text-[#43b9d6] whitespace-nowrap">
        +{status.coins}포인트
      </span>
    );
  }
  return null;
}

function renderQuizTag(status: EarnStatusItem) {
  // Quiz tags are suppressed when daily limit or expired
  if (status.status === "DAILY_LIMIT" || status.status === "EXPIRED") return null;
  if (!status.has_quiz) return null;

  if (status.status === "PENDING") {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#f5a623]/15 text-[#f5a623] whitespace-nowrap flex-shrink-0">
        보너스 퀴즈
      </span>
    );
  }

  if (status.status === "DUPLICATE") {
    if (status.quiz_completed) {
      return (
        <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#f5a623]/10 text-[#f5a623]/70 whitespace-nowrap flex-shrink-0">
          퀴즈 풀이 완료
        </span>
      );
    }
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#f5a623]/15 text-[#f5a623] whitespace-nowrap flex-shrink-0">
        보너스 퀴즈
      </span>
    );
  }

  return null;
}
