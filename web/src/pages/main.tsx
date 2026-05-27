import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { Helmet } from "react-helmet-async";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { UserLevelCard } from "@/components/user-level-card";
import { CoinTag } from "@/components/coin-tag";
import { LoadingState, Spinner } from "@/components/spinner";
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
    // Mirror the carousel/editor-picks 2-1 split below so the right-hand
    // column lines up across both rows.
    <section className="grid grid-cols-1 lg:grid-cols-3 gap-6 items-start">
      <div className="lg:col-span-2 space-y-5">
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

      <div className="lg:pt-2">
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
            <span
              aria-hidden
              className="mt-4 sm:mt-5 text-[#231815]/40 text-5xl sm:text-6xl font-light leading-none shrink-0 select-none"
            >
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

// 5s between auto-advance ticks. Pauses on hover and after any manual nav
// click so the user isn't fighting the carousel.
const HERO_AUTO_ADVANCE_MS = 5000;

function TodaysTopNews() {
  const { user } = useAuth();
  const [topics, setTopics] = useState<TopicPreview[]>([]);
  const [earnMap, setEarnMap] = useState<Record<string, EarnStatusItem>>({});
  const [loading, setLoading] = useState(true);
  const [index, setIndex] = useState(0);
  const [paused, setPaused] = useState(false);
  // Holds the timeout id from the most recent manual nav click so a fast
  // second click can cancel the earlier "resume" handler instead of letting
  // it race the new pause.
  const resumeTimer = useRef<number | null>(null);

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

  // Auto-advance every 5 seconds. Re-runs whenever `paused` flips so a manual
  // click resets the timer (and pauses for one tick) instead of double-firing.
  useEffect(() => {
    if (paused || total < 2) return;
    const handle = window.setInterval(() => {
      setIndex((i) => (i + 1) % total);
    }, HERO_AUTO_ADVANCE_MS);
    return () => window.clearInterval(handle);
  }, [paused, total]);

  // Clean up any pending resume timer when the component unmounts.
  useEffect(() => {
    return () => {
      if (resumeTimer.current !== null) {
        window.clearTimeout(resumeTimer.current);
      }
    };
  }, []);

  const go = (delta: number) => {
    if (total === 0) return;
    setIndex((i) => (i + delta + total) % total);
    // Cancel any earlier resume scheduled by a previous click so timeouts
    // don't stack and prematurely un-pause auto-advance.
    if (resumeTimer.current !== null) {
      window.clearTimeout(resumeTimer.current);
    }
    setPaused(true);
    resumeTimer.current = window.setTimeout(() => {
      resumeTimer.current = null;
      setPaused(false);
    }, HERO_AUTO_ADVANCE_MS);
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
      ) : total === 0 ? (
        <div className="aspect-[16/10] rounded-xl border border-[#231815]/30 bg-white flex items-center justify-center">
          <p className="text-sm text-[#231815]/50">표시할 소식이 없습니다.</p>
        </div>
      ) : (
        <article
          className="rounded-xl border-[2px] border-[#231815] bg-white overflow-hidden flex flex-col"
          onMouseEnter={() => setPaused(true)}
          onMouseLeave={() => setPaused(false)}
        >
          {/* Slide track: all slides sit in a flex row, the track translates
              horizontally to expose the active slide. transition-transform
              handles the actual animation. */}
          <div className="relative overflow-hidden">
            <div
              className="flex transition-transform duration-500 ease-out"
              style={{ transform: `translateX(-${index * 100}%)` }}
            >
              {topics.map((topic) => (
                <CarouselSlide
                  key={topic.id}
                  topic={topic}
                  earnStatus={earnMap[topic.id]}
                />
              ))}
            </div>
          </div>
          <div className="flex items-center justify-between px-4 py-3 border-t border-[#231815]/20">
            <span className="text-xs text-[#231815]/60">
              {index + 1} / {total}
            </span>
            <div className="flex items-center gap-2">
              <button
                onClick={() => go(-1)}
                aria-label="이전 소식"
                className="w-10 h-10 rounded-full border-[2px] border-[#231815] flex items-center justify-center text-[#231815] hover:bg-[#231815]/5 transition-colors"
              >
                <ChevronIcon direction="left" />
              </button>
              <button
                onClick={() => go(1)}
                aria-label="다음 소식"
                className="w-10 h-10 rounded-full border-[2px] border-[#231815] flex items-center justify-center text-[#231815] hover:bg-[#231815]/5 transition-colors"
              >
                <ChevronIcon direction="right" />
              </button>
            </div>
          </div>
        </article>
      )}
    </section>
  );
}

