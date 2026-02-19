import { useState, useRef, useEffect, type ReactNode } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { KakaoLoginButton } from "@/components/kakao-login-button";
import { useAuth } from "@/contexts/auth-context";

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

const rotatingTexts = [
  "개인화에 갇혀버린",
  "추천에 길들여진",
  "피드에 묶여버린",
  "취향에 갇혀버린",
];

function RotatingText() {
  const [tick, setTick] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => setTick((t) => t + 1), 2500);
    return () => clearInterval(interval);
  }, []);

  const n = rotatingTexts.length;
  const cur = tick % n;
  const pre = (tick - 1 + n) % n;
  const longestText = rotatingTexts.reduce((a, b) =>
    a.length >= b.length ? a : b,
  );

  return (
    <>
      <span
        style={{
          position: "relative",
          display: "inline-block",
          overflow: "hidden",
          verticalAlign: "bottom",
          whiteSpace: "nowrap",
          textAlign: "right",
        }}
      >
        {/* 가장 긴 텍스트로 너비/높이 확보 */}
        <span style={{ visibility: "hidden" }} aria-hidden>
          {longestText}
        </span>

        {/* 나가는 카드 */}
        {tick > 0 && (
          <span
            key={`x${tick}`}
            aria-hidden
            style={{
              position: "absolute",
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              textAlign: "right",
              animation: "ota-exit 0.2s cubic-bezier(0.4,0,1,1) both",
            }}
          >
            {rotatingTexts[pre]}
          </span>
        )}

        {/* 들어오는 카드 */}
        <span
          key={`e${tick}`}
          style={{
            position: "absolute",
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            animation:
              tick === 0
                ? "none"
                : "ota-enter 0.45s cubic-bezier(0,0,0.2,1) both",
          }}
        >
          {rotatingTexts[cur]}
        </span>
      </span>
    </>
  );
}

const features = [
  {
    title: "매일 아침, 핵심 맥락 배달",
    description:
      "매일 아침 7시, 카카오톡과 이메일로 지금 가장 뜨거운 주제를 정리해 전달합니다. 바쁜 아침에도 빠르게 세상을 읽으세요.",
    color: "#f0923b",
    icon: (
      <svg
        className="w-7 h-7"
        viewBox="0 0 24 24"
        fill="none"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="12" cy="12" r="10" stroke="#f0923b" />
        <path d="M12 6v6l4 2" stroke="#f0923b" />
      </svg>
    ),
  },
  {
    title: "알고리즘 버블을 넘어서",
    description:
      "유튜브, 틱톡의 개인화 알고리즘에 갇히지 않고, 분야를 막론한 진짜 화제를 만나보세요.",
    color: "#7bc67e",
    icon: (
      <svg
        className="w-7 h-7"
        viewBox="0 0 24 24"
        fill="none"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="12" cy="12" r="10" stroke="#7bc67e" />
        <path d="M2 12h20" stroke="#7bc67e" />
        <path
          d="M12 2a15 15 0 014 10 15 15 0 01-4 10 15 15 0 01-4-10 15 15 0 014-10z"
          stroke="#7bc67e"
        />
      </svg>
    ),
  },
  {
    title: "짧고 명확한 한 줄 요약",
    description:
      "각 주제를 한 문장으로 핵심만 담아, 누구에게든 자연스럽게 대화에 참여할 수 있습니다.",
    color: "#5ba4d9",
    icon: (
      <svg
        className="w-7 h-7"
        viewBox="0 0 24 24"
        fill="none"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" stroke="#5ba4d9" />
      </svg>
    ),
  },
];

