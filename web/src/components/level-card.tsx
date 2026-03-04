import type { LevelInfo } from "@/lib/api";

const LEVEL_IMAGES = [
  "/rainbow_lv1.png",
  "/rainbow_lv2.png",
  "/rainbow_lv3.png",
  "/rainbow_lv4.png",
  "/rainbow_lv5.png",
];

export function LevelCard({ level }: { level: LevelInfo }) {
  const imgSrc = LEVEL_IMAGES[level.level - 1] ?? LEVEL_IMAGES[0];
  const isMax = level.coins_to_next === 0;
  const progressPercent = isMax
    ? 100
    : Math.round((level.current_progress / level.coins_to_next) * 100);

  return (
    <div className="rounded-2xl bg-gradient-to-br from-[#f0f7ff] to-[#e8f4fd] border border-[#d4e6f5] px-4 py-3 flex items-center gap-3">
      <img
        src={imgSrc}
        alt={`Level ${level.level}`}
        className="w-44 h-44 flex-shrink-0 object-contain"
      />
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline gap-2 mb-2">
          <span className="text-xl font-bold text-[#1e3a5f]">Lv.{level.level}</span>
          <span className="text-xs text-[#6b8db5]">
            {isMax ? "MAX" : `${level.current_progress} / ${level.coins_to_next}`}
          </span>
        </div>
        <div className="w-full h-2 rounded-full bg-[#d4e6f5] overflow-hidden mb-2">
          <div
            className="h-full rounded-full bg-gradient-to-r from-[#4a9fe5] to-[#3498db] transition-all duration-500"
            style={{ width: `${progressPercent}%` }}
          />
        </div>
        <p className="text-sm text-[#6b8db5] truncate">{level.description}</p>
        <p className="text-xs text-[#335071] mt-2">🌈 Over the Algorithm 토픽을 읽으면 코인이 쌓여요</p>
      </div>
    </div>
  );
}
