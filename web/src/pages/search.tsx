import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Helmet } from "react-helmet-async";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { Spinner } from "@/components/spinner";
import { searchTopics, defaultImage, type TopicPreview } from "@/lib/api";
import { formatDate } from "@/lib/utils";

const PAGE_SIZE = 10;

export function SearchPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const initialQuery = searchParams.get("q") ?? "";

  // input is the live text in the search box; submittedQuery is the one driving
  // the API. Decoupled so we don't refetch on every keystroke.
  const [input, setInput] = useState(initialQuery);
  const [submittedQuery, setSubmittedQuery] = useState(initialQuery);

  const [results, setResults] = useState<TopicPreview[]>([]);
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Sentinel for IntersectionObserver-driven infinite scroll.
  const sentinelRef = useRef<HTMLDivElement>(null);

  // Reload from scratch whenever the submitted query changes.
  useEffect(() => {
    const q = submittedQuery.trim();
    if (!q) {
      setResults([]);
      setOffset(0);
      setHasMore(false);
      setLoading(false);
      setError(null);
      return;
    }

    setLoading(true);
    setError(null);
    setResults([]);
    setOffset(0);

    searchTopics(q, PAGE_SIZE, 0)
      .then((page) => {
        setResults(page.data);
        setHasMore(page.has_more);
        setOffset(page.data.length);
      })
      .catch(() => setError("검색에 실패했습니다. 잠시 후 다시 시도해주세요."))
      .finally(() => setLoading(false));
  }, [submittedQuery]);

  const loadMore = useCallback(async () => {
    if (loadingMore || !hasMore) return;
    const q = submittedQuery.trim();
    if (!q) return;

    setLoadingMore(true);
    try {
      const page = await searchTopics(q, PAGE_SIZE, offset);
      setResults((prev) => [...prev, ...page.data]);
      setHasMore(page.has_more);
      setOffset((prev) => prev + page.data.length);
    } catch {
      setError("검색에 실패했습니다.");
    } finally {
      setLoadingMore(false);
    }
  }, [hasMore, loadingMore, offset, submittedQuery]);

  // Wire up infinite scroll once results are present.
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el || !hasMore) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) loadMore();
      },
      { rootMargin: "200px" },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [hasMore, loadMore]);

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

  return (
    <div className="min-h-screen flex flex-col bg-[#fdf9ee]">
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
            className="flex-1 h-11 px-4 rounded-full border-[2px] border-[#231815] bg-white text-[#231815] placeholder:text-[#231815]/40 focus:outline-none focus:ring-2 focus:ring-[#43b9d6]"
          />
          <button
            type="submit"
            disabled={!input.trim()}
            className="h-11 px-6 rounded-full text-sm font-semibold text-[#231815] border-[2px] border-[#231815] bg-[#43b9d6] hover:brightness-110 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
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
        ) : submittedQuery && results.length === 0 ? (
          <p className="text-center py-12 text-[#231815]/60">
            "{submittedQuery}"와 일치하는 소식이 없습니다.
          </p>
        ) : submittedQuery ? (
          <>
            <p className="text-sm text-[#231815]/60 mb-4">
              "{submittedQuery}" 검색 결과 {results.length}개
              {hasMore ? "+" : ""}
            </p>
            <ul className="space-y-3">
              {results.map((topic) => (
                <SearchResultRow
                  key={topic.id}
                  topic={topic}
                  query={submittedQuery}
                />
              ))}
            </ul>

            <div ref={sentinelRef} className="h-12 flex items-center justify-center">
              {loadingMore && <Spinner />}
              {!hasMore && results.length > 0 && (
                <span className="text-xs text-[#231815]/40">— 결과 끝 —</span>
              )}
            </div>
          </>
        ) : (
          <p className="text-center py-12 text-[#231815]/50">
            제목과 본문에서 키워드를 찾아 드려요. 한글/영문 모두 지원합니다.
          </p>
        )}
      </main>

      <Footer />
    </div>
  );
}

function SearchResultRow({ topic, query }: { topic: TopicPreview; query: string }) {
  return (
    <li>
      <Link
        to={`/topic/${topic.id}`}
        className="flex gap-4 p-4 border border-[#231815] rounded-lg bg-white hover:shadow-md transition-shadow"
      >
        <div className="w-24 h-24 sm:w-28 sm:h-28 shrink-0 overflow-hidden rounded-md bg-[#f0ece0] border border-[#231815]/30">
          <img
            src={topic.image_url || defaultImage}
            alt={topic.topic}
            className="w-full h-full object-cover"
            onError={(e) => {
              if (e.currentTarget.src !== defaultImage) e.currentTarget.src = defaultImage;
            }}
          />
        </div>
        <div className="flex-1 min-w-0">
          {topic.created_at && (
            <p className="text-xs text-[#231815]/50 mb-1">
              {formatDate(topic.created_at)}
            </p>
          )}
          <h2 className="text-base font-bold text-[#231815] line-clamp-2 leading-snug">
            <Highlighted text={topic.topic} query={query} />
          </h2>
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