const scenarios = [
  {
    emoji: "😶",
    situation: '"어제 그 연예인 뉴스 봤어?"',
    feeling: "무슨 얘긴지 몰라서 어색하게 웃기만 했던 순간",
  },
  {
    emoji: "📱",
    situation: '"요즘 다 이거 보던데"',
    feeling: "내 피드엔 안 뜨는데, 모두가 아는 그 이야기",
  },
  {
    emoji: "🤷",
    situation: '"너 이것도 몰라?"',
    feeling: "관심 없는 분야인데, 모르면 뒤처지는 느낌",
  },
];

export function LandingPage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const loginError = searchParams.get("error");

  const [scrolled, setScrolled] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [loginOpen, setLoginOpen] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 20);
    window.addEventListener("scroll", onScroll);
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
    // 에러 파라미터가 URL에 남아 새로고침 시 모달이 재오픈되는 문제 방지
    if (loginError) navigate("/", { replace: true });
  };

  const handleStart = () => {
    if (user) {
      navigate("/home");
    } else {
      setLoginOpen(true);
    }
  };

  return (
    <div className="bg-[#0f0a19] text-[#f5f0ff] min-h-screen">
      {/* Navbar */}
      <nav
        className={`fixed top-0 left-0 right-0 z-50 transition-all duration-300 ${
          scrolled
            ? "bg-[#0f0a19]/90 backdrop-blur-lg border-b border-[#2d1f42]"
            : "bg-transparent"
        }`}
      >
        <div className="max-w-[1200px] mx-auto px-6 h-16 flex items-center justify-between">
          <a href="#top" className="flex items-center gap-3">
            <img src="/OTA_logo.png" alt="OTA" className="w-[63px] h-[42px]" />
          </a>

          <div className="hidden md:flex items-center gap-8">
            <a
              href="#features"
              className="text-sm text-[#9b8bb4] hover:text-[#f5f0ff] transition-colors"
            >
              서비스 특징
            </a>
            <a
              href="#why"
              className="text-sm text-[#9b8bb4] hover:text-[#f5f0ff] transition-colors"
            >
              왜 필요한가
            </a>
            <button
              onClick={handleStart}
              className="px-5 py-2 rounded-full text-sm font-medium bg-[#e84d3d] text-white hover:bg-[#d4382a] transition-colors"
            >
              시작하기
            </button>
          </div>

          <button
            className="md:hidden text-[#f5f0ff] p-2"
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

        {menuOpen && (
          <div className="md:hidden bg-[#0f0a19]/95 backdrop-blur-lg border-b border-[#2d1f42] px-6 py-4 flex flex-col gap-4">
            <a
              href="#features"
              className="text-sm text-[#9b8bb4] hover:text-[#f5f0ff]"
              onClick={() => setMenuOpen(false)}
            >
              서비스 특징
            </a>
            <a
              href="#why"
              className="text-sm text-[#9b8bb4] hover:text-[#f5f0ff]"
              onClick={() => setMenuOpen(false)}
            >
              왜 필요한가
            </a>
            <button
              className="px-5 py-2 rounded-full text-sm font-medium text-center bg-[#e84d3d] text-white hover:bg-[#d4382a] transition-colors"
              onClick={() => {
                setMenuOpen(false);
                handleStart();
              }}
            >
              시작하기
            </button>
          </div>
        )}
      </nav>

      {/* Hero */}
      <section id="top" className="relative pt-32 pb-24 px-6 overflow-hidden">
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] rounded-full bg-[radial-gradient(circle,rgba(232,77,61,0.06),rgba(91,164,217,0.06),transparent_70%)] pointer-events-none" />

        <div className="relative max-w-[1200px] mx-auto flex flex-col items-center text-center">
          <FadeIn>
            <img
              src="/OTA_logo.png"
              alt="Over the Algorithm"
              className="h-20 md:h-28 animate-float"
            />
          </FadeIn>

          <FadeIn delay={100} className="mt-8">
            <h1 className="text-4xl md:text-6xl lg:text-7xl font-bold leading-tight">
              <RotatingText /> 알고리즘 너머
              <br />
              <span className="font-brush text-4xl md:text-6xl lg:text-7xl bg-gradient-to-r from-[#e84d3d] via-[#f5d547] to-[#5ba4d9] bg-clip-text text-transparent">
                세상의 맥락
              </span>
              을 읽다
            </h1>
          </FadeIn>

          <FadeIn delay={200} className="mt-6">
            <p className="font-dongle text-2xl md:text-3xl text-[#9b8bb4] max-w-2xl leading-relaxed">
              매일 아침 7시, 개인화된 알고리즘에 갇히지 않은
              <br className="hidden md:block" />
              가장 뜨거운 이야기를 전해드립니다.
            </p>
          </FadeIn>

          <FadeIn delay={300} className="mt-10">
            <button
              onClick={handleStart}
              className="inline-flex items-center gap-2 px-8 py-4 rounded-full text-lg font-medium bg-[#e84d3d] text-white hover:bg-[#d4382a] transition-colors"
            >
              무료로 시작하기
              <svg className="w-5 h-5" viewBox="0 0 20 20" fill="currentColor">
                <path
                  fillRule="evenodd"
                  d="M7.21 14.77a.75.75 0 01.02-1.06L11.168 10 7.23 6.29a.75.75 0 111.04-1.08l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 01-1.06-.02z"
                  clipRule="evenodd"
                />
              </svg>
            </button>
          </FadeIn>
        </div>
      </section>

      {/* Features */}
      <section id="features" className="py-24 px-6">
        <div className="max-w-[1200px] mx-auto">
          <FadeIn>
            <h2 className="font-dongle text-5xl md:text-6xl font-bold text-center mb-4">
              서비스 특징
            </h2>
            <p className="text-[#9b8bb4] text-center mb-16 max-w-lg mx-auto">
              Over the Algorithm이 매일 아침 전하는 가치
            </p>
          </FadeIn>

          <div className="grid md:grid-cols-3 gap-8">
            {features.map((feature, i) => (
              <FadeIn key={feature.title} delay={i * 150}>
                <div className="group rounded-2xl bg-[#1a1229] border border-[#2d1f42] p-8 hover:border-[#3d2f55] transition-all duration-300 h-full">
                  <div
                    className="w-12 h-12 rounded-xl flex items-center justify-center mb-5"
                    style={{ backgroundColor: `${feature.color}15` }}
                  >
                    {feature.icon}
                  </div>
                  <h3 className="font-dongle text-3xl font-bold mb-3">
                    {feature.title}
                  </h3>
                  <p className="text-sm text-[#9b8bb4] leading-relaxed">
                    {feature.description}
                  </p>
                </div>
              </FadeIn>
            ))}
          </div>
        </div>
      </section>

      {/* Pain Point */}
      <section id="why" className="py-24 px-6 bg-[#130e20]">
        <div className="max-w-[1200px] mx-auto">
          <FadeIn>
            <h2 className="text-3xl md:text-5xl font-bold text-center mb-4">
              이런 적, 있으시죠?
            </h2>
            <p className="text-[#9b8bb4] text-center mb-16 max-w-lg mx-auto">
              알고리즘은 내 취향만 보여줍니다. 세상의 맥락은 보여주지 않죠.
            </p>
          </FadeIn>

          <div className="grid md:grid-cols-3 gap-8">
            {scenarios.map((s, i) => (
              <FadeIn key={i} delay={i * 150}>
                <div className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] p-8 hover:border-[#3d2f55] transition-all duration-300 h-full flex flex-col items-center text-center">
                  <span className="text-5xl mb-5">{s.emoji}</span>
                  <p className="text-lg font-semibold text-[#f5f0ff] mb-3">
                    {s.situation}
                  </p>
                  <p className="text-sm text-[#9b8bb4] leading-relaxed">
                    {s.feeling}
                  </p>
                </div>
              </FadeIn>
            ))}
          </div>

          <FadeIn delay={500}>
            <p className="text-center mt-12 text-lg text-[#9b8bb4]">
              관심 없는 분야까지 전부 챙길 순 없습니다.
              <br />
              하지만{" "}
              <span className="text-[#f5f0ff] font-semibold">
                핵심 맥락 한 줄
              </span>
              이면 충분합니다.
            </p>
          </FadeIn>
        </div>
      </section>

      {/* CTA */}
      <section className="py-24 px-6">
        <FadeIn>
          <div className="max-w-[800px] mx-auto text-center rounded-3xl bg-gradient-to-br from-[#1a1229] to-[#231a35] border border-[#2d1f42] p-12 md:p-16 relative overflow-hidden">
            <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(232,77,61,0.05),transparent_50%),radial-gradient(circle_at_bottom_left,rgba(91,164,217,0.05),transparent_50%)] pointer-events-none" />
            <div className="relative">
              <h2 className="text-3xl md:text-4xl font-bold mb-4">
                오늘의{" "}
                <span className="font-brush text-4xl md:text-5xl bg-gradient-to-r from-[#f0923b] to-[#e84d3d] bg-clip-text text-transparent">
                  맥락
                </span>
                , 놓치고 계신가요?
              </h2>
              <p className="font-dongle text-2xl md:text-3xl text-[#9b8bb4] mb-8 max-w-md mx-auto">
                지금 가입하고 내일 아침부터 받아보세요.
              </p>
              <button
                onClick={handleStart}
                className="inline-flex items-center gap-2 px-8 py-4 rounded-full text-lg font-medium bg-[#e84d3d] text-white hover:bg-[#d4382a] transition-colors"
              >
                지금 시작하기
                <svg
                  className="w-5 h-5"
                  viewBox="0 0 20 20"
                  fill="currentColor"
                >
                  <path
                    fillRule="evenodd"
                    d="M7.21 14.77a.75.75 0 01.02-1.06L11.168 10 7.23 6.29a.75.75 0 111.04-1.08l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 01-1.06-.02z"
                    clipRule="evenodd"
                  />
                </svg>
              </button>
            </div>
          </div>
        </FadeIn>
      </section>

      {/* Footer */}
      <footer className="border-t border-[#2d1f42] py-8 px-6">
        <div className="max-w-[1200px] mx-auto flex flex-col md:flex-row justify-between items-center gap-4">
          <img src="/OTA_logo.png" alt="OTA" className="h-6 opacity-50" />
          <p className="text-sm text-[#9b8bb4]">
            &copy; 2026 Over the Algorithm. All rights reserved.
          </p>
        </div>
      </footer>

      {/* Login Modal */}
      {loginOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm px-4"
          onClick={handleCloseLogin}
        >
          <div
            className="relative w-full max-w-sm bg-[#1a1229] border border-[#2d1f42] rounded-2xl p-8 flex flex-col items-center gap-6"
            onClick={(e) => e.stopPropagation()}
          >
            {/* 닫기 */}
            <button
              onClick={handleCloseLogin}
              className="absolute top-4 right-4 text-[#9b8bb4] hover:text-[#f5f0ff] transition-colors"
            >
              <svg
                width="20"
                height="20"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <path d="M18 6L6 18M6 6l12 12" />
              </svg>
            </button>

            <img src="/OTA_logo.png" alt="OTA" className="h-10" />

            <div className="text-center">
              <h2 className="text-xl font-bold text-[#f5f0ff]">시작하기</h2>
              <p className="mt-1 text-sm text-[#9b8bb4]">
                알고리즘을 넘어, 지금 가장 뜨거운 맥락을 만나보세요
              </p>
            </div>

            {loginError && (
              <p className="text-sm text-[#e84d3d] text-center">
                로그인에 실패했습니다. 다시 시도해주세요.
              </p>
            )}

            <KakaoLoginButton />
          </div>
        </div>
      )}
    </div>
  );
}
