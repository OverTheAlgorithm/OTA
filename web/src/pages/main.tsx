import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { Helmet } from "react-helmet-async";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { UserLevelCard } from "@/components/user-level-card";
import { CoinTag } from "@/components/coin-tag";
import { LoadingState } from "@/components/spinner";
import { LoginModal } from "@/components/login-modal";
import { useAuth } from "@/contexts/auth-context";
import {
  fetchLatestRunTopics,
  fetchAllTopics,
  listEditorPicks,
  getBrainCategories,
  batchEarnStatus,
  defaultImage,
  type TopicPreview,
  type BrainCategory,
  type EditorPickCard,
  type EarnStatusItem,
} from "@/lib/api";
import { formatDate } from "@/lib/utils";

const HERO_CAROUSEL_MAX = 7;
const EDITOR_PICK_PREVIEW = 3;
const CATEGORY_PAGE_SIZE = 6;

const CATEGORY_LABELS: Record<string, string> = {
  general: "종합",
  entertainment: "연예/오락",
  business: "경제",
  sports: "스포츠",
  technology: "IT/기술",
  science: "과학",
  health: "건강",
};

export function MainPage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const loginError = searchParams.get("error");
  const [loginOpen, setLoginOpen] = useState(false);

  // Open login modal automatically if OAuth callback delivered an error.
  useEffect(() => {
    if (loginError) setLoginOpen(true);
  }, [loginError]);

  return (
    <div className="min-h-screen flex flex-col bg-[#fdf9ee] text-[#231815]">
      <Helmet>
        <title>위즈레터 - 매일 아침 5분, 세상의 흐름을 읽다</title>
        <meta
          name="description"
          content="AI가 매일 아침 핵심 뉴스를 요약해 드립니다. 읽을수록 포인트가 쌓이고, 현금으로 교환할 수 있어요."
        />
        <link rel="canonical" href="https://wizletter.com/" />
        <meta property="og:title" content="위즈레터 - 매일 아침 5분, 세상의 흐름을 읽다" />
        <meta property="og:description" content="복잡한 뉴스를 간결하게 요약해 드립니다." />
        <meta property="og:url" content="https://wizletter.com/" />
        <meta property="og:type" content="website" />
        <meta property="og:image" content="https://wizletter.com/w_logo.png" />
        <script type="application/ld+json">
          {JSON.stringify({
            "@context": "https://schema.org",
            "@type": "WebSite",
            name: "위즈레터",
            url: "https://wizletter.com",
            description: "매일 아침 AI가 요약하는 뉴스 브리핑 서비스",
          })}
        </script>
      </Helmet>

      <Header />

      <main className="flex-1 max-w-[1200px] w-full mx-auto px-6 py-8">
        <HeroSection
          loggedIn={!!user}
          onOpenLogin={() => setLoginOpen(true)}
        />

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mt-10">
          <div className="lg:col-span-2">
            <TodaysTopNews />
          </div>
          <div>
            <EditorPicksSection />
          </div>
        </div>

        <CategoryNewsSection />

        <div className="flex justify-center mt-10">
          <button
            onClick={() => navigate("/allnews")}
            className="px-8 py-3 rounded-full text-sm font-semibold text-[#231815] border-[2px] border-[#231815] bg-[#43b9d6] hover:brightness-110 transition-all"
          >
            모든 소식 보러가기
          </button>
        </div>
      </main>

      <Footer />

      <LoginModal
        open={loginOpen}
        onClose={() => setLoginOpen(false)}
        error={!!loginError}
        redirectPath="/"
      />
    </div>
  );
}

// ─── Hero ────────────────────────────────────────────────────────────────────

function HeroSection({
  loggedIn,
  onOpenLogin,
}: {
  loggedIn: boolean;
  onOpenLogin: () => void;
}) {
  return (
    <section className="grid grid-cols-1 md:grid-cols-2 gap-6 items-start">
      <div className="space-y-5">
        <h1 className="text-3xl sm:text-4xl lg:text-5xl font-bold leading-tight tracking-tight">
          읽는 만큼 쌓이는 포인트,
          <br />
          현금으로 바꾸는 위즈레터
        </h1>
        <p className="text-base sm:text-lg text-[#231815]/75 leading-relaxed">
          같은 뉴스만 반복하는 알고리즘 대신,
          <br />
          오늘 무조건 알아야 할 소식만 간결하게.
        </p>
        <StepFlow />
      </div>

      <div className="md:pt-4">
        {loggedIn ? <UserLevelCard /> : <UnauthenticatedCTA onClick={onOpenLogin} />}
      </div>
    </section>
  );
}

