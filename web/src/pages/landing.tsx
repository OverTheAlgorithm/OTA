import { useState, useRef, useEffect, type ReactNode } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { LOGIN_REDIRECT_KEY } from "@/components/kakao-login-button";
import { LoginModal } from "@/components/login-modal";
import { Footer } from "@/components/footer";
import { useAuth } from "@/contexts/auth-context";
import { fetchRecentTopics, defaultImage, type TopicPreview } from "@/lib/api";

function useInView(threshold = 0.15) {
  const ref = useRef<HTMLDivElement>(null);
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setIsVisible(true);
          observer.unobserve(el);
        }
      },
      { threshold },
    );

    observer.observe(el);
    return () => observer.disconnect();
  }, [threshold]);

  return { ref, isVisible };
}

function FadeIn({
  children,
  className = "",
  delay = 0,
}: {
  children: ReactNode;
  className?: string;
  delay?: number;
}) {
  const { ref, isVisible } = useInView();

  return (
    <div
      ref={ref}
      className={`transition-all duration-700 ease-out ${
        isVisible ? "opacity-100 translate-y-0" : "opacity-0 translate-y-8"
      } ${className}`}
      style={{ transitionDelay: `${delay}ms` }}
    >
      {children}
    </div>
  );
}

const steps = [
  { icon: "/wl-step-1.svg", label: "위즈레터 무료로 가입하기" },
  { icon: "/wl-step-2.svg", label: "매일 아침 소식 보고\n포인트 쌓기" },
  { icon: "/wl-step-3.svg", label: "포인트를 모아서\n레벨업 하기" },
];

