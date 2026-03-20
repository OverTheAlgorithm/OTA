import type { LevelInfo } from "@/lib/api";

export function LevelCard({ level }: { level: LevelInfo }) {
  const { total_coins, coin_cap } = level;
  const fillPercent = Math.min((total_coins / coin_cap) * 100, 100);
  const remaining = Math.max(coin_cap - total_coins, 0);

  return (
    <div className="rounded-[22px] bg-white border-[2px] border-[#231815] px-6 py-5 flex items-center gap-5">
      {/* Point icon */}
      <div className="flex-shrink-0 w-[90px] h-[90px] md:w-[108px] md:h-[108px] rounded-full border-[3px] border-[#231815] bg-[#43b9d6]/15 flex items-center justify-center">
        <span className="text-4xl md:text-5xl font-bold text-[#231815]">P</span>
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        {/* Level */}
        <p className="text-xl md:text-2xl font-bold text-[#231815] leading-tight">
          Lv.{level.level}
        </p>

        {/* Points */}
        <div className="flex items-baseline gap-1.5 mt-0.5">
          <span className="text-3xl md:text-[40px] font-bold text-[#231815] leading-tight">
            {total_coins.toLocaleString()}
          </span>
          <span className="text-sm md:text-base font-bold text-[#231815]">
            포인트
          </span>
        </div>

        {/* Progress bar */}
        <div className="flex items-center gap-3 mt-2">
          <div className="relative flex-1 h-[14px] rounded-full bg-[#e8f4fd] border border-[#231815]/50 overflow-hidden">
            <div
              className="absolute inset-y-0 left-0 rounded-full transition-all duration-700 ease-out"
              style={{
                width: `${fillPercent}%`,
                background: "linear-gradient(to right, rgba(67,185,214,0.5), #43b9d6)",
              }}
            />
          </div>
          <span className="text-sm font-bold text-[#231815] whitespace-nowrap">
            {total_coins.toLocaleString()} / {coin_cap.toLocaleString()}
          </span>
        </div>

        {/* Remaining */}
        <p className="text-sm font-bold text-[#231815] mt-1.5">
          {remaining.toLocaleString()} 포인트를 더 모으면 레벨업!
        </p>
      </div>
    </div>
  );
}