function StepFlow() {
  const steps = [
    { icon: "/wl-step-1.svg", title: "뉴스 읽기", caption: "무료로 구독하고\n최신 뉴스를 읽어보세요" },
    { icon: "/wl-step-2.svg", title: "포인트 적립", caption: "뉴스를 읽은 만큼\n포인트가 쌓여요" },
    { icon: "/wl-step-3.svg", title: "현금 전환", caption: "모은 포인트는\n현금으로 바꿀 수 있어요" },
  ];

  return (
    <div className="flex items-start gap-2 sm:gap-3">
      {steps.map((step, i) => (
        <div key={step.title} className="flex items-start gap-2 sm:gap-3 flex-1">
          <div className="flex flex-col items-center flex-1 min-w-0">
            <div className="w-14 h-14 sm:w-16 sm:h-16 rounded-full border-[2px] border-[#231815]/30 bg-[#fdf9ee] flex items-center justify-center">
              <img src={step.icon} alt="" className="w-8 h-8 sm:w-9 sm:h-9" />
            </div>
            <p className="mt-2 text-sm font-bold text-[#231815] text-center">
              {step.title}
            </p>
            <p className="text-[11px] text-[#231815]/60 text-center whitespace-pre-line leading-tight mt-0.5">
              {step.caption}
            </p>
          </div>
          {i < steps.length - 1 && (
            <span className="mt-5 sm:mt-6 text-[#231815]/40 text-xl shrink-0">
              ›
            </span>
          )}
        </div>
      ))}
    </div>
  );
}

function UnauthenticatedCTA({ onClick }: { onClick: () => void }) {
  return (
    <div className="rounded-2xl border-[2px] border-[#231815] bg-white p-6 sm:p-7 flex flex-col items-center text-center">
      <h2 className="text-lg sm:text-xl font-bold mb-2">
        위즈레터를 구독하고
        <br />더 많은 혜택을 누려보세요!
      </h2>
      <p className="text-sm text-[#231815]/70 leading-relaxed mb-5">
        매일 아침 꼭 알아야 하는 필독 뉴스를 전달해 드립니다.
        위즈레터를 읽으면 용돈이 차곡차곡, 좋은 습관이 작은 수익으로 돌아옵니다.
      </p>
      <button
        onClick={onClick}
        className="px-8 py-3 rounded-full text-sm sm:text-base font-semibold text-[#231815] border-[2px] border-[#231815] bg-[#43b9d6] hover:brightness-110 transition-all"
      >
        무료로 구독하기
      </button>
    </div>
  );
}

// ─── Today's Top News (carousel) ─────────────────────────────────────────────

function TodaysTopNews() {
  const { user } = useAuth();
  const [topics, setTopics] = useState<TopicPreview[]>([]);
  const [earnMap, setEarnMap] = useState<Record<string, EarnStatusItem>>({});
  const [loading, setLoading] = useState(true);
  const [index, setIndex] = useState(0);

  useEffect(() => {
    setLoading(true);
    fetchLatestRunTopics()
      .then((data) => {
        const top = data.slice(0, HERO_CAROUSEL_MAX);
        setTopics(top);
        if (user && top.length > 0) {
          batchEarnStatus(top.map((t) => t.id))
            .then((statuses) => {
              const m: Record<string, EarnStatusItem> = {};
              for (const s of statuses) m[s.id] = s;
              setEarnMap(m);
            })
            .catch(() => {});
        }
      })
      .catch(() => setTopics([]))
      .finally(() => setLoading(false));
  }, [user]);

  const total = topics.length;
  const active = topics[index];

  const go = (delta: number) => {
    if (total === 0) return;
    setIndex((i) => (i + delta + total) % total);
  };

  return (
    <section>
      <h2 className="text-lg font-bold text-[#231815] mb-3 flex items-center gap-2">
        <span aria-hidden>🔥</span> 오늘의 주요 뉴스
      </h2>

      {loading ? (
        <div className="aspect-[16/10] rounded-xl border border-[#231815]/30 bg-white flex items-center justify-center">
          <LoadingState size="sm" />
        </div>
      ) : !active ? (
        <div className="aspect-[16/10] rounded-xl border border-[#231815]/30 bg-white flex items-center justify-center">
          <p className="text-sm text-[#231815]/50">표시할 소식이 없습니다.</p>
        </div>
      ) : (
        <article className="rounded-xl border-[2px] border-[#231815] bg-white overflow-hidden">
          <Link to={`/topic/${active.id}`} className="block group">
            <div className="aspect-[16/9] overflow-hidden bg-[#f0ece0]">
              <img
                src={active.image_url || defaultImage}
                alt={active.topic}
                className="w-full h-full object-cover group-hover:scale-[1.02] transition-transform duration-300"
                onError={(e) => {
                  if (e.currentTarget.src !== defaultImage) e.currentTarget.src = defaultImage;
                }}
              />
            </div>
            <div className="p-5 sm:p-6">
              <div className="flex items-center gap-2 mb-2 text-xs font-bold text-[#231815]">
                {active.created_at && <span>{formatDate(active.created_at)}</span>}
                {active.category && (
                  <span className="text-[#231815]/60">
                    {CATEGORY_LABELS[active.category] ?? active.category}
                  </span>
                )}
                {earnMap[active.id] && <CoinTag status={earnMap[active.id]} />}
              </div>
              <h3 className="text-xl sm:text-2xl font-bold leading-snug line-clamp-2 group-hover:opacity-70 transition-opacity">
                {active.topic}
              </h3>
              <p className="mt-2 text-sm text-[#231815]/70 line-clamp-2 leading-relaxed">
                {active.summary}
              </p>
            </div>
          </Link>
          <div className="flex items-center justify-between px-4 py-3 border-t border-[#231815]/20">
            <span className="text-xs text-[#231815]/60">
              {index + 1} / {total}
            </span>
            <div className="flex items-center gap-1">
              <button
                onClick={() => go(-1)}
                aria-label="이전 소식"
                className="w-9 h-9 rounded-full border border-[#231815] flex items-center justify-center text-[#231815] hover:bg-[#231815]/5"
              >
                ‹
              </button>
              <button
                onClick={() => go(1)}
                aria-label="다음 소식"
                className="w-9 h-9 rounded-full border border-[#231815] flex items-center justify-center text-[#231815] hover:bg-[#231815]/5"
              >
                ›
              </button>
            </div>
          </div>
        </article>
      )}
    </section>
  );
}

