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
  const isMax = level.points_to_next === 0;
  const progressPercent = isMax
    ? 100
    : Math.round((level.current_progress / level.points_to_next) * 100);

  return (
    <div className="rounded-2xl bg-gradient-to-br from-[#1a1229] to-[#1e1530] border border-[#2d1f42] px-4 py-3 flex items-center gap-3">
      <img
        src={imgSrc}
        alt={`Level ${level.level}`}
        className="w-44 h-44 flex-shrink-0 object-contain"
      />
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline gap-2 mb-2">
          <span className="text-xl font-bold text-[#f5f0ff]">Lv.{level.level}</span>
          <span className="text-xs text-[#9b8bb4]">
            {isMax ? "MAX" : `${level.current_progress} / ${level.points_to_next}`}
          </span>
        </div>
        <div className="w-full h-2 rounded-full bg-[#2d1f42] overflow-hidden mb-2">
          <div
            className="h-full rounded-full bg-gradient-to-r from-[#5ba4d9] to-[#7bc67e] transition-all duration-500"
            style={{ width: `${progressPercent}%` }}
          />
        </div>
        <p className="text-sm text-[#9b8bb4] truncate">{level.description}</p>
        <p className="text-xs text-[#4a3d5c] mt-2">🌈 Over the Algorithm 토픽을 읽으면 포인트가 쌓여요</p>
      </div>
    </div>
  );
}
