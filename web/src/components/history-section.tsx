import { useState } from "react";
import { Link } from "react-router-dom";
import type { HistoryEntry, HistoryItem } from "@/lib/api";

interface Props {
  entries: HistoryEntry[];
  subscriptions: string[];
  loading: boolean;
}

function formatDate(dateStr: string): string {
  const [y, m, d] = dateStr.split("-");
  return `${y}.${m}.${d}`;
}

function BuzzBadge({ score }: { score: number }) {
  if (!score) return null;
  return (
    <span className="text-xs font-bold text-[#e84d3d]">
      🔥 화제도 {score}
    </span>
  );
}

function TopicRow({ item, accent }: { item: HistoryItem; accent?: string }) {
  const dotColor = accent ?? "#9b8bb4";
  const hasDetails = item.details && item.details.length > 0;
  return (
    <li className="flex gap-3 py-2.5 border-b border-[#2d1f42]/60 last:border-0">
      <span
        className="mt-2 w-1.5 h-1.5 rounded-full shrink-0"
        style={{ backgroundColor: dotColor + "99" }}
      />
      <div className="min-w-0">
        <BuzzBadge score={item.buzz_score} />
        <p className="text-sm font-semibold text-[#f5f0ff] leading-snug">
          {item.topic}
        </p>
        <p className="text-xs text-[#d4cee0] mt-1 leading-relaxed">{item.summary}</p>
        {hasDetails && (
          <Link
            to={`/topic/${item.id}`}
            className="inline-block mt-1 text-xs transition-colors"
            style={{ color: "#9b8bb4" }}
            onMouseEnter={e => (e.currentTarget.style.color = "#f5f0ff")}
            onMouseLeave={e => (e.currentTarget.style.color = "#9b8bb4")}
          >
            {item.details.length}개의 추가 정보가 있어요 →
          </Link>
        )}
      </div>
    </li>
  );
}