export function LandingPage() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const loginError = searchParams.get("error");

  // Redirect to the page where login was initiated (if any)
  useEffect(() => {
    if (!user) return;
    const redirectPath = localStorage.getItem(LOGIN_REDIRECT_KEY);
    localStorage.removeItem(LOGIN_REDIRECT_KEY);
    navigate(redirectPath || "/latest", { replace: true });
  }, [user, navigate]);

  const [recentTopics, setRecentTopics] = useState<TopicPreview[]>([]);

  useEffect(() => {
    fetchRecentTopics()
      .then(setRecentTopics)
      .catch(() => setRecentTopics([]))
      .finally(() => {
        const saved = sessionStorage.getItem("landing_scroll");
        if (saved) {
          requestAnimationFrame(() => window.scrollTo(0, Number(saved)));
        }
      });
  }, []);

  const [scrolled, setScrolled] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [loginOpen, setLoginOpen] = useState(false);

  // Save scroll position (throttled) + track header scroll state
  useEffect(() => {
    let ticking = false;
    const onScroll = () => {
      setScrolled(window.scrollY > 20);
      if (ticking) return;
      ticking = true;
      requestAnimationFrame(() => {
        sessionStorage.setItem("landing_scroll", String(window.scrollY));
        ticking = false;
      });
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  useEffect(() => {
    if (loginError) setLoginOpen(true);
  }, [loginError]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setLoginOpen(false);
        if (loginError) navigate("/", { replace: true });
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [loginError, navigate]);

  const handleCloseLogin = () => {
    setLoginOpen(false);
    if (loginError) navigate("/", { replace: true });
  };

  const handleStart = () => {
    if (user) {
      navigate("/mypage");
    } else {
      setLoginOpen(true);
    }
  };

  return (
    <div className="min-h-screen bg-[#fdf9ee] text-[#231815]">
      {/* ── Header ── */}
      <nav
        className={`fixed top-0 left-0 right-0 z-50 transition-all duration-300 ${
          scrolled ? "backdrop-blur-md" : ""
        }`}
      >
        <div className="bg-[#fdf9ee] border-b-[3px] border-[#231815]">
          <div className="max-w-[1200px] mx-auto px-6 h-[65px] flex items-center justify-between">
            <a href="#top" className="flex items-center">
              <img
                src="/wl-logo.png"
                alt="WizLetter"
                className="w-[140px] md:w-[200px] object-contain"
              />
            </a>

            <div className="hidden md:flex items-center gap-8">
              <a
                href="#intro"
                className="text-base font-medium text-[#231815] hover:opacity-70 transition-opacity"
              >
                서비스 소개
              </a>
              <a
                href="#news"
                className="text-base font-medium text-[#231815] hover:opacity-70 transition-opacity"
              >
                소식보기
              </a>
              <a
                href="#howto"
                className="text-base font-medium text-[#231815] hover:opacity-70 transition-opacity"
              >
                이용 방법
              </a>
              <a
                href="/allnews"
                className="text-base font-medium text-[#231815] hover:opacity-70 transition-opacity"
              >
                모든 소식 보기
              </a>
            </div>

            <div className="hidden md:flex items-center gap-3">
              {user ? (
                <>
                  <button
                    onClick={async () => { await logout(); navigate("/", { replace: true }); }}
                    className="text-base font-medium text-[#231815] hover:opacity-70 transition-opacity"
                  >
                    로그아웃
                  </button>
                  <button
                    onClick={() => navigate("/mypage")}
                    className="px-5 h-9 rounded-full text-sm font-medium bg-[#43b9d6] text-[#231815] border-[2px] border-[#231815] hover:opacity-80 transition-opacity"
                  >
                    마이페이지
                  </button>
                </>
              ) : (
                <button
                  onClick={() => setLoginOpen(true)}
                  className="px-6 py-2 rounded-full text-sm font-medium bg-[#43b9d6] text-[#231815] border border-[#231815] hover:brightness-110 transition-all"
                >
                  무료로 구독하기
                </button>
              )}
            </div>

            <button
              className="md:hidden text-[#231815] p-2"
              onClick={() => setMenuOpen(!menuOpen)}
            >
              <svg
                width="24"
                height="24"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                {menuOpen ? (
                  <path d="M18 6L6 18M6 6l12 12" />
                ) : (
                  <path d="M3 12h18M3 6h18M3 18h18" />
                )}
              </svg>
            </button>
          </div>
        </div>

        {menuOpen && (
          <div className="md:hidden bg-[#fdf9ee] border-b-[3px] border-[#231815] px-6 py-4 flex flex-col gap-4">
            <a
              href="#intro"
              className="text-base font-medium text-[#231815]"
              onClick={() => setMenuOpen(false)}
            >
              서비스 소개
            </a>
            <a
              href="#news"
              className="text-base font-medium text-[#231815]"
              onClick={() => setMenuOpen(false)}
            >
              소식보기
            </a>
            <a
              href="#howto"
              className="text-base font-medium text-[#231815]"
              onClick={() => setMenuOpen(false)}
            >
              이용 방법
            </a>
            <a
              href="/allnews"
              className="text-base font-medium text-[#231815]"
              onClick={() => setMenuOpen(false)}
            >
              모든 소식 보기
            </a>
            {user ? (
              <>
                <button
                  className="text-base font-medium text-[#231815]"
                  onClick={async () => { setMenuOpen(false); await logout(); navigate("/", { replace: true }); }}
                >
                  로그아웃
                </button>
                <button
                  className="px-6 py-2 rounded-full text-sm font-medium text-center bg-[#43b9d6] text-[#231815] border border-[#231815]"
                  onClick={() => { setMenuOpen(false); navigate("/mypage"); }}
                >
                  마이페이지
                </button>
              </>
            ) : (
              <button
                className="px-6 py-2 rounded-full text-sm font-medium text-center bg-[#43b9d6] text-[#231815] border border-[#231815]"
                onClick={() => { setMenuOpen(false); setLoginOpen(true); }}
              >
                무료로 구독하기
              </button>
            )}
          </div>
        )}
      </nav>

      <div className="max-w-[1400px] mx-auto">
      {/* ── Hero ── */}
      <section
        id="top"
        className="pt-24 md:pt-16 bg-[#fdf9ee] overflow-hidden"
      >
        <div className="max-w-[1200px] mx-auto px-6 min-h-[calc(100vh-65px)] flex flex-col justify-center py-12">
          <FadeIn>
            <h1 className="text-4xl md:text-5xl lg:text-[74px] font-semibold leading-tight lg:leading-[90px] tracking-tight">
              개인화에 갇힌 알고리즘 너머
              <br />
              진짜 세상을 읽고 수익까지
            </h1>
          </FadeIn>

          <div className="mt-6 md:mt-8 flex flex-col md:flex-row items-center gap-6 md:gap-8">
            <div className="flex-1 flex flex-col justify-center max-w-[500px]">
              <FadeIn delay={100}>
                <p className="text-lg md:text-xl lg:text-[22px] font-semibold leading-relaxed tracking-wide text-[#231815]/80 max-w-[600px]">
                  같은 뉴스만 반복하는 알고리즘 대신,
                  <br />
                  오늘 무조건 알아야 할 소식만 간결하게.
                  <br /><br />
                  세상이 돌아가는 이야기를 빠르게 파악하세요.
                  <br /><br />
                  위즈레터를 읽으면 용돈이 차곡차곡,
                  <br />
                  좋은 습관이 작은 수익으로 돌아옵니다.
                </p>
              </FadeIn>

              <FadeIn delay={200}>
                <button
                  onClick={handleStart}
                  className="mt-10 inline-flex items-center justify-center px-14 py-5 rounded-full text-xl md:text-2xl font-semibold bg-[#43b9d6] text-[#231815] border-[2.5px] border-[#231815] hover:brightness-110 transition-all w-fit"
                >
                  무료로 구독하기
                </button>
              </FadeIn>
            </div>

            <FadeIn delay={300} className="flex-shrink-0">
              <img
                src="/wl-hero.png"
                alt="위즈레터 히어로"
                className="w-[204px] md:w-[272px] lg:w-[340px] object-contain"
              />
            </FadeIn>
          </div>
        </div>
      </section>

      {/* ── 뉴스레터 특징 (비둘기) ── */}
      <section
        id="intro"
        className="border-t-[3px] border-[#231815] overflow-hidden scroll-mt-[68px]"
      >
        <div className="max-w-[1200px] mx-auto px-6 py-20 md:py-28 flex flex-col-reverse md:flex-row items-center gap-8 md:gap-16">
          <FadeIn className="flex-shrink-0">
            <img
              src="/wl-bird.png"
              alt="비둘기 일러스트"
              className="w-[220px] md:w-[320px] lg:w-[380px] object-contain"
            />
          </FadeIn>

          <div className="flex-1">
            <FadeIn>
              <h2 className="text-4xl md:text-5xl lg:text-[70px] font-bold leading-tight lg:leading-[90px]">
                5분 안에 읽는
                <br />
                간결한 뉴스레터
              </h2>
            </FadeIn>
            <FadeIn delay={100}>
              <p className="mt-6 text-xl md:text-2xl font-semibold leading-relaxed tracking-wide text-[#231815]/80">
                위즈레터를 읽으면 세상의 흐름을 파악할 수 있어요!
                <br />
                바쁜 아침에도 최신 소식을 놓치지 말아요!
              </p>
            </FadeIn>
          </div>
        </div>
      </section>

      {/* ── 포인트 보상 (사람) ── */}
      <section className="border-t-[3px] border-b-[3px] border-[#231815] bg-[#fdf9ee] overflow-hidden">
        <div className="max-w-[1200px] mx-auto px-6 py-20 md:py-28 flex flex-col md:flex-row items-center gap-8 md:gap-16">
          <div className="flex-1">
            <FadeIn>
              <h2 className="text-4xl md:text-5xl lg:text-[70px] font-bold leading-tight lg:leading-[90px]">
                지식과 포인트를
                <br />
                동시에 쌓을 수 있어요
              </h2>
            </FadeIn>
            <FadeIn delay={100}>
              <p className="mt-6 text-xl md:text-2xl font-semibold leading-relaxed tracking-wide text-[#231815]/80">
                소식을 읽을 때마다 포인트가 적립됩니다.
                <br />
                포인트가 모이면 레벨이 올라 더 많은 포인트를 모을 수 있어요!
              </p>
            </FadeIn>
          </div>

          <FadeIn delay={200} className="flex-shrink-0">
            <img
              src="/wl-person.png"
              alt="사람 일러스트"
              className="w-[220px] md:w-[300px] lg:w-[350px] object-contain"
            />
          </FadeIn>
        </div>
      </section>

      {/* ── 최신 뉴스 ── */}
      <section id="news" className="overflow-hidden scroll-mt-[68px]">
        {recentTopics.length > 0 && (
          <div className="max-w-[1200px] mx-auto px-6 pt-16 pb-8">
            <FadeIn>
              <h2 className="text-4xl md:text-5xl lg:text-[64px] font-semibold text-center tracking-widest">
                최신 소식 바로 확인하기
              </h2>
            </FadeIn>
          </div>
        )}

        {recentTopics.length > 0 && (
          <div className="border-t-[3px] border-[#231815]">
            {recentTopics.map((news, i) => (
              <FadeIn key={news.id} delay={i * 100}>
                <div className="border-b-[3px] border-[#231815] flex flex-col md:flex-row">
                  <div className="md:w-[42%] aspect-[16/9] md:aspect-auto overflow-hidden bg-[#f0ece0] flex items-center justify-center">
                    <img
                      src={news.image_url || defaultImage}
                      alt={news.topic}
                      className="w-full h-full object-contain [image-rendering:-webkit-optimize-contrast] [will-change:transform]"
                      onError={(e) => {
                        if (e.currentTarget.src !== defaultImage) e.currentTarget.src = defaultImage;
                      }}
                    />
                  </div>
                  <div
                    className={`flex-1 ${news.image_url ? "border-t-[3px] md:border-t-0 md:border-l-[3px]" : ""} border-[#231815] p-6 md:p-8 lg:p-10 flex flex-col justify-between relative`}
                  >
                    <div className="absolute top-6 left-5 w-2 h-2 rounded-sm bg-[#5bc2d9]" />
                    <div className="pl-4">
                      <h3 className="text-2xl md:text-3xl lg:text-[40px] font-normal leading-snug lg:leading-[50px] tracking-tight">
                        {news.topic}
                      </h3>
                      <p className="mt-4 text-base md:text-lg font-semibold leading-relaxed tracking-wider text-[#231815]/80">
                        {news.summary}
                      </p>
                    </div>
                    <div className="mt-6 flex justify-end">
                      <button
                        onClick={() => navigate(`/topic/${news.id}`)}
                        className="px-6 py-2 rounded-full text-sm font-semibold bg-[#43b9d6] text-[#231815] border border-[#231815] hover:brightness-110 transition-all"
                      >
                        자세히 보기
                      </button>
                    </div>
                  </div>
                </div>
              </FadeIn>
            ))}
          </div>
        )}

        <div className="flex justify-center py-10">
          <FadeIn>
            <button
              onClick={() => navigate("/allnews")}
              className="px-14 py-5 rounded-full text-xl md:text-2xl font-bold bg-[#43b9d6] text-[#231815] border-[2.5px] border-[#231815] hover:brightness-110 transition-all"
            >
              더 많은 소식 보기
            </button>
          </FadeIn>
        </div>
      </section>

      {/* ── 이용 방법 ── */}
      <section
        id="howto"
        className="border-t-[3px] border-b-[3px] border-[#231815] bg-[#fdf9ee] overflow-hidden scroll-mt-[68px]"
      >
        <div className="max-w-[1200px] mx-auto px-6 py-16 md:py-20">
          <FadeIn>
            <h2 className="text-4xl md:text-5xl lg:text-[64px] font-semibold text-center mb-16 md:mb-20">
              위즈레터 이용 방법
            </h2>
          </FadeIn>

          <div className="relative flex flex-col md:flex-row items-center md:items-start justify-between gap-10 md:gap-0">
            {/* step line (desktop only) */}
            <div className="hidden md:block absolute top-[48px] left-0 right-0 h-[2px] bg-[#bdbdbd]" />

            {steps.map((step, i) => (
              <FadeIn
                key={i}
                delay={i * 150}
                className="flex-1 flex flex-col items-center gap-3 md:gap-6 relative"
              >
                <img
                  src={step.icon}
                  alt={`Step ${i + 1}`}
                  className="w-20 h-20 md:w-24 md:h-24"
                />
                <div className="hidden md:block w-5 h-5 rounded-lg bg-[#e36901] relative z-10" />
                <p className="text-xl md:text-2xl lg:text-[28px] font-semibold text-center leading-snug whitespace-pre-line">
                  {step.label}
                </p>
              </FadeIn>
            ))}
          </div>
        </div>
      </section>

      {/* ── 마무리 CTA ── */}
      <section className="overflow-hidden">
        <div className="max-w-[1200px] mx-auto px-6 py-20 md:py-28">
          <FadeIn>
            <h2 className="text-3xl md:text-5xl lg:text-[70px] font-semibold leading-snug lg:leading-[108px] tracking-wider">
              위즈레터를 구독하고
              <br />
              슬기롭게 아침을 시작하세요.
            </h2>
          </FadeIn>
          <FadeIn delay={100}>
            <button
              onClick={handleStart}
              className="mt-10 px-14 py-5 rounded-full text-xl md:text-2xl font-semibold bg-[#43b9d6] text-[#231815] border-[2.5px] border-[#231815] hover:brightness-110 transition-all"
            >
              무료 구독하기
            </button>
          </FadeIn>
        </div>
      </section>

      </div>

      <Footer />

      <LoginModal open={loginOpen} onClose={handleCloseLogin} error={!!loginError} />
    </div>
  );
}
