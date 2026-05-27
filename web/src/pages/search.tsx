import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Helmet } from "react-helmet-async";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { Spinner } from "@/components/spinner";
import {
  searchTopics,
  searchEditorPicks,
  defaultImage,
  type TopicPreview,
  type EditorPickCard,
} from "@/lib/api";
import { formatDate } from "@/lib/utils";

const PAGE_SIZE = 10;

export function SearchPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const initialQuery = searchParams.get("q") ?? "";

  // `input` is the live text in the search box; `submittedQuery` is the one
  // driving the API. Decoupled so we don't refetch on every keystroke.
  const [input, setInput] = useState(initialQuery);
  const [submittedQuery, setSubmittedQuery] = useState(initialQuery);

  // 기사 (news topic) results
  const [topicResults, setTopicResults] = useState<TopicPreview[]>([]);
  const [topicOffset, setTopicOffset] = useState(0);
  const [topicHasMore, setTopicHasMore] = useState(false);
  const [topicLoadingMore, setTopicLoadingMore] = useState(false);

  // 에디터 픽 results
  const [pickResults, setPickResults] = useState<EditorPickCard[]>([]);
  const [pickOffset, setPickOffset] = useState(0);
  const [pickHasMore, setPickHasMore] = useState(false);
  const [pickLoadingMore, setPickLoadingMore] = useState(false);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Sentinel for IntersectionObserver-driven infinite scroll on the 기사 list.
  // (Editor picks tend to be few; we use a "더 보기" button there.)
  const topicSentinelRef = useRef<HTMLDivElement>(null);

  // Reload from scratch whenever the submitted query changes.
  useEffect(() => {
    const q = submittedQuery.trim();
    if (!q) {
      setTopicResults([]);
      setTopicOffset(0);
      setTopicHasMore(false);
      setPickResults([]);
      setPickOffset(0);
      setPickHasMore(false);
      setLoading(false);
      setError(null);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setError(null);
    setTopicResults([]);
    setTopicOffset(0);
    setTopicHasMore(false);
    setPickResults([]);
    setPickOffset(0);
    setPickHasMore(false);

    // Fire both searches in parallel — they target independent tables and
    // have no shared state.
    Promise.allSettled([
      searchTopics(q, PAGE_SIZE, 0),
      searchEditorPicks(q, PAGE_SIZE, 0),
    ])
      .then(([topicRes, pickRes]) => {
        if (cancelled) return;
        if (topicRes.status === "fulfilled") {
          setTopicResults(topicRes.value.data);
          setTopicHasMore(topicRes.value.has_more);
          setTopicOffset(topicRes.value.data.length);
        }
        if (pickRes.status === "fulfilled") {
          setPickResults(pickRes.value.items);
          setPickHasMore(pickRes.value.has_more);
          setPickOffset(pickRes.value.items.length);
        }
        if (topicRes.status === "rejected" && pickRes.status === "rejected") {
          setError("검색에 실패했습니다. 잠시 후 다시 시도해주세요.");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [submittedQuery]);

  const loadMoreTopics = useCallback(async () => {
    if (topicLoadingMore || !topicHasMore) return;
    const q = submittedQuery.trim();
    if (!q) return;

    setTopicLoadingMore(true);
    try {
      const page = await searchTopics(q, PAGE_SIZE, topicOffset);
      setTopicResults((prev) => [...prev, ...page.data]);
      setTopicHasMore(page.has_more);
      setTopicOffset((prev) => prev + page.data.length);
    } catch {
      setError("검색에 실패했습니다.");
    } finally {
      setTopicLoadingMore(false);
    }
  }, [topicHasMore, topicLoadingMore, topicOffset, submittedQuery]);

  const loadMorePicks = useCallback(async () => {
    if (pickLoadingMore || !pickHasMore) return;
    const q = submittedQuery.trim();
    if (!q) return;

    setPickLoadingMore(true);
    try {
      const page = await searchEditorPicks(q, PAGE_SIZE, pickOffset);
      setPickResults((prev) => [...prev, ...page.items]);
      setPickHasMore(page.has_more);
      setPickOffset((prev) => prev + page.items.length);
    } catch {
      setError("검색에 실패했습니다.");
    } finally {
      setPickLoadingMore(false);
    }
  }, [pickHasMore, pickLoadingMore, pickOffset, submittedQuery]);

  // Wire up infinite scroll for the topics list only.
  useEffect(() => {
    const el = topicSentinelRef.current;
    if (!el || !topicHasMore) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) loadMoreTopics();
      },
      { rootMargin: "200px" },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [topicHasMore, loadMoreTopics]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = input.trim();
    setSubmittedQuery(trimmed);
    if (trimmed) {
      setSearchParams({ q: trimmed }, { replace: true });
    } else {
      setSearchParams({}, { replace: true });
    }
  };

  const hasAnyResult = topicResults.length > 0 || pickResults.length > 0;

  return (
    <div className="min-h-screen flex flex-col bg-[#ffffff]">
      <Helmet>
        <title>
          {submittedQuery ? `"${submittedQuery}" 검색 결과` : "검색"} | 위즈레터
        </title>
        <meta name="robots" content="noindex" />
      </Helmet>
      <Header />

      <main className="flex-1 max-w-[900px] w-full mx-auto px-6 py-8">
        <h1 className="text-2xl font-bold text-[#231815] mb-5">소식 검색</h1>

        <form onSubmit={handleSubmit} className="flex gap-2 mb-8">
          <input
            type="search"
            autoFocus
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="키워드를 입력하세요 (예: 삼성, AI, 환율)"
            className="flex-1 h-11 px-4 rounded-full border border-[#231815]/40 bg-white text-[#231815] placeholder:text-[#231815]/40 focus:outline-none focus:ring-2 focus:ring-[#43b9d6]"
          />
          <button
            type="submit"
            disabled={!input.trim()}
            className="h-11 px-6 rounded-full text-sm font-semibold text-[#231815] border border-[#231815] bg-[#43b9d6] hover:brightness-110 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
          >
            검색
          </button>
        </form>

        {error && (
          <p className="mb-6 rounded-lg border border-red-400 bg-red-50 px-4 py-3 text-sm text-red-700">
            {error}
          </p>
        )}

        {loading ? (
          <div className="py-12 flex justify-center">
            <Spinner />
          </div>
        ) : !submittedQuery ? (
          <p className="text-center py-12 text-[#231815]/50">
            제목과 본문에서 키워드를 찾아 드려요. 한글/영문 모두 지원합니다.
          </p>
        ) : !hasAnyResult ? (
          <p className="text-center py-12 text-[#231815]/60">
            "{submittedQuery}"와 일치하는 소식이 없습니다.
          </p>
        ) : (
          <div className="space-y-10">
            {/* ── 기사 (news topics) ── */}
            <section>
              <SectionHeader
                title="기사"
                emoji="📰"
                count={topicResults.length}
                hasMore={topicHasMore}
              />
              {topicResults.length === 0 ? (
                <p className="py-6 text-sm text-[#231815]/50">
                  일치하는 기사가 없습니다.
                </p>
              ) : (
                <>
                  <ul className="divide-y divide-[#231815]/10 border-y border-[#231815]/10">
                    {topicResults.map((topic) => (
                      <TopicResultRow
                        key={topic.id}
                        topic={topic}
                        query={submittedQuery}
                      />
                    ))}
                  </ul>
                  <div
                    ref={topicSentinelRef}
                    className="h-12 flex items-center justify-center"
                  >
                    {topicLoadingMore && <Spinner size="sm" />}
                    {!topicHasMore && topicResults.length > 0 && (
                      <span className="text-xs text-[#231815]/40">
                        — 기사 끝 —
                      </span>
                    )}
                  </div>
                </>
              )}
            </section>

            {/* ── 에디터 픽 ── */}
            <section>
              <SectionHeader
                title="에디터 픽"
                emoji="📝"
                count={pickResults.length}
                hasMore={pickHasMore}
              />
              {pickResults.length === 0 ? (
                <p className="py-6 text-sm text-[#231815]/50">
                  일치하는 에디터 픽이 없습니다.
                </p>
              ) : (
                <>
                  <ul className="divide-y divide-[#231815]/10 border-y border-[#231815]/10">
                    {pickResults.map((card) => (
                      <EditorPickResultRow
                        key={card.id}
                        card={card}
                        query={submittedQuery}
                      />
                    ))}
                  </ul>
                  {pickHasMore && (
                    <div className="flex justify-center mt-4">
                      <button
                        type="button"
                        onClick={loadMorePicks}
                        disabled={pickLoadingMore}
                        className="px-6 h-10 rounded-full border border-[#231815]/30 text-sm font-medium text-[#231815] hover:bg-[#231815]/5 disabled:opacity-60"
                      >
                        {pickLoadingMore ? "불러오는 중..." : "에디터 픽 더 보기"}
                      </button>
                    </div>
                  )}
                </>
              )}
            </section>
          </div>
        )}
      </main>

      <Footer />
    </div>
  );
}

function SectionHeader({
  title,
  emoji,
  count,
  hasMore,
}: {
  title: string;
  emoji: string;
  count: number;
  hasMore: boolean;
}) {
  return (
    <div className="flex items-baseline gap-3 mb-3">
      <h2 className="text-lg font-bold text-[#231815] flex items-center gap-2">
        <span aria-hidden>{emoji}</span> {title}
      </h2>
      <span className="text-sm text-[#231815]/50">
        {count}
        {hasMore ? "+" : ""}건
      </span>
    </div>
  );
}

function TopicResultRow({
  topic,
  query,
}: {
  topic: TopicPreview;
  query: string;
}) {
  return (
    <li>
      <Link
        to={`/topic/${topic.id}`}
        className="flex gap-4 py-4 hover:opacity-70 transition-opacity"
      >
        <div className="w-24 h-24 sm:w-28 sm:h-28 shrink-0 overflow-hidden rounded-md bg-[#f5f5f5]">
          <img
            src={topic.image_url || defaultImage}
            alt={topic.topic}
            className="w-full h-full object-cover"
            onError={(e) => {
              if (e.currentTarget.src !== defaultImage)
                e.currentTarget.src = defaultImage;
            }}
          />
        </div>
        <div className="flex-1 min-w-0">
          {topic.created_at && (
            <p className="text-xs text-[#231815]/50 mb-1">
              {formatDate(topic.created_at)}
            </p>
          )}
          <h3 className="text-base font-bold text-[#231815] line-clamp-2 leading-snug">
            <Highlighted text={topic.topic} query={query} />
          </h3>
          {topic.summary && (
            <p className="mt-1.5 text-sm text-[#231815]/70 line-clamp-2 leading-relaxed">
              <Highlighted text={topic.summary} query={query} />
            </p>
          )}
        </div>
      </Link>
    </li>
  );
}

function EditorPickResultRow({
  card,
  query,
}: {
  card: EditorPickCard;
  query: string;
}) {
  const thumbnail = card.first_image_url || defaultImage;
  return (
    <li>
      <Link
        to={`/editor-picks/${card.id}`}
        className="flex gap-4 py-4 hover:opacity-70 transition-opacity"
      >
        <div className="w-24 h-24 sm:w-28 sm:h-28 shrink-0 overflow-hidden rounded-md bg-[#f5f5f5]">
          <img
            src={thumbnail}
            alt={card.title}
            className="w-full h-full object-cover"
            onError={(e) => {
              if (e.currentTarget.src !== defaultImage)
                e.currentTarget.src = defaultImage;
            }}
          />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-xs text-[#231815]/50 mb-1">
            {card.author_name ? `${card.author_name} · ` : ""}
            {formatDate(card.published_at)}
          </p>
          <h3 className="text-base font-bold text-[#231815] line-clamp-2 leading-snug">
            <Highlighted text={card.title} query={query} />
          </h3>
          {card.excerpt && (
            <p className="mt-1.5 text-sm text-[#231815]/70 line-clamp-2 leading-relaxed">
              <Highlighted text={card.excerpt} query={query} />
            </p>
          )}
        </div>
      </Link>
    </li>
  );
}

// escapeRegExp protects user-supplied search text from being interpreted as a
// regex. Critical when the query contains characters like ".", "*", "(", etc.
function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function Highlighted({ text, query }: { text: string; query: string }) {
  const q = query.trim();
  if (!q) return <>{text}</>;

  // Splitting with a captured group puts each match at an odd index. We use
  // that index parity for marking instead of re.test, which would advance the
  // shared regex's lastIndex when /g is set.
  const re = new RegExp(`(${escapeRegExp(q)})`, "gi");
  const parts = text.split(re);

  return (
    <>
      {parts.map((part, i) =>
        i % 2 === 1 ? (
          <mark
            key={i}
            className="bg-[#fff3a1] text-inherit rounded-sm px-0.5"
          >
            {part}
          </mark>
        ) : (
          <span key={i}>{part}</span>
        ),
      )}
    </>
  );
}