// ─── Editor Picks (sidebar) ──────────────────────────────────────────────────

function EditorPicksSection() {
  const [items, setItems] = useState<EditorPickCard[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    listEditorPicks(EDITOR_PICK_PREVIEW, 0)
      .then((page) => setItems(page.items))
      .catch(() => setItems([]))
      .finally(() => setLoading(false));
  }, []);

  return (
    <section>
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-lg font-bold text-[#231815] flex items-center gap-2">
          <span aria-hidden>📝</span> 에디터 픽
        </h2>
        <Link
          to="/editor-picks"
          className="text-xs text-[#231815]/60 hover:text-[#231815]"
        >
          더보기 ›
        </Link>
      </div>

      {loading ? (
        <div className="py-6 flex justify-center">
          <LoadingState size="sm" />
        </div>
      ) : items.length === 0 ? (
        <p className="text-sm text-[#231815]/50 py-6 text-center border border-[#231815]/15 rounded-lg bg-white">
          아직 발행된 에디터 픽이 없습니다.
        </p>
      ) : (
        <ul className="space-y-2.5">
          {items.map((card) => (
            <li key={card.id}>
              <Link
                to={`/editor-picks/${card.id}`}
                className="flex gap-3 p-3 rounded-lg bg-white border border-[#231815]/20 hover:shadow-md transition-shadow"
              >
                <div className="w-16 h-16 shrink-0 overflow-hidden rounded-md bg-[#f0ece0]">
                  <img
                    src={card.first_image_url || defaultImage}
                    alt={card.title}
                    className="w-full h-full object-cover"
                    onError={(e) => {
                      if (e.currentTarget.src !== defaultImage) e.currentTarget.src = defaultImage;
                    }}
                  />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-[11px] text-[#231815]/50">
                    {formatDate(card.published_at)}
                    {card.author_name && ` · ${card.author_name}`}
                  </p>
                  <h3 className="text-sm font-bold text-[#231815] line-clamp-2 mt-0.5">
                    {card.title}
                  </h3>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

// ─── Category News (tab nav + card list) ─────────────────────────────────────

// Short tab labels matching the Figma design — these are the news categories
// (categories table) rather than brain_categories (which carry long descriptive
// labels like "모르면 나만 모르는 이야기예요" unsuited to a tab strip).
const NEWS_CATEGORY_TABS: { key: string; emoji: string; label: string }[] = [
  { key: "all", emoji: "🏠", label: "전체" },
  { key: "general", emoji: "📰", label: "종합" },
  { key: "entertainment", emoji: "🎬", label: "연예" },
  { key: "business", emoji: "💰", label: "경제" },
  { key: "sports", emoji: "⚽", label: "스포츠" },
  { key: "technology", emoji: "💻", label: "IT/기술" },
  { key: "science", emoji: "🔬", label: "과학" },
  { key: "health", emoji: "🏥", label: "건강" },
];

function CategoryNewsSection() {
  const { user } = useAuth();
  const [brainCategories, setBrainCategories] = useState<BrainCategory[]>([]);
  const [activeKey, setActiveKey] = useState<string>("all");
  const [topics, setTopics] = useState<TopicPreview[]>([]);
  const [earnMap, setEarnMap] = useState<Record<string, EarnStatusItem>>({});
  const [loading, setLoading] = useState(true);

  // brain_categories still come from the API so we can label individual cards
  // (#밈/유머 etc.) even though the tab strip uses news categories.
  useEffect(() => {
    getBrainCategories()
      .then(setBrainCategories)
      .catch(() => setBrainCategories([]));
  }, []);

  useEffect(() => {
    setLoading(true);
    const filterType = activeKey === "all" ? "" : "category";
    const filterValue = activeKey === "all" ? "" : activeKey;
    fetchAllTopics(filterType, filterValue, CATEGORY_PAGE_SIZE, 0)
      .then((page) => {
        setTopics(page.data);
        if (user && page.data.length > 0) {
          batchEarnStatus(page.data.map((t) => t.id))
            .then((statuses) => {
              const m: Record<string, EarnStatusItem> = {};
              for (const s of statuses) m[s.id] = s;
              setEarnMap(m);
            })
            .catch(() => {});
        } else {
          setEarnMap({});
        }
      })
      .catch(() => setTopics([]))
      .finally(() => setLoading(false));
  }, [activeKey, user]);

  const brainCategoryMap = useMemo(() => {
    const m: Record<string, { emoji: string; label: string }> = {};
    for (const bc of brainCategories) m[bc.key] = { emoji: bc.emoji, label: bc.label };
    return m;
  }, [brainCategories]);

  return (
    <section className="mt-12">
      <h2 className="text-lg font-bold text-[#231815] mb-4 flex items-center gap-2">
        <span aria-hidden>🌐</span> 카테고리별 뉴스
      </h2>

      <div className="border-b border-[#231815]/20 mb-5 overflow-x-auto">
        <div className="flex items-stretch gap-1 min-w-max">
          {NEWS_CATEGORY_TABS.map((tab) => {
            const isActive = activeKey === tab.key;
            return (
              <button
                key={tab.key}
                onClick={() => setActiveKey(tab.key)}
                className={`flex flex-col items-center justify-end px-3 py-2 min-w-[68px] text-xs transition-colors ${
                  isActive
                    ? "text-[#231815] border-b-[3px] border-[#43b9d6] -mb-px"
                    : "text-[#231815]/60 hover:text-[#231815]"
                }`}
              >
                <span className="text-lg leading-none" aria-hidden>{tab.emoji}</span>
                <span className={`mt-1 ${isActive ? "font-bold" : "font-medium"}`}>
                  {tab.label}
                </span>
              </button>
            );
          })}
        </div>
      </div>

      {loading ? (
        <div className="py-12 flex justify-center">
          <LoadingState size="sm" />
        </div>
      ) : topics.length === 0 ? (
        <p className="text-center py-10 text-sm text-[#231815]/50">
          이 카테고리에는 아직 소식이 없습니다.
        </p>
      ) : (
        <ul className="space-y-3">
          {topics.map((topic) => (
            <CategoryCard
              key={topic.id}
              topic={topic}
              earnStatus={earnMap[topic.id]}
              brainCategoryMap={brainCategoryMap}
            />
          ))}
        </ul>
      )}
    </section>
  );
}

function CategoryCard({
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
          <div className="flex items-center gap-2 text-[11px] text-[#231815]/50 mb-1">
            {topic.created_at && <span>{formatDate(topic.created_at)}</span>}
            {categoryLabel && <span>{categoryLabel}</span>}
          </div>
          <h3 className="text-sm sm:text-base font-bold text-[#231815] leading-snug line-clamp-2">
            {topic.topic}
          </h3>
          {topic.summary && (
            <p className="mt-1 text-xs sm:text-sm text-[#231815]/70 line-clamp-2 leading-relaxed">
              {topic.summary}
            </p>
          )}
          <div className="mt-2 flex items-center gap-1.5 flex-wrap">
            {categoryLabel && <Tag>#{categoryLabel}</Tag>}
            {brainCat && (
              <Tag>
                {brainCat.emoji} #{brainCat.label}
              </Tag>
            )}
            {earnStatus && <CoinTag status={earnStatus} />}
          </div>
        </div>
      </Link>
    </li>
  );
}

function Tag({ children }: { children: React.ReactNode }) {
  return (
    <span className="text-[11px] text-[#231815]/50">{children}</span>
  );
}
