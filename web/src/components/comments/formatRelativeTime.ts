// Minimal Korean relative-time formatter for comment timestamps.
// Pulled into its own file so the comment-list rendering can stay pure.

export function formatRelativeTime(iso: string, now: Date = new Date()): string {
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "";
  const diffMs = now.getTime() - t;
  if (diffMs < 0) return "방금";
  const sec = Math.floor(diffMs / 1000);
  if (sec < 60) return "방금";
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}분 전`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}시간 전`;
  const day = Math.floor(hr / 24);
  if (day < 7) return `${day}일 전`;
  const wk = Math.floor(day / 7);
  if (wk < 5) return `${wk}주 전`;
  const mo = Math.floor(day / 30);
  if (mo < 12) return `${mo}개월 전`;
  const yr = Math.floor(day / 365);
  return `${yr}년 전`;
}
