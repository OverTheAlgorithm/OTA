import { useEffect, useRef, useState, useCallback } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { Turnstile } from "@marsidev/react-turnstile";
import {
  fetchTopicDetail,
  earnCoin,
  initEarn,
  getBrainCategories,
  type TopicDetail,
  type BrainCategory,
} from "@/lib/api";
import { useAuth } from "@/contexts/auth-context";
import { detectAdBlock } from "@/lib/adblock";
import { UserLevelCard } from "@/components/user-level-card";
import { KakaoLoginButton } from "@/components/kakao-login-button";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";

const TURNSTILE_SITE_KEY = import.meta.env.VITE_TURNSTILE_SITE_KEY || "1x00000000000000000000AA";

const CATEGORY_LABELS: Record<string, string> = {
  general: "종합",
  entertainment: "연예",
  business: "경제",
  sports: "스포츠",
  technology: "IT",
  science: "과학",
  health: "건강",
};

function formatDate(iso: string): string {
  const d = new Date(iso);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}.${m}.${day}`;
}

// ── Coin tag states (priority order) ─────────────────────────────────────────

type CoinTagState =
  | { kind: "expired" }
  | { kind: "not_logged_in" }
  | { kind: "adblock" }
  | { kind: "duplicate" }
  | { kind: "daily_limit" }
  | { kind: "countdown"; remaining: number; isPaused: boolean }
  | { kind: "success"; coins: number; leveledUp: boolean; newLevel: number }
  | { kind: "loading" }
  | null;

// ── Inline Countdown Tag ─────────────────────────────────────────────────────

function CountdownTag({
  requiredSeconds,
  onComplete,
  externalPaused = false,
}: {
  requiredSeconds: number;
  onComplete: (token: string) => void;
  externalPaused?: boolean;
}) {
  const [remaining, setRemaining] = useState(requiredSeconds);
  const [visibilityPaused, setVisibilityPaused] = useState(false);
  const isPaused = visibilityPaused || externalPaused;
  const [turnstileToken, setTurnstileToken] = useState<string>("");
  const [timerDone, setTimerDone] = useState(false);

  const requestRef = useRef<number>(0);
  const lastTimeRef = useRef<number>(0);
  const elapsedRef = useRef<number>(0);
  const completedRef = useRef(false);
  const tokenRef = useRef<string>("");

  // Keep ref in sync so rAF closure always sees latest token
  useEffect(() => {
    tokenRef.current = turnstileToken;
  }, [turnstileToken]);

  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        setVisibilityPaused(true);
      } else {
        setVisibilityPaused(false);
        lastTimeRef.current = performance.now();
      }
    };
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => document.removeEventListener("visibilitychange", handleVisibilityChange);
  }, []);

  const animate = useCallback((time: number) => {
    if (lastTimeRef.current !== undefined && !isPaused) {
      const deltaTime = time - lastTimeRef.current;
      elapsedRef.current += deltaTime;

      if (elapsedRef.current >= 1000) {
        elapsedRef.current -= 1000;
        setRemaining((prev) => {
          const next = prev - 1;
          if (next <= 0 && !completedRef.current) {
            completedRef.current = true;
            setTimerDone(true);
            // If token is already available, fire immediately
            if (tokenRef.current) {
              onComplete(tokenRef.current);
            }
            // Otherwise, the useEffect below will fire when token arrives
            return 0;
          }
          return Math.max(0, next);
        });
      }
    }
    lastTimeRef.current = time;
    if (remaining > 0) {
      requestRef.current = requestAnimationFrame(animate);
    }
  }, [isPaused, remaining, onComplete]);

  // Fire onComplete when both timer is done AND token is available
  useEffect(() => {
    if (timerDone && turnstileToken && completedRef.current) {
      onComplete(turnstileToken);
    }
  }, [timerDone, turnstileToken, onComplete]);

  useEffect(() => {
    lastTimeRef.current = performance.now();
    requestRef.current = requestAnimationFrame(animate);
    return () => {
      if (requestRef.current) cancelAnimationFrame(requestRef.current);
    };
  }, [animate]);

  return (
    <>
      {/* Invisible Turnstile widget */}
      <div style={{ position: "absolute", opacity: 0, pointerEvents: "none", width: 0, height: 0, overflow: "hidden" }}>
        <Turnstile
          siteKey={TURNSTILE_SITE_KEY}
          onSuccess={setTurnstileToken}
          options={{ theme: "auto" }}
        />
      </div>
      <CoinTag
        state={
          remaining > 0
            ? { kind: "countdown", remaining, isPaused }
            : { kind: "loading" }
        }
      />
    </>
  );
}

// ── CoinTag component ────────────────────────────────────────────────────────

function CoinTag({ state }: { state: CoinTagState }) {
  if (!state) return null;

  let label: string;
  let color: string;
  let bgColor: string;

  switch (state.kind) {
    case "loading":
      label = "대기 중...";
      color = "#43b9d6";
      bgColor = "rgba(67, 185, 214, 0.15)";
      break;
    case "expired":
      label = "지난 기사";
      color = "#231815";
      bgColor = "#e8e8e8";
      break;
    case "not_logged_in":
      label = "로그인 필요";
      color = "#231815";
      bgColor = "#e8e8e8";
      break;
    case "adblock":
      label = "광고 차단 해제 필요";
      color = "#231815";
      bgColor = "#e8e8e8";
      break;
    case "duplicate":
      label = "획득 완료";
      color = "#231815";
      bgColor = "#e8e8e8";
      break;
    case "daily_limit":
      label = "일일 한도 도달";
      color = "#231815";
      bgColor = "#e8e8e8";
      break;
    case "countdown":
      label = state.isPaused
        ? "일시정지"
        : `${state.remaining}초`;
      color = "#43b9d6";
      bgColor = "rgba(67, 185, 214, 0.15)";
      break;
    case "success":
      label = state.leveledUp
        ? `+${state.coins} Lv.${state.newLevel}!`
        : `+${state.coins} 획득!`;
      color = "#43b9d6";
      bgColor = "rgba(67, 185, 214, 0.15)";
      break;
  }

  return (
    <span
      className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-bold transition-all duration-300"
      style={{ color, backgroundColor: bgColor }}
    >
      {state.kind === "countdown" && !state.isPaused && (
        <span className="inline-block w-1.5 h-1.5 rounded-full mr-1 animate-pulse" style={{ backgroundColor: color }} />
      )}
      {label}
    </span>
  );
}

// ── Page ─────────────────────────────────────────────────────────────────────

export function TopicPage() {
  const { id } = useParams<{ id: string }>();
  const { user } = useAuth();
  const navigate = useNavigate();
  const [topic, setTopic] = useState<TopicDetail | null>(null);
  const [brainCategories, setBrainCategories] = useState<BrainCategory[]>([]);
  const [coinTag, setCoinTag] = useState<CoinTagState>(null);
  const [levelRefreshKey, setLevelRefreshKey] = useState(0);
  const [error, setError] = useState<"not_found" | "server_error" | null>(null);
  const [loading, setLoading] = useState(true);
  const [loginOpen, setLoginOpen] = useState(false);
  const [showCountdown, setShowCountdown] = useState<{ seconds: number; topicId: string } | null>(null);

  // Block navigation while coin earning is in progress
  const isEarning = showCountdown !== null || coinTag?.kind === "loading";
  const [showLeaveModal, setShowLeaveModal] = useState(false);

  const pendingNavRef = useRef<string | null>(null);

  useEffect(() => {
    if (!isEarning) return;

    // 1) Browser close/refresh/address bar → native dialog
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };

    // 2) Browser back/forward buttons → show modal
    window.history.pushState({ earning: true }, "");
    const handlePopState = () => {
      pendingNavRef.current = "__back__";
      setShowLeaveModal(true);
    };

    // 3) Internal link clicks (Header, Footer, etc.) → intercept in capture phase
    const handleLinkClick = (e: MouseEvent) => {
      const anchor = (e.target as Element).closest("a");
      if (!anchor) return;
      const href = anchor.getAttribute("href");
      // Skip external links (http, mailto, tel) and anchor-only links
      if (!href || /^(https?:|mailto:|tel:|#)/.test(href)) return;
      e.preventDefault();
      e.stopPropagation();
      pendingNavRef.current = href;
      setShowLeaveModal(true);
    };

    window.addEventListener("beforeunload", handleBeforeUnload);
    window.addEventListener("popstate", handlePopState);
    document.addEventListener("click", handleLinkClick, true);
    return () => {
      window.removeEventListener("beforeunload", handleBeforeUnload);
      window.removeEventListener("popstate", handlePopState);
      document.removeEventListener("click", handleLinkClick, true);
    };
  }, [isEarning]);

  const earnCalledRef = useRef(false);
  const handleCountdownComplete = useCallback((topicId: string, turnstileToken: string) => {
    // Guard against double-fire (rAF + useEffect race)
    if (earnCalledRef.current) return;
    earnCalledRef.current = true;

    setShowCountdown(null);
    setCoinTag({ kind: "loading" });

    earnCoin(topicId, turnstileToken)
      .then((result) => {
        if (result.earned) {
          setCoinTag({
            kind: "success",
            coins: result.coins_earned,
            leveledUp: result.leveled_up,
            newLevel: result.new_level,
          });
          setLevelRefreshKey((k) => k + 1);
          setTimeout(() => setCoinTag({ kind: "duplicate" }), 3000);
        } else {
          setCoinTag(
            result.reason === "EXPIRED"
              ? { kind: "expired" }
              : { kind: "duplicate" }
          );
        }
      })
      .catch(() => {
        setCoinTag({ kind: "duplicate" });
      });
  }, []);

  useEffect(() => {
    if (!id) return;

    fetchTopicDetail(id)
      .then(setTopic)
      .catch((e: Error) => {
        setError(e.message === "not_found" ? "not_found" : "server_error");
      })
      .finally(() => setLoading(false));

    getBrainCategories().then(setBrainCategories).catch(() => {});

    if (!user) {
      setCoinTag({ kind: "not_logged_in" });
      return;
    }

    detectAdBlock().then((blocked) => {
      if (blocked) {
        setCoinTag({ kind: "adblock" });
        return;
      }

      initEarn(id)
        .then((result) => {
          switch (result.status) {
            case "PENDING":
              setShowCountdown({ seconds: result.required_seconds ?? 10, topicId: id });
              break;
            case "EXPIRED":
              setCoinTag({ kind: "expired" });
              break;
            case "DUPLICATE":
              setCoinTag({ kind: "duplicate" });
              break;
            case "DAILY_LIMIT":
              setCoinTag({ kind: "daily_limit" });
              break;
          }
        })
        .catch(() => {
          setCoinTag(null);
        });
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, user]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#fdf9ee]">
        <p className="text-[#231815]/60">불러오는 중...</p>
      </div>
    );
  }

  if (error === "not_found") {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#fdf9ee]">
        <p className="text-[#231815]/60">존재하지 않는 주제입니다.</p>
      </div>
    );
  }

  if (error || !topic) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#fdf9ee]">
        <p className="text-[#231815]/60">불러오기에 실패했습니다. 잠시 후 다시 시도해 주세요.</p>
      </div>
    );
  }

  const categoryLabel = topic.category
    ? CATEGORY_LABELS[topic.category] || topic.category
    : "";

  const brainCat = topic.brain_category
    ? brainCategories.find((bc) => bc.key === topic.brain_category)
    : undefined;

  const coinTagElement = !user ? null : showCountdown ? (
    <CountdownTag
      requiredSeconds={showCountdown.seconds}
      onComplete={(token) => handleCountdownComplete(showCountdown.topicId, token)}
      externalPaused={showLeaveModal}
    />
  ) : (
    <CoinTag state={coinTag} />
  );

  return (
    <div className="min-h-screen flex flex-col bg-[#fdf9ee]">
      {/* ── Header ── */}
      <Header />

      {/* ── Content ── */}
      <main className="flex-1">
        <div className="max-w-[900px] mx-auto px-6 py-8">
          {/* Level Card */}
          <div className="mb-8">
            <UserLevelCard refreshKey={levelRefreshKey} />
          </div>

          {/* Back Button */}
          <button
            onClick={() => isEarning ? setShowLeaveModal(true) : navigate(-1)}
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

          {/* Article */}
          <article>
            {/* Hero Image */}
            {topic.image_url && (
              <div className="flex justify-center mb-8">
                <img
                  src={topic.image_url}
                  alt=""
                  className="rounded-xl max-h-[400px] object-cover"
                />
              </div>
            )}

            {/* Date */}
            <p className="text-2xl font-bold text-[#231815]">
              {formatDate(topic.created_at)}
            </p>

            {/* Category bracket + tags */}
            <div className="flex items-baseline gap-3 flex-wrap mt-1 mb-1">
              {categoryLabel && (
                <span
                  className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-bold"
                  style={{ color: "#231815", backgroundColor: "#23181520" }}
                >
                  {categoryLabel}
                </span>
              )}
              {brainCat && (
                <span
                  className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-bold"
                  style={{
                    color: brainCat.accent_color,
                    backgroundColor: `${brainCat.accent_color}20`,
                  }}
                >
                  {brainCat.emoji} {brainCat.label}
                </span>
              )}
              {coinTagElement}
            </div>

            {/* Title */}
            <h1 className="text-3xl md:text-4xl font-bold text-[#231815] leading-tight mb-8">
              {topic.topic}
            </h1>

            {/* Detail Items */}
            {topic.details && topic.details.length > 0 ? (
              <div className="space-y-6 mb-10">
                {topic.details.map((detail, i) => {
                  const title = typeof detail === "string" ? detail : detail?.title;
                  const content = typeof detail === "string" ? "" : detail?.content;
                  if (!title && !content) return null;
                  return (
                    <div key={i} className="border-l-[3px] border-[#43b9d6] pl-5">
                      {title && (
                        <h3 className="text-lg font-bold text-[#231815] leading-snug mb-2">
                          {title}
                        </h3>
                      )}
                      {content && (
                        <p className="text-base leading-relaxed text-[#231815]/80">
                          {content}
                        </p>
                      )}
                    </div>
                  );
                })}
              </div>
            ) : topic.detail ? (
              <p className="text-base leading-relaxed text-[#231815] mb-10">
                {topic.detail}
              </p>
            ) : (
              <p className="text-sm text-[#231815]/60 mb-10">
                추가 정보가 없습니다.
              </p>
            )}

            {/* Sources */}
            {topic.sources && topic.sources.length > 0 && (
              <div className="rounded-2xl border border-[#231815]/20 px-6 py-5 mb-8">
                <p className="text-lg font-bold text-[#231815] mb-4">
                  출처
                </p>
                <div className="flex flex-wrap gap-3">
                  {topic.sources.map((src, i) => (
                    <a
                      key={i}
                      href={src}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-sm px-4 py-2 rounded-full bg-white border border-[#231815] text-[#231815] font-medium hover:bg-[#231815] hover:text-white transition-colors"
                    >
                      출처 {i + 1}
                    </a>
                  ))}
                </div>
              </div>
            )}
          </article>
        </div>
      </main>

      {/* ── Footer ── */}
      <Footer />

      {/* ── Leave Confirmation Modal ── */}
      {showLeaveModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm px-4">
          <div className="w-full max-w-sm bg-[#fdf9ee] border-[3px] border-[#231815] rounded-2xl p-8 flex flex-col items-center gap-5">
            <div className="w-14 h-14 rounded-full bg-[#43b9d6]/15 flex items-center justify-center">
              <svg className="w-7 h-7 text-[#43b9d6]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10" />
                <path d="M12 8v4M12 16h.01" />
              </svg>
            </div>
            <div className="text-center">
              <h2 className="text-xl font-bold text-[#231815]">포인트 획득 중이에요</h2>
              <p className="mt-2 text-sm text-[#231815]/60 leading-relaxed">
                지금 나가면 포인트를 받을 수 없어요.<br />조금만 더 기다려주세요!
              </p>
            </div>
            <div className="flex gap-3 w-full">
              <button
                onClick={() => {
                  const dest = pendingNavRef.current;
                  pendingNavRef.current = null;
                  setShowLeaveModal(false);
                  if (dest === "__back__") {
                    navigate(-1);
                  } else if (dest) {
                    navigate(dest);
                  }
                }}
                className="flex-1 py-3 rounded-xl text-sm font-semibold text-[#231815] border-[2px] border-[#231815] bg-white hover:bg-[#231815]/5 transition-colors"
              >
                나가기
              </button>
              <button
                onClick={() => {
                  pendingNavRef.current = null;
                  setShowLeaveModal(false);
                  // Re-push dummy entry so next browser-back is caught again
                  window.history.pushState({ earning: true }, "");
                }}
                className="flex-1 py-3 rounded-xl text-sm font-semibold text-[#231815] border-[2px] border-[#231815] bg-[#43b9d6] hover:brightness-110 transition-all"
              >
                머무르기
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ── Login Modal ── */}
      {loginOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm px-4"
          onClick={() => setLoginOpen(false)}
        >
          <div
            className="relative w-full max-w-sm bg-[#fdf9ee] border-[3px] border-[#231815] rounded-2xl p-8 flex flex-col items-center gap-6"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              onClick={() => setLoginOpen(false)}
              className="absolute top-4 right-4 text-[#231815]/60 hover:text-[#231815] transition-colors"
            >
              <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
                <path d="M18 6L6 18M6 6l12 12" />
              </svg>
            </button>
            <img src="/wl-logo.png" alt="WizLetter" className="w-[140px] md:w-[200px] object-contain" />
            <div className="text-center">
              <h2 className="text-xl font-bold text-[#231815]">로그인</h2>
              <p className="mt-1 text-sm text-[#231815]/60">매일 아침 5분, 세상의 흐름을 읽는 위즈레터</p>
            </div>
            <KakaoLoginButton redirectPath={`/topic/${id}`} />
          </div>
        </div>
      )}
    </div>
  );
}
