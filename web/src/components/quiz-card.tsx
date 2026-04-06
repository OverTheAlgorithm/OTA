import { useState } from "react";
import { submitQuizAnswer, type QuizForUser, type QuizSubmitResult } from "@/lib/api";

interface CompletedState {
  result: QuizSubmitResult;
  selectedIndex: number;
  options: string[];
}

interface QuizCardProps {
  quiz: QuizForUser | null;
  hasQuiz: boolean;
  earnDone: boolean;
  contextItemId: string;
  onCoinsEarned?: (newTotal: number) => void;
}

export function QuizCard({ quiz, earnDone, contextItemId, onCoinsEarned }: QuizCardProps) {
  const [submitting, setSubmitting] = useState(false);
  const [completed, setCompleted] = useState<CompletedState | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleSelect = async (index: number) => {
    if (submitting || !quiz) return;
    setSubmitting(true);
    setError(null);
    try {
      const result = await submitQuizAnswer(contextItemId, index);
      setCompleted({ result, selectedIndex: index, options: quiz.options });
      if (onCoinsEarned) onCoinsEarned(result.total_coins);
    } catch {
      setError("퀴즈 제출에 실패했습니다. 잠시 후 다시 시도해주세요.");
    } finally {
      setSubmitting(false);
    }
  };

  // Disabled state: logged-in user, has_quiz, but earn not yet done
  if (!earnDone) {
    return (
      <div className="rounded-2xl border border-[#231815]/15 bg-[#231815]/[0.03] px-6 py-5 mb-8 transition-all duration-300">
        <div className="flex items-center gap-2 mb-2 flex-wrap">
          <span className="text-base font-bold text-[#231815]/30">보너스 퀴즈</span>
          <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/30 whitespace-nowrap flex-shrink-0">
            잠김
          </span>
        </div>
        <p className="text-sm text-[#231815]/40 leading-relaxed">
          포인트를 획득하면 퀴즈를 풀 수 있어요
        </p>
      </div>
    );
  }

  // Completed state
  if (completed) {
    const { result, selectedIndex, options } = completed;
    return (
      <div className="rounded-2xl border-[2px] border-[#231815]/20 bg-white px-6 py-5 mb-8 transition-all duration-300">
        <div className="flex items-center gap-2 mb-3 flex-wrap">
          <span className="text-base font-bold text-[#231815]">보너스 퀴즈</span>
          {result.correct ? (
            <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-green-100 text-green-700 whitespace-nowrap flex-shrink-0">
              정답!
            </span>
          ) : (
            <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-red-100 text-red-600 whitespace-nowrap flex-shrink-0">
              오답
            </span>
          )}
        </div>

        {result.correct && result.coins_earned > 0 && (
          <div className="mb-4 flex items-center gap-2 px-4 py-2.5 rounded-xl bg-green-50 border border-green-200">
            <span className="text-green-700 font-bold text-sm">+{result.coins_earned} 보너스!</span>
            <span className="text-xs text-green-600">포인트가 적립되었습니다</span>
          </div>
        )}

        {!result.correct && (
          <div className="mb-4 px-4 py-2.5 rounded-xl bg-red-50 border border-red-200">
            <p className="text-sm text-red-700">아쉽지만 틀렸어요.</p>
          </div>
        )}

        <div className="flex flex-col gap-2">
          {options.map((option, i) => {
            const isSelected = i === selectedIndex;
            let cls = "border-[#231815]/15 bg-[#231815]/[0.02] text-[#231815]/50";
            if (isSelected && result.correct) cls = "border-green-400 bg-green-50 text-green-700";
            else if (isSelected && !result.correct) cls = "border-red-400 bg-red-50 text-red-600";
            return (
              <div
                key={i}
                className={`w-full px-4 py-3 rounded-xl border-[2px] ${cls} text-sm font-medium`}
              >
                <span className="font-bold mr-2">{["①", "②", "③", "④"][i]}</span>
                {option}
              </div>
            );
          })}
        </div>
      </div>
    );
  }

  // Active state: earn done, quiz available
  if (!quiz) return null;

  return (
    <div className="rounded-2xl border-[2px] border-[#f5a623] bg-[#fffbf2] px-6 py-5 mb-8 transition-all duration-300">
      <div className="flex items-center gap-2 mb-3 flex-wrap">
        <span className="text-base font-bold text-[#f5a623]">보너스 퀴즈</span>
        <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-[#f5a623]/15 text-[#f5a623] whitespace-nowrap flex-shrink-0">
          정답 맞히면 보너스 포인트!
        </span>
      </div>
      <p className="text-base font-semibold text-[#231815] leading-snug mb-4">
        {quiz.question}
      </p>
      <div className="flex flex-col gap-2">
        {quiz.options.map((option, i) => (
          <button
            key={i}
            onClick={() => handleSelect(i)}
            disabled={submitting}
            className="w-full text-left px-4 py-3 rounded-xl border-[2px] border-[#f5a623]/40 bg-white text-sm font-medium text-[#231815] hover:border-[#f5a623] hover:bg-[#f5a623]/5 transition-all duration-150 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <span className="font-bold text-[#f5a623] mr-2">{["①", "②", "③", "④"][i]}</span>
            {option}
          </button>
        ))}
      </div>
      {error && (
        <p className="mt-3 text-xs text-[#d94040]">{error}</p>
      )}
      {submitting && (
        <p className="mt-3 text-xs text-[#231815]/50">제출 중...</p>
      )}
    </div>
  );
}
