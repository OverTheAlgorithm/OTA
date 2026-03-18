import { useEffect, useRef, useState, useCallback } from "react";
import { useParams, Link } from "react-router-dom";
import { Turnstile } from "@marsidev/react-turnstile";
import {
  fetchTopicDetail,
  earnCoin,
  initEarn,
  type TopicDetail,
} from "@/lib/api";
import { useAuth } from "@/contexts/auth-context";
import { detectAdBlock } from "@/lib/adblock";

const TURNSTILE_SITE_KEY = import.meta.env.VITE_TURNSTILE_SITE_KEY || "1x00000000000000000000AA";

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString("ko-KR", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
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
}: {
  requiredSeconds: number;
  onComplete: (token: string) => void;
}) {
  const [remaining, setRemaining] = useState(requiredSeconds);
  const [isPaused, setIsPaused] = useState(false);
  const [turnstileToken, setTurnstileToken] = useState<string>("");

  const requestRef = useRef<number>(0);
  const lastTimeRef = useRef<number>(0);
  const elapsedRef = useRef<number>(0);
  const completedRef = useRef(false);

  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        setIsPaused(true);
      } else {
        setIsPaused(false);
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
            if (turnstileToken) {
              onComplete(turnstileToken);
            } else {
              console.warn("Timer completed but turnstile token is missing!");
              onComplete("");
            }
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
  }, [isPaused, remaining, turnstileToken, onComplete]);

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
  if (!state || state.kind === "loading") return null;

  let label: string;
  let color: string;
  let bgColor: string;

  switch (state.kind) {
    case "expired":
      label = "지난 기사";
      color = "var(--color-text-secondary)";
      bgColor = "var(--color-border)";
      break;
    case "not_logged_in":
      label = "로그인 필요";
      color = "var(--color-text-secondary)";
      bgColor = "var(--color-border)";
      break;
    case "adblock":
      label = "광고 차단 해제 필요";
      color = "var(--color-text-secondary)";
      bgColor = "var(--color-border)";
      break;
    case "duplicate":
      label = "획득 완료";
      color = "var(--color-text-secondary)";
      bgColor = "var(--color-border)";
      break;
    case "daily_limit":
      label = "일일 한도 도달";
      color = "var(--color-text-secondary)";
      bgColor = "var(--color-border)";
      break;
    case "countdown":
      label = state.isPaused
        ? "일시정지"
        : `${state.remaining}초`;
      color = "var(--color-button-primary)";
      bgColor = "rgba(74, 159, 229, 0.15)";
      break;
    case "success":
      label = state.leveledUp
        ? `+${state.coins} Lv.${state.newLevel}!`
        : `+${state.coins} 획득!`;
      color = "var(--color-button-primary)";
      bgColor = "rgba(74, 159, 229, 0.15)";
      break;
  }

  return (
    <span
      className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium transition-all duration-300"
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
  const [topic, setTopic] = useState<TopicDetail | null>(null);
  const [coinTag, setCoinTag] = useState<CoinTagState>(null);
  const [error, setError] = useState<"not_found" | "server_error" | null>(null);
  const [loading, setLoading] = useState(true);
  const [showCountdown, setShowCountdown] = useState<{ seconds: number; topicId: string } | null>(null);

  const handleCountdownComplete = useCallback((topicId: string, turnstileToken: string) => {
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
          // After 3 seconds, transition to "duplicate"
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
          // On error, don't show any tag — fail silently
          setCoinTag(null);
        });
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, user]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ backgroundColor: "var(--color-bg)", color: "var(--color-fg)" }}>
        <p style={{ color: "var(--color-text-secondary)" }}>불러오는 중...</p>
      </div>
    );
  }

  if (error === "not_found") {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ backgroundColor: "var(--color-bg)", color: "var(--color-fg)" }}>
        <p style={{ color: "var(--color-text-secondary)" }}>존재하지 않는 주제입니다.</p>
      </div>
    );
  }

  if (error || !topic) {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ backgroundColor: "var(--color-bg)", color: "var(--color-fg)" }}>
        <p style={{ color: "var(--color-text-secondary)" }}>불러오기에 실패했습니다. 잠시 후 다시 시도해 주세요.</p>
      </div>
    );
  }

  return (
    <div
      className="min-h-screen"
      style={{ backgroundColor: "var(--color-bg)", color: "var(--color-fg)" }}
    >
      <header
        className="sticky top-0 z-10 border-b bg-opacity-90 backdrop-blur-lg"
        style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-bg)" }}
      >
        <div className="max-w-2xl mx-auto px-6 h-16 flex items-center">
          <Link to="/">
            <img src="/OTA_logo.png" alt="OTA" className="w-[63px] h-[42px]" />
          </Link>
        </div>
      </header>

      <div className="max-w-2xl mx-auto px-6 py-8 space-y-6">
        {topic.image_url && (
          <img
            src={topic.image_url}
            alt=""
            className="w-full rounded-xl"
            style={{ maxHeight: "360px", objectFit: "cover" }}
          />
        )}
        <div>
          <p className="text-sm mb-3" style={{ color: "var(--color-text-secondary)" }}>
            {formatDate(topic.created_at)}
          </p>
          {topic.buzz_score > 0 && (
            <div className="flex items-center gap-2 mb-2 flex-wrap">
              <p className="text-sm font-bold" style={{ color: "var(--color-error)" }}>
                {"\uD83D\uDD25"} 화제도 {topic.buzz_score}
              </p>
              {showCountdown ? (
                <CountdownTag
                  requiredSeconds={showCountdown.seconds}
                  onComplete={(token) => handleCountdownComplete(showCountdown.topicId, token)}
                />
              ) : (
                <CoinTag state={coinTag} />
              )}
            </div>
          )}
          {/* Show tag even when no buzz score */}
          {(!topic.buzz_score || topic.buzz_score <= 0) && (
            <div className="flex items-center gap-2 mb-2">
              {showCountdown ? (
                <CountdownTag
                  requiredSeconds={showCountdown.seconds}
                  onComplete={(token) => handleCountdownComplete(showCountdown.topicId, token)}
                />
              ) : (
                <CoinTag state={coinTag} />
              )}
            </div>
          )}
          <h1 className="text-2xl font-bold mb-6 leading-snug" style={{ color: "var(--color-fg)" }}>
            {topic.topic}
          </h1>
          {topic.details && topic.details.length > 0 ? (
            <div className="space-y-5">
              {topic.details.map((detail, i) => {
                const title = typeof detail === "string" ? detail : detail?.title;
                const content = typeof detail === "string" ? "" : detail?.content;
                if (!title && !content) return null;
                return (
                  <div key={i} className="border-l-2 pl-4" style={{ borderColor: "var(--color-border)" }}>
                    {title && (
                      <h3 className="text-base font-semibold leading-snug mb-1.5" style={{ color: "var(--color-fg)" }}>
                        {title}
                      </h3>
                    )}
                    {content && (
                      <p className="text-sm leading-relaxed" style={{ color: "var(--color-text-secondary)" }}>
                        {content}
                      </p>
                    )}
                  </div>
                );
              })}
            </div>
          ) : topic.detail ? (
            <p className="text-base leading-relaxed" style={{ color: "var(--color-fg)" }}>
              {topic.detail}
            </p>
          ) : (
            <p className="text-sm" style={{ color: "var(--color-text-secondary)" }}>
              추가 정보가 없습니다.
            </p>
          )}
        </div>

        {topic.sources && topic.sources.length > 0 && (
          <div className="rounded-xl border px-5 py-4 space-y-2" style={{ borderColor: "var(--color-border)", background: "var(--color-card-bg)" }}>
            <p className="text-xs font-semibold uppercase tracking-wider" style={{ color: "var(--color-text-secondary)" }}>
              출처
            </p>
            <div className="flex flex-wrap gap-2">
              {topic.sources.map((src, i) => (
                <a
                  key={i}
                  href={src}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm px-3 py-1 rounded-full transition-colors"
                  style={{ color: "var(--color-text-secondary)", border: "1px solid var(--color-border)" }}
                  onMouseEnter={e => {
                    e.currentTarget.style.color = "var(--color-fg)";
                    e.currentTarget.style.borderColor = "var(--color-text-secondary)";
                  }}
                  onMouseLeave={e => {
                    e.currentTarget.style.color = "var(--color-text-secondary)";
                    e.currentTarget.style.borderColor = "var(--color-border)";
                  }}
                >
                  출처 {i + 1}
                </a>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
