import type { LevelInfo } from "@/lib/api";

export function LevelCard({ level, onWithdrawClick }: { level: LevelInfo; onWithdrawClick?: () => void }) {
  const { total_coins, coin_cap, thresholds } = level;
  const fillPercent = Math.min((total_coins / coin_cap) * 100, 100);

  // Next level threshold: find the first threshold > total_coins
  const nextThreshold = thresholds.find((t) => t > total_coins) ?? coin_cap;
  const remaining = Math.max(nextThreshold - total_coins, 0);
  const isMaxLevel = nextThreshold === coin_cap && total_coins >= (thresholds[thresholds.length - 1] ?? 0);

  return (
    <div className="rounded-[22px] bg-white border-[2px] border-[#231815] px-6 py-5 flex items-center gap-5">
      {/* Point icon */}
      <div className="flex-shrink-0 w-[90px] h-[90px] md:w-[108px] md:h-[108px] rounded-full border-[3px] border-[#231815] bg-[#43b9d6]/15 flex items-center justify-center">
        <span className="text-4xl md:text-5xl font-bold text-[#231815]">P</span>
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        {/* Top row: Level + Withdraw button */}
        <div className="flex items-start justify-between">
          <div>
            <p className="text-xl md:text-2xl font-bold text-[#231815] leading-tight">
              Lv.{level.level}
            </p>
            <div className="flex items-baseline gap-1.5 mt-0.5">
              <span className="text-3xl md:text-[40px] font-bold text-[#231815] leading-tight">
                {total_coins.toLocaleString()}
              </span>
              <span className="text-sm md:text-base font-bold text-[#231815]">
                포인트
              </span>
            </div>
          </div>
          {onWithdrawClick && (
            <button
              className="flex-shrink-0 px-8 py-3 rounded-full bg-[#43b9d6] border-[2px] border-[#231815] text-lg font-medium text-[#231815] hover:brightness-110 transition-all"
              onClick={onWithdrawClick}
            >
              출금하기
            </button>
          )}
        </div>

        {/* Progress bar with tick marks */}
        <div className="flex items-center gap-3 mt-2">
          <div className="flex-1">
            <div className="relative h-[14px] rounded-full bg-[#e8f4fd] border border-[#231815]/50 overflow-hidden">
              <div
                className="absolute inset-y-0 left-0 rounded-full transition-all duration-700 ease-out"
                style={{
                  width: `${fillPercent}%`,
                  background: "linear-gradient(to right, rgba(67,185,214,0.5), #43b9d6)",
                }}
              />
            </div>
            {/* Tick marks + level labels */}
            <div className="relative h-4 mt-0.5">
              {/* Lv.1 at start */}
              <div
                className="absolute flex flex-col items-center"
                style={{ left: "0%", transform: "translateX(-10%)" }}
              >
                <div className="w-px h-1.5 bg-[#231815]/30" />
                <span className="text-[9px] text-[#231815]/40 leading-none mt-px">
                  Lv.1
                </span>
              </div>
              {/* Lv.2+ at each threshold */}
              {thresholds.slice(1).map((t, i) => {
                const pos = (t / coin_cap) * 100;
                return (
                  <div
                    key={t}
                    className="absolute flex flex-col items-center"
                    style={{ left: `${pos}%`, transform: "translateX(-50%)" }}
                  >
                    <div className="w-px h-1.5 bg-[#231815]/30" />
                    <span className="text-[9px] text-[#231815]/40 leading-none mt-px">
                      Lv.{i + 2}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
          <span className="text-sm font-bold text-[#231815] whitespace-nowrap">
            {total_coins.toLocaleString()} / {coin_cap.toLocaleString()}
          </span>
        </div>

        {/* Remaining */}
        <p className="text-sm font-bold text-[#231815] mt-0.5">
          {isMaxLevel
            ? "최고 레벨 달성!"
            : `${remaining.toLocaleString()} 포인트를 더 모으면 레벨업!`}
        </p>
      </div>
    </div>
  );
}