function CarouselSlide({
  topic,
  earnStatus,
}: {
  topic: TopicPreview;
  earnStatus?: EarnStatusItem;
}) {
  return (
    <Link
      to={`/topic/${topic.id}`}
      // w-full + shrink-0 keep each slide exactly one frame wide so the track
      // translate-x math (`-index * 100%`) lands on slide boundaries.
      className="block group w-full shrink-0"
    >
      <div className="aspect-[16/9] overflow-hidden bg-[#f0ece0]">
        <img
          src={topic.image_url || defaultImage}
          alt={topic.topic}
          className="w-full h-full object-cover group-hover:scale-[1.02] transition-transform duration-300"
          onError={(e) => {
            if (e.currentTarget.src !== defaultImage) e.currentTarget.src = defaultImage;
          }}
        />
      </div>
      {/* Fixed-height body so swapping slides with shorter/longer text does
          not push the section below up and down. */}
      <div className="p-5 sm:p-6 h-[140px] sm:h-[156px] flex flex-col overflow-hidden">
        <div className="flex items-center gap-2 mb-2 text-xs font-bold text-[#231815] shrink-0">
          {topic.created_at && <span>{formatDate(topic.created_at)}</span>}
          {topic.category && (
            <span className="text-[#231815]/60">
              {CATEGORY_LABELS[topic.category] ?? topic.category}
            </span>
          )}
          {earnStatus && <CoinTag status={earnStatus} />}
        </div>
        <h3 className="text-lg sm:text-xl font-bold leading-snug line-clamp-2 group-hover:opacity-70 transition-opacity">
          {topic.topic}
        </h3>
        <p className="mt-1.5 text-sm text-[#231815]/70 line-clamp-2 leading-relaxed">
          {topic.summary}
        </p>
      </div>
    </Link>
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

// `type` discriminates which backend filter to apply. `"all"` clears filters,
// `"category"` targets the news category table, and `"brain_category"` targets
// the curated brain_categories. The tab strip mixes both with the news
// categories appearing first.
type TabKind = "all" | "category" | "brain_category";

interface CategoryTab {
  type: TabKind;
  key: string;
  emoji: string;
  label: string;
}

// Short tab labels matching the Figma design.
const NEWS_CATEGORY_TABS: CategoryTab[] = [
  { type: "all", key: "all", emoji: "🏠", label: "전체" },
  { type: "category", key: "general", emoji: "📰", label: "종합" },
  { type: "category", key: "entertainment", emoji: "🎬", label: "연예" },
  { type: "category", key: "business", emoji: "💰", label: "경제" },
  { type: "category", key: "sports", emoji: "⚽", label: "스포츠" },
  { type: "category", key: "technology", emoji: "💻", label: "IT/기술" },
  { type: "category", key: "science", emoji: "🔬", label: "과학" },
  { type: "category", key: "health", emoji: "🏥", label: "건강" },
];

// Shortens the long, descriptive brain_category labels (e.g. "모르면 나만
// 모르는 이야기예요") down to a tab-sized snippet. Falls back to the first
// 6 chars + "…" when no manual override is set.
const BRAIN_LABEL_OVERRIDES: Record<string, string> = {
  must_know: "필독",
  plan_ahead: "일정",
  conversation: "대화",
  opinion: "의견",
  result: "결과",
  trend: "트렌드",
  useful: "생활팁",
  fun: "유머",
  over_the_algorithm: "OTA",
};

function shortBrainLabel(bc: BrainCategory): string {
  const override = BRAIN_LABEL_OVERRIDES[bc.key];
  if (override) return override;
  if (bc.label.length <= 6) return bc.label;
  return bc.label.slice(0, 6) + "…";
}

// Time before a slow fetch surfaces *any* loading affordance. Below this we
// keep the previous tab's results on screen so a typical sub-second response
// produces no flicker at all.
const TAB_SPINNER_DELAY_MS = 3000;
// Time before we replace the content with the full skeleton state.
const TAB_FULL_LOADER_DELAY_MS = 10000;

type LoadPhase = "initial" | "idle" | "fetching" | "slow" | "verySlow";

function CategoryNewsSection() {
  const { user } = useAuth();
  const [brainCategories, setBrainCategories] = useState<BrainCategory[]>([]);
  const [active, setActive] = useState<{ type: TabKind; key: string }>({
    type: "all",
    key: "all",
  });
  const [topics, setTopics] = useState<TopicPreview[]>([]);
  const [earnMap, setEarnMap] = useState<Record<string, EarnStatusItem>>({});
  const [phase, setPhase] = useState<LoadPhase>("initial");

  useEffect(() => {
    getBrainCategories()
      .then(setBrainCategories)
      .catch(() => setBrainCategories([]));
  }, []);

  useEffect(() => {
    let cancelled = false;
    // Only escalate to a visible loading state if the fetch is *actually* slow.
    // Below the first threshold we keep the previous tab's content on screen
    // unchanged so a quick API response causes no visual disturbance.
    setPhase((prev) => (prev === "initial" ? "initial" : "fetching"));

    const slowTimer = window.setTimeout(() => {
      if (!cancelled) setPhase((prev) => (prev === "fetching" ? "slow" : prev));
    }, TAB_SPINNER_DELAY_MS);
    const verySlowTimer = window.setTimeout(() => {
      if (!cancelled) setPhase((prev) => (prev === "slow" || prev === "fetching" ? "verySlow" : prev));
    }, TAB_FULL_LOADER_DELAY_MS);

    const filterType = active.type === "all" ? "" : active.type;
    const filterValue = active.type === "all" ? "" : active.key;
    fetchAllTopics(filterType, filterValue, CATEGORY_PAGE_SIZE, 0)
      .then((page) => {
        if (cancelled) return;
        setTopics(page.data);
        setEarnMap({}); // wipe stale earn data from the previous tab
        if (user && page.data.length > 0) {
          batchEarnStatus(page.data.map((t) => t.id))
            .then((statuses) => {
              if (cancelled) return;
              const m: Record<string, EarnStatusItem> = {};
              for (const s of statuses) m[s.id] = s;
              setEarnMap(m);
            })
            .catch(() => {});
        }
      })
      .catch(() => {
        if (!cancelled) setTopics([]);
      })
      .finally(() => {
        if (!cancelled) setPhase("idle");
      });

    return () => {
      cancelled = true;
      window.clearTimeout(slowTimer);
      window.clearTimeout(verySlowTimer);
    };
  }, [active, user]);

  const brainCategoryMap = useMemo(() => {
    const m: Record<string, { emoji: string; label: string }> = {};
    for (const bc of brainCategories) m[bc.key] = { emoji: bc.emoji, label: bc.label };
    return m;
  }, [brainCategories]);

  const tabs: CategoryTab[] = useMemo(
    () => [
      ...NEWS_CATEGORY_TABS,
      ...brainCategories.map<CategoryTab>((bc) => ({
        type: "brain_category",
        key: bc.key,
        emoji: bc.emoji,
        label: shortBrainLabel(bc),
      })),
    ],
    [brainCategories],
  );

  return (
    <section className="mt-12">
      <h2 className="text-lg font-bold text-[#231815] mb-4 flex items-center gap-2">
        <span aria-hidden>🌐</span> 카테고리별 뉴스
      </h2>

      <div className="border-b border-[#231815]/20 mb-5 overflow-x-auto">
        <div className="flex items-stretch gap-1 min-w-max">
          {tabs.map((tab) => {
            const isActive = active.type === tab.type && active.key === tab.key;
            const tabId = `${tab.type}:${tab.key}`;
            return (
              <button
                key={tabId}
                onClick={() => setActive({ type: tab.type, key: tab.key })}
                title={
                  tab.type === "brain_category"
                    ? brainCategoryMap[tab.key]?.label ?? tab.label
                    : tab.label
                }
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

      {/* min-h reserves vertical space so swapping between dense and sparse
          categories does not collapse the layout while the next fetch is in
          flight. */}
      <div className="relative min-h-[600px]">
        {phase === "initial" || phase === "verySlow" ? (
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

        {/* Small inline spinner appears only once the fetch has actually
            exceeded TAB_SPINNER_DELAY_MS — sub-3s responses leave the screen
            visually untouched. */}
        {phase === "slow" && (
          <div className="absolute top-2 right-2 bg-white/80 backdrop-blur-sm rounded-full p-2 shadow-sm">
            <Spinner size="sm" className="text-[#231815]/60" />
          </div>
        )}
      </div>
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

function ChevronIcon({ direction }: { direction: "left" | "right" }) {
  // SVG keeps the icon crisp at any size and avoids the tiny default rendering
  // of `‹`/`›` characters at this button scale.
  const rotate = direction === "left" ? "rotate-180" : "";
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={rotate}
      aria-hidden
    >
      <polyline points="9 6 15 12 9 18" />
    </svg>
  );
}
