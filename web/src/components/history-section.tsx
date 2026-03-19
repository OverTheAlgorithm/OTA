import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import type { HistoryEntry, HistoryItem, BrainCategory } from "@/lib/api";
import { getContextHistory, getBrainCategories } from "@/lib/api";

const PAGE_SIZE = 10;

interface Props {
  subscriptions: string[];
  onFirstLoad?: () => void;
}

function formatDate(dateStr: string): string {
  const [y, m, d] = dateStr.split("-");
  return `${y}.${m}.${d}`;
}

function BuzzBadge({ score }: { score: number }) {
  if (!score) return null;
  return (
    <span className="text-xs font-bold text-[#ff5442]">
      🔥 화제도 {score}
    </span>
  );
}

function TopicRow({ item, accent }: { item: HistoryItem; accent?: string }) {
  const dotColor = accent ?? "#6b8db5";
  const hasDetails = item.details && item.details.length > 0;
  return (
    <li className="flex gap-3 py-2.5 border-b border-[#d4e6f5]/60 last:border-0">
      <span
        className="mt-2 w-1.5 h-1.5 rounded-full shrink-0"
        style={{ backgroundColor: dotColor + "99" }}
      />
      <div className="min-w-0">
        <BuzzBadge score={item.buzz_score} />
        <p className="text-sm font-semibold text-[#1e3a5f] leading-snug">
          {item.topic}
        </p>
        <p className="text-xs text-[#6b8db5] mt-1 leading-relaxed">{item.summary}</p>
        {hasDetails && (
          <Link
            to={`/topic/${item.id}`}
            className="inline-block mt-1 text-xs transition-colors"
            style={{ color: "var(--color-text-secondary)" }}
            onMouseEnter={e => (e.currentTarget.style.color = "var(--color-fg)")}
            onMouseLeave={e => (e.currentTarget.style.color = "var(--color-text-secondary)")}
          >
            {item.details.length}개의 추가 정보가 있어요 →
          </Link>
        )}
      </div>
    </li>
  );
}

function groupByBrainCategory(items: HistoryItem[]): Record<string, HistoryItem[]> {
  const groups: Record<string, HistoryItem[]> = {};
  for (const item of items) {
    const key = item.brain_category || "";
    if (!groups[key]) groups[key] = [];
    groups[key].push(item);
  }
  return groups;
}

function HistoryCard({
  entry,
  subscriptions,
  brainCategories,
  defaultOpen,
}: {
  entry: HistoryEntry;
  subscriptions: string[];
  brainCategories: BrainCategory[];
  defaultOpen: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);

  // Filter: always include top + brief priority, plus subscribed categories
  const subSet = new Set(subscriptions);
  const selectedItems = entry.items.filter(
    (i) => i.priority === "top" || i.priority === "brief" || subSet.has(i.category),
  );

  const bcGroups = groupByBrainCategory(selectedItems);

  return (
    <div className="rounded-2xl bg-[#f0f7ff] border border-[#d4e6f5] overflow-hidden">
      <button
        onClick={() => setOpen((o) => !o)}
        className="w-full px-6 py-4 flex items-center justify-between cursor-pointer"
        style={{ borderBottom: open ? "1px solid var(--color-border)" : "none" }}
      >
        <span className="font-semibold text-[#1e3a5f]">{formatDate(entry.date)}</span>
        <div className="flex items-center gap-2">
          <span className="text-xs text-[#6b8db5] bg-white px-2.5 py-1 rounded-full border border-[#d4e6f5]">
            {selectedItems.length}개 토픽
          </span>
          <svg
            className="w-4 h-4 text-[#6b8db5] transition-transform duration-200"
            style={{ transform: open ? "rotate(180deg)" : "rotate(0deg)" }}
            viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
            strokeLinecap="round" strokeLinejoin="round"
          >
            <polyline points="6 9 12 15 18 9" />
          </svg>
        </div>
      </button>

      {open && <div className="p-6 space-y-5">
        {/* Render brain category sections in display_order */}
        {brainCategories.map((bc) => {
          const items = bcGroups[bc.key];
          if (!items || items.length === 0) return null;
          return (
            <div key={bc.key}>
              <div className="flex items-center gap-2 mb-3">
                <div
                  className="w-6 h-6 rounded-md flex items-center justify-center text-sm"
                  style={{ backgroundColor: bc.accent_color + "15" }}
                >
                  {bc.emoji}
                </div>
                <span
                  className="text-xs font-semibold tracking-wider"
                  style={{ color: bc.accent_color }}
                >
                  {bc.label}
                </span>
              </div>
              <ul>
                {items.map((item, i) => (
                  <TopicRow key={i} item={item} accent={bc.accent_color} />
                ))}
              </ul>
            </div>
          );
        })}

        {/* Ungrouped items (no brain_category) */}
        {bcGroups[""] && bcGroups[""].length > 0 && (
          <div>
            <div className="flex items-center gap-2 mb-3">
              <div className="w-6 h-6 rounded-md bg-[#6b8db5]/10 flex items-center justify-center">
                <span className="text-sm">📌</span>
              </div>
              <span className="text-xs font-semibold text-[#6b8db5] tracking-wider">
                기타
              </span>
            </div>
            <ul>
              {bcGroups[""].map((item, i) => (
                <TopicRow key={i} item={item} accent="#9b8bb4" />
              ))}
            </ul>
          </div>
        )}
      </div>}
    </div>
  );
}

