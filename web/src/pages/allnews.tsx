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

const CATEGORY_LABELS: Record<string, string> = {
  general: "종합",
  entertainment: "연예/오락",
  business: "경제/비즈니스",
  sports: "스포츠",
  technology: "IT/기술",
  science: "과학",
  health: "건강/의학",
};

function formatDate(iso: string): string {
  const d = new Date(iso);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}.${m}.${day}`;
}

interface ActiveFilter {
  type: FilterType;
  value: string;
}

function CoinTag({ status }: { status: EarnStatusItem }) {
  if (status.status === "DUPLICATE") {
    return (
      <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/50">
        획득!
      </span>
    );
  }
  if (status.status === "EXPIRED") {
    return (
      <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/50">
        획득 기간 경과
      </span>
    );
  }
  if (status.status === "DAILY_LIMIT") {
    return (
      <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/50">
        일일 한도
      </span>
    );
  }
  if (status.status === "PENDING" && status.coins > 0) {
    return (
      <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-[#43b9d6]/15 text-[#43b9d6]">
        +{status.coins}포인트
      </span>
    );
  }
  return null;
}

function NewsCard({
  topic,
  earnStatus,
  brainCategoryMap,
}: {
  topic: TopicPreview;
  earnStatus?: EarnStatusItem;
  brainCategoryMap: Record<string, { emoji: string; label: string }>;
}) {
  const categoryLabel = topic.category
    ? CATEGORY_LABELS[topic.category] ?? topic.category
    : "";
  const brainCat = topic.brain_category
    ? brainCategoryMap[topic.brain_category]
    : undefined;

  return (
    <Link to={`/topic/${topic.id}`} className="group block">
      {topic.image_url && (
        <div className="aspect-[16/10] overflow-hidden rounded-xl bg-[#f0ece0] mb-3">
          <img
            src={topic.image_url}
            alt={topic.topic}
            className="w-full h-full object-cover [image-rendering:-webkit-optimize-contrast] [will-change:transform] group-hover:scale-105 transition-transform duration-300"
          />
        </div>
      )}

      <div className="flex items-center gap-2 mb-1">
        {topic.created_at && (
          <span className="text-sm font-medium text-[#231815]">
            {formatDate(topic.created_at)}
          </span>
        )}
        {earnStatus && <CoinTag status={earnStatus} />}
      </div>

      {(categoryLabel || brainCat) && (
        <p className="text-xs text-[#231815]/50 mb-1">
          {brainCat ? `${brainCat.emoji} ${brainCat.label}` : categoryLabel}
        </p>
      )}

      <h3 className="text-sm font-bold text-[#231815] leading-snug line-clamp-2 group-hover:opacity-70 transition-opacity">
        {topic.topic}
      </h3>
      <p className="text-xs text-[#231815]/50 mt-1 leading-relaxed line-clamp-2">
        {topic.summary}
      </p>
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

  // Build brain category lookup map
  const brainCategoryMap: Record<string, { emoji: string; label: string }> = {};
  for (const bc of filterOptions.brain_categories) {
    brainCategoryMap[bc.key] = { emoji: bc.emoji, label: bc.label };
  }

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
    <div className="min-h-screen flex flex-col bg-[#fdf9ee]">
      <Header />

      <main className="flex-1">
        <div className="max-w-[900px] mx-auto px-6 py-8">
          {/* Level Card */}
          <div className="mb-8">
            <UserLevelCard />
          </div>

          {/* Back to Home */}
          <button
            onClick={() => navigate(-1)}
            className="flex items-center gap-2 mb-6 group"
          >
            <svg
              className="w-8 h-8 text-[#231815] group-hover:opacity-70 transition-opacity"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M19 12H5" />
              <path d="M12 19l-7-7 7-7" />
            </svg>
            <span className="text-2xl font-bold text-[#231815] group-hover:opacity-70 transition-opacity">
              돌아가기
            </span>
          </button>

          {/* Category tabs */}
          <div className="mb-8 -mx-6 px-6 overflow-x-auto scrollbar-hide">
            <div className="flex gap-5 pb-2 min-w-max border-b border-[#dbdade]">
              {tabs.map((tab) => {
                const active = isActiveTab(tab.type, tab.value);
                return (
                  <button
                    key={`${tab.type}-${tab.value}`}
                    onClick={() => handleFilterChange(tab.type, tab.value)}
                    className={`pb-2 text-base font-medium whitespace-nowrap transition-colors relative ${
                      active
                        ? "text-[#008fb2]"
                        : "text-[#231815]/60 hover:text-[#231815]"
                    }`}
                  >
                    {tab.emoji && <span className="mr-1">{tab.emoji}</span>}
                    {tab.label}
                    {active && (
                      <div className="absolute bottom-0 left-0 right-0 h-[3px] bg-[#008fb2]" />
                    )}
                  </button>
                );
              })}
            </div>
          </div>

          {/* News grid */}
          {loading ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-x-6 gap-y-8">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="animate-pulse">
                  <div className="aspect-[16/10] bg-[#231815]/10 rounded-xl mb-3" />
                  <div className="h-4 bg-[#231815]/10 rounded w-1/3 mb-2" />
                  <div className="h-3 bg-[#231815]/10 rounded w-1/4 mb-2" />
                  <div className="h-4 bg-[#231815]/10 rounded w-full mb-1" />
                  <div className="h-3 bg-[#231815]/10 rounded w-2/3" />
                </div>
              ))}
            </div>
          ) : topics.length === 0 ? (
            <div className="text-center py-20">
              <p className="text-[#231815]/50 text-sm">소식이 없습니다.</p>
            </div>
          ) : (
            <>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-x-6 gap-y-8">
                {topics.map((topic) => (
                  <NewsCard
                    key={topic.id}
                    topic={topic}
                    earnStatus={earnMap[topic.id]}
                    brainCategoryMap={brainCategoryMap}
                  />
                ))}
              </div>

              {/* Load more */}
              {hasMore && (
                <div className="flex justify-center mt-10">
                  <button
                    onClick={handleLoadMore}
                    disabled={loadingMore}
                    className="px-8 py-3 rounded-full text-sm font-semibold text-[#231815] border-[2px] border-[#231815] bg-white hover:bg-[#231815]/5 transition-colors disabled:opacity-50"
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
                <p className="text-center text-xs text-[#231815]/40 py-8">
                  모든 소식을 불러왔습니다
                </p>
              )}
            </>
          )}
        </div>
      </main>

      <Footer />
    </div>
  );
}
