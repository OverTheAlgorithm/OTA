import type { LevelInfo } from "@/lib/api";

const LEVEL_IMAGES = [
  "/rainbow_lv1.png",
  "/rainbow_lv2.png",
  "/rainbow_lv3.png",
  "/rainbow_lv4.png",
  "/rainbow_lv5.png",
];

// Colors per level segment: calm greens → intense reds
const LEVEL_COLORS = [
  "#4ade80", // Lv1 — green
  "#facc15", // Lv2 — yellow
  "#fb923c", // Lv3 — orange
  "#f87171", // Lv4 — red-light
  "#ef4444", // Lv5 — red
];

export function LevelCard({ level }: { level: LevelInfo }) {
  const imgSrc = LEVEL_IMAGES[level.level - 1] ?? LEVEL_IMAGES[0];
  const { thresholds, coin_cap, total_coins } = level;
  const maxLevel = thresholds.length;
  const fillPercent = Math.min((total_coins / coin_cap) * 100, 100);

  return (
    <div className="rounded-2xl bg-gradient-to-br from-[#f0f7ff] to-[#e8f4fd] border border-[#d4e6f5] px-4 py-3 flex items-center gap-3">
      <img
        src={imgSrc}
        alt={`Level ${level.level}`}
        className="w-44 h-44 flex-shrink-0 object-contain"
      />
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline gap-2 mb-2">
          <span className="text-xl font-bold text-[#1e3a5f]">
            Lv.{level.level}
          </span>
          <span className="text-xs text-[#6b8db5]">
            {total_coins.toLocaleString()} / {coin_cap.toLocaleString()} 코인
          </span>
        </div>

        {/* Segmented progress bar */}
        <div className="relative w-full h-3 rounded-full bg-[#e2e8f0] overflow-hidden mb-2">
          {/* Tick marks between segments */}
          {thresholds.slice(1).map((th, i) => {
            const pos = (th / coin_cap) * 100;
            return (
              <div
                key={i}
                className="absolute top-0 bottom-0 w-[2px] bg-white/70 z-20"
                style={{ left: `${pos}%` }}
              />
            );
          })}

          {/* Colored fill — gradient through level colors */}
          <div
            className="absolute inset-y-0 left-0 z-10 rounded-full transition-all duration-700 ease-out"
            style={{
              width: `${fillPercent}%`,
              background: buildGradient(
                total_coins,
                thresholds,
                coin_cap,
                maxLevel,
              ),
            }}
          />
        </div>

        {/* Level labels under the bar */}
        <div className="relative w-full flex text-[10px] text-[#94a3b8] mb-4">
          {thresholds.map((th, i) => {
            const pos = (th / coin_cap) * 100;
            return (
              <span
                key={i}
                className="absolute -translate-x-1/2"
                style={{ left: `${pos}%` }}
              >
                Lv{i + 1}
              </span>
            );
          })}
        </div>

        <p className="text-sm text-[#6b8db5] truncate mt-2">
          {level.description}
        </p>
        <p className="text-xs text-[#335071] mt-1">
          레벨이 올라가면 하루에 얻을 수 있는 코인의 양이 늘어나요!
        </p>
        {level.daily_limit > 0 && (
          <p className="text-xs text-[#6b8db5] mt-0.5">
            오늘 획득 한도: {level.daily_limit} 코인
          </p>
        )}
      </div>
    </div>
  );
}

/**
 * Build a CSS linear-gradient that colors each level segment differently,
 * but only up to the current fill position.
 */
function buildGradient(
  totalCoins: number,
  thresholds: number[],
  coinCap: number,
  maxLevel: number,
): string {
  const stops: string[] = [];

  for (let i = 0; i < maxLevel; i++) {
    const segStart = thresholds[i];
    const segEnd = i + 1 < maxLevel ? thresholds[i + 1] : coinCap;
    const color = LEVEL_COLORS[i] ?? LEVEL_COLORS[LEVEL_COLORS.length - 1];

    if (totalCoins <= segStart) break;

    const startPct = (segStart / coinCap) * 100;
    const endPct = (Math.min(totalCoins, segEnd) / coinCap) * 100;

    stops.push(`${color} ${startPct}%`);
    stops.push(`${color} ${endPct}%`);
  }

  if (stops.length === 0) return LEVEL_COLORS[0];
  return `linear-gradient(to right, ${stops.join(", ")})`;
}
