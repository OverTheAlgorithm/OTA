import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { UserLevelCard } from "@/components/user-level-card";
import { Footer } from "@/components/footer";
import { Header } from "@/components/header";
import {
  fetchAllTopics,
  fetchFilterOptions,
  batchEarnStatus,
  type TopicPreview,
  type FilterOptions,
  type FilterType,
  type EarnStatusItem,
} from "@/lib/api";

const PAGE_SIZE = 12;

interface ActiveFilter {
  type: FilterType;
  value: string;
}

function CoinTag({ status }: { status: EarnStatusItem }) {
  if (status.status === "DUPLICATE") {
    return (
      <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-[#e2e8f0] text-[#94a3b8]">
        적립 완료
      </span>
    );
  }
  if (status.status === "EXPIRED") {
    return (
      <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-[#fef2f2] text-[#f87171]">
        만료됨
      </span>
    );
  }
  if (status.status === "DAILY_LIMIT") {
    return (
      <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-[#fff7ed] text-[#fb923c]">
        일일 한도
      </span>
    );
  }
  if (status.status === "PENDING" && status.coins > 0) {
    return (
      <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-[#ecfdf5] text-[#10b981]">
        +{status.coins}코인
      </span>
    );
  }
  return null;
}

function NewsCard({
  topic,
  earnStatus,
}: {
  topic: TopicPreview;
  earnStatus?: EarnStatusItem;
}) {
  return (
    <Link
      to={`/topic/${topic.id}`}
      className="group block rounded-2xl bg-white border border-[#e2e8f0] overflow-hidden hover:shadow-md hover:border-[#d4e6f5] transition-all"
    >
      {topic.image_url && (
        <div className="aspect-[16/10] overflow-hidden bg-[#f8fafc]">
          <img
            src={topic.image_url}
            alt={topic.topic}
            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
          />
        </div>
      )}
      <div className="p-4">
        <div className="flex items-center gap-2 mb-2">
          {earnStatus && <CoinTag status={earnStatus} />}
        </div>
        <h3 className="text-sm font-semibold text-[#1e3a5f] leading-snug line-clamp-2 group-hover:text-[#4a9fe5] transition-colors">
          {topic.topic}
        </h3>
        <p className="text-xs text-[#6b8db5] mt-1.5 leading-relaxed line-clamp-2">
          {topic.summary}
        </p>
      </div>
    </Link>
  );
}

export function AllNewsPage() {
  const { user } = useAuth();
  const navigate = useNavigate();

  const [filterOptions, setFilterOptions] = useState<FilterOptions>({
    categories: [],
    brain_categories: [],
  });
  const [activeFilter, setActiveFilter] = useState<ActiveFilter>({
    type: "",
    value: "",
  });
  const [topics, setTopics] = useState<TopicPreview[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [earnMap, setEarnMap] = useState<Record<string, EarnStatusItem>>({});
  const offsetRef = useRef(0);

  // Load filter options once
  useEffect(() => {
    fetchFilterOptions().then(setFilterOptions).catch(() => {});
  }, []);

  // Load topics when filter changes
  const loadTopics = useCallback(
    async (filter: ActiveFilter, append = false) => {
      if (!append) setLoading(true);
      else setLoadingMore(true);

      const offset = append ? offsetRef.current : 0;
      try {
        const { data, has_more } = await fetchAllTopics(
          filter.type,
          filter.value,
          PAGE_SIZE,
          offset,
        );
        if (append) {
          setTopics((prev) => [...prev, ...data]);
        } else {
          setTopics(data);
        }
        setHasMore(has_more);
        offsetRef.current = offset + data.length;

        // Batch earn status for logged-in users
        if (user && data.length > 0) {
          const ids = data.map((t) => t.id);
          batchEarnStatus(ids).then((statuses) => {
            setEarnMap((prev) => {
              const next = { ...prev };
              for (const s of statuses) next[s.id] = s;
              return next;
            });
          });
        }
      } catch {
        if (!append) setTopics([]);
      } finally {
        setLoading(false);
        setLoadingMore(false);
      }
    },
    [user],
  );

  useEffect(() => {
    loadTopics(activeFilter);
  }, [activeFilter, loadTopics]);

  const handleFilterChange = (type: FilterType, value: string) => {
    if (activeFilter.type === type && activeFilter.value === value) {
      setActiveFilter({ type: "", value: "" });
    } else {
      setActiveFilter({ type, value });
    }
  };

  const handleLoadMore = () => {
    if (!loadingMore && hasMore) {
      loadTopics(activeFilter, true);
    }
  };

  // Build tab list: "전체" + categories + brain_categories
  const tabs: { label: string; type: FilterType; value: string; emoji?: string }[] = [
    { label: "전체", type: "", value: "" },
    ...filterOptions.categories.map((c) => ({
      label: c.label,
      type: "category" as FilterType,
      value: c.key,
    })),
    ...filterOptions.brain_categories.map((bc) => ({
      label: bc.label,
      type: "brain_category" as FilterType,
      value: bc.key,
      emoji: bc.emoji,
    })),
  ];

  const isActiveTab = (type: FilterType, value: string) =>
    activeFilter.type === type && activeFilter.value === value;

  return (
    <div className="min-h-screen bg-[#fafcff]">
      <Header />

      <main className="max-w-[1200px] mx-auto px-4 py-6">
        {/* Level card for logged-in users */}
        <div className="mb-6">
          <UserLevelCard />
        </div>

        {/* Category tabs */}
        <div className="mb-6 -mx-4 px-4 overflow-x-auto scrollbar-hide">
          <div className="flex gap-2 pb-2 min-w-max">
            {tabs.map((tab) => {
              const active = isActiveTab(tab.type, tab.value);
              return (
                <button
                  key={`${tab.type}-${tab.value}`}
                  onClick={() => handleFilterChange(tab.type, tab.value)}
                  className={`px-4 py-2 rounded-full text-sm font-medium whitespace-nowrap transition-all ${
                    active
                      ? "bg-[#1e3a5f] text-white"
                      : "bg-white text-[#6b8db5] border border-[#e2e8f0] hover:border-[#d4e6f5] hover:bg-[#f0f7ff]"
                  }`}
                >
                  {tab.emoji && <span className="mr-1">{tab.emoji}</span>}
                  {tab.label}
                </button>
              );
            })}
          </div>
        </div>

        {/* News grid */}
        {loading ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {Array.from({ length: 6 }).map((_, i) => (
              <div
                key={i}
                className="rounded-2xl bg-white border border-[#e2e8f0] overflow-hidden animate-pulse"
              >
                <div className="aspect-[16/10] bg-[#e2e8f0]" />
                <div className="p-4 space-y-2">
                  <div className="h-4 bg-[#e2e8f0] rounded w-3/4" />
                  <div className="h-3 bg-[#e2e8f0] rounded w-full" />
                  <div className="h-3 bg-[#e2e8f0] rounded w-2/3" />
                </div>
              </div>
            ))}
          </div>
        ) : topics.length === 0 ? (
          <div className="text-center py-20">
            <p className="text-[#6b8db5] text-sm">뉴스가 없습니다.</p>
          </div>
        ) : (
          <>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              {topics.map((topic) => (
                <NewsCard
                  key={topic.id}
                  topic={topic}
                  earnStatus={earnMap[topic.id]}
                />
              ))}
            </div>

            {/* Load more */}
            {hasMore && (
              <div className="flex justify-center mt-8">
                <button
                  onClick={handleLoadMore}
                  disabled={loadingMore}
                  className="px-8 py-3 rounded-full text-sm font-semibold bg-white text-[#1e3a5f] border border-[#d4e6f5] hover:bg-[#f0f7ff] transition-all disabled:opacity-50"
                >
                  {loadingMore ? (
                    <span className="flex items-center gap-2">
                      <svg className="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                      </svg>
                      불러오는 중...
                    </span>
                  ) : (
                    "더 보기"
                  )}
                </button>
              </div>
            )}

            {!hasMore && topics.length > 0 && (
              <p className="text-center text-xs text-[#6b8db5]/60 py-6">
                모든 뉴스를 불러왔습니다
              </p>
            )}
          </>
        )}
      </main>

      <Footer />
    </div>
  );
}