export function HistorySection({ subscriptions, onFirstLoad }: Props) {
  const [entries, setEntries] = useState<HistoryEntry[]>([]);
  const [brainCategories, setBrainCategories] = useState<BrainCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [offset, setOffset] = useState(0);
  const sentinelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    getBrainCategories().then(setBrainCategories).catch(() => {});
    getContextHistory(PAGE_SIZE, 0)
      .then(({ data, has_more }) => {
        setEntries(data);
        setHasMore(has_more);
        setOffset(data.length);
      })
      .catch(() => {})
      .finally(() => {
        setLoading(false);
        onFirstLoad?.();
      });
  }, []);

  const loadMore = useCallback(async () => {
    if (loadingMore || !hasMore) return;
    setLoadingMore(true);
    try {
      const { data, has_more } = await getContextHistory(PAGE_SIZE, offset);
      setEntries((prev) => [...prev, ...data]);
      setHasMore(has_more);
      setOffset((prev) => prev + data.length);
    } catch {
      // silently fail — user can scroll again to retry
    } finally {
      setLoadingMore(false);
    }
  }, [loadingMore, hasMore, offset]);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) loadMore();
      },
      { rootMargin: "200px" },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [loadMore]);

  return (
    <section>
      <div className="flex items-center gap-2 mb-4">
        <div className="w-8 h-8 rounded-lg bg-[#4a9fe5]/10 flex items-center justify-center">
          <svg className="w-4 h-4 text-[#4a9fe5]" viewBox="0 0 24 24" fill="none"
            stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
            <line x1="16" y1="13" x2="8" y2="13"/>
            <line x1="16" y1="17" x2="8" y2="17"/>
            <polyline points="10 9 9 9 8 9"/>
          </svg>
        </div>
        <h2 className="font-semibold text-[#1e3a5f]">받아본 맥락</h2>
      </div>

      {loading ? (
        <div className="space-y-4">
          {[1, 2].map((i) => (
            <div key={i} className="rounded-2xl bg-[#f0f7ff] border border-[#d4e6f5] p-6 animate-pulse">
              <div className="h-4 bg-[#d4e6f5] rounded w-24 mb-4" />
              <div className="space-y-2">
                <div className="h-3 bg-[#d4e6f5] rounded w-full" />
                <div className="h-3 bg-[#d4e6f5] rounded w-3/4" />
              </div>
            </div>
          ))}
        </div>
      ) : entries.length === 0 ? (
        <div className="rounded-2xl bg-[#f0f7ff] border border-[#d4e6f5] p-12 text-center">
          <p className="text-[#6b8db5] text-sm">아직 받은 맥락이 없습니다.</p>
          <p className="text-[#6b8db5]/60 text-xs mt-1">매일 아침 7시에 첫 브리핑이 전달됩니다.</p>
        </div>
      ) : (
        <div className="space-y-4">
          {entries.map((entry, i) => (
            <HistoryCard
              key={entry.date}
              entry={entry}
              subscriptions={subscriptions}
              brainCategories={brainCategories}
              defaultOpen={i === 0}
            />
          ))}

          {/* Sentinel for infinite scroll */}
          <div ref={sentinelRef} className="h-1" />

          {loadingMore && (
            <div className="flex justify-center py-4">
              <div className="flex items-center gap-2 text-[#6b8db5] text-sm">
                <svg className="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
                이전 맥락을 불러오는 중...
              </div>
            </div>
          )}

          {!hasMore && entries.length > 0 && (
            <p className="text-center text-xs text-[#6b8db5]/60 py-2">
              모든 맥락을 불러왔습니다
            </p>
          )}
        </div>
      )}
    </section>
  );
}