function HistoryCard({
  entry,
  subscriptions,
  defaultOpen,
}: {
  entry: HistoryEntry;
  subscriptions: string[];
  defaultOpen: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const topItems = entry.items.filter((i) => i.category === "top");
  const briefItems = entry.items.filter((i) => i.category === "brief");
  const interestItems = entry.items.filter(
    (i) => i.category !== "top" && i.category !== "brief" && subscriptions.includes(i.category),
  );

  return (
    <div className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] overflow-hidden">
      <button
        onClick={() => setOpen((o) => !o)}
        className="w-full px-6 py-4 flex items-center justify-between cursor-pointer"
        style={{ borderBottom: open ? "1px solid #2d1f42" : "none" }}
      >
        <span className="font-semibold text-[#f5f0ff]">{formatDate(entry.date)}</span>
        <div className="flex items-center gap-2">
          <span className="text-xs text-[#9b8bb4] bg-[#0f0a19] px-2.5 py-1 rounded-full border border-[#2d1f42]">
            {topItems.length + briefItems.length + interestItems.length}개 토픽
          </span>
          <svg
            className="w-4 h-4 text-[#9b8bb4] transition-transform duration-200"
            style={{ transform: open ? "rotate(180deg)" : "rotate(0deg)" }}
            viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
            strokeLinecap="round" strokeLinejoin="round"
          >
            <polyline points="6 9 12 15 18 9" />
          </svg>
        </div>
      </button>

      {open && <div className="p-6 space-y-5">
        {topItems.length > 0 && (
          <div>
            <div className="flex items-center gap-2 mb-3">
              <div className="w-6 h-6 rounded-md bg-[#e84d3d]/10 flex items-center justify-center">
                <svg className="w-3.5 h-3.5 text-[#e84d3d]" viewBox="0 0 24 24" fill="none"
                  stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="10"/>
                  <path d="M2 12h20"/>
                  <path d="M12 2a15 15 0 014 10 15 15 0 01-4 10 15 15 0 01-4-10 15 15 0 014-10z"/>
                </svg>
              </div>
              <span className="text-xs font-semibold text-[#e84d3d] uppercase tracking-wider">
                대화 소재
              </span>
            </div>
            <ul>
              {topItems.map((item, i) => (
                <TopicRow key={i} item={item} accent="#e84d3d" />
              ))}
            </ul>
          </div>
        )}

        {interestItems.length > 0 && (
          <div>
            <div className="flex items-center gap-2 mb-3">
              <div className="w-6 h-6 rounded-md bg-[#5ba4d9]/10 flex items-center justify-center">
                <svg className="w-3.5 h-3.5 text-[#5ba4d9]" viewBox="0 0 24 24" fill="none"
                  stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/>
                </svg>
              </div>
              <span className="text-xs font-semibold text-[#5ba4d9] uppercase tracking-wider">
                내 관심사
              </span>
            </div>
            <ul>
              {interestItems.map((item, i) => {
                const hasDetails = item.details && item.details.length > 0;
                return (
                  <li key={i} className="flex gap-3 py-2.5 border-b border-[#2d1f42]/60 last:border-0">
                    <span className="mt-2 w-1.5 h-1.5 rounded-full bg-[#5ba4d9]/60 shrink-0" />
                    <div className="min-w-0">
                      <div className="flex items-center gap-1.5 mb-0.5">
                        <span className="text-xs px-1.5 py-0.5 rounded bg-[#5ba4d9]/10 text-[#5ba4d9] border border-[#5ba4d9]/20">
                          {item.category}
                        </span>
                      </div>
                      <BuzzBadge score={item.buzz_score} />
                      <p className="text-sm font-semibold text-[#f5f0ff] leading-snug">
                        {item.topic}
                      </p>
                      <p className="text-xs text-[#d4cee0] mt-1 leading-relaxed">{item.summary}</p>
                      {hasDetails && (
                        <Link
                          to={`/topic/${item.id}`}
                          className="inline-block mt-1 text-xs transition-colors"
                          style={{ color: "#9b8bb4" }}
                          onMouseEnter={e => (e.currentTarget.style.color = "#f5f0ff")}
                          onMouseLeave={e => (e.currentTarget.style.color = "#9b8bb4")}
                        >
                          {item.details.length}개의 추가 정보가 있어요 →
                        </Link>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          </div>
        )}

        {briefItems.length > 0 && (
          <div className="pt-2">
            <div className="flex items-center gap-2 mb-2">
              <div className="w-5 h-5 rounded-md bg-[#4a3d5c]/20 flex items-center justify-center">
                <span className="text-xs">💡</span>
              </div>
              <span className="text-[10px] font-semibold text-[#4a3d5c] uppercase tracking-wider">
                알아두면 좋은 것
              </span>
            </div>
            <ul>
              {briefItems.map((item, i) => (
                <li key={i} className="flex gap-2.5 py-1.5">
                  <span className="mt-1.5 w-1 h-1 rounded-full bg-[#4a3d5c] shrink-0" />
                  <div className="min-w-0">
                    <p className="text-xs font-medium text-[#9b8bb4] leading-snug">
                      {item.topic}
                    </p>
                    <p className="text-xs text-[#6b5f80] mt-0.5 leading-relaxed">{item.summary}</p>
                  </div>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>}
    </div>
  );
}

export function HistorySection({ entries, subscriptions, loading }: Props) {
  return (
    <section>
      <div className="flex items-center gap-2 mb-4">
        <div className="w-8 h-8 rounded-lg bg-[#7bc67e]/10 flex items-center justify-center">
          <svg className="w-4 h-4 text-[#7bc67e]" viewBox="0 0 24 24" fill="none"
            stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
            <line x1="16" y1="13" x2="8" y2="13"/>
            <line x1="16" y1="17" x2="8" y2="17"/>
            <polyline points="10 9 9 9 8 9"/>
          </svg>
        </div>
        <h2 className="font-semibold text-[#f5f0ff]">받아본 맥락</h2>
      </div>

      {loading ? (
        <div className="space-y-4">
          {[1, 2].map((i) => (
            <div key={i} className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] p-6 animate-pulse">
              <div className="h-4 bg-[#2d1f42] rounded w-24 mb-4" />
              <div className="space-y-2">
                <div className="h-3 bg-[#2d1f42] rounded w-full" />
                <div className="h-3 bg-[#2d1f42] rounded w-3/4" />
              </div>
            </div>
          ))}
        </div>
      ) : entries.length === 0 ? (
        <div className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] p-12 text-center">
          <p className="text-[#9b8bb4] text-sm">아직 받은 맥락이 없습니다.</p>
          <p className="text-[#9b8bb4]/60 text-xs mt-1">매일 아침 7시에 첫 브리핑이 전달됩니다.</p>
        </div>
      ) : (
        <div className="space-y-4">
          {entries.map((entry, i) => (
            <HistoryCard key={entry.date} entry={entry} subscriptions={subscriptions} defaultOpen={i === 0} />
          ))}
        </div>
      )}
    </section>
  );
}
