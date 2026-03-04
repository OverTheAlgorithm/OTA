import { useEffect, useRef, useState } from "react";
import { useParams, useSearchParams, Link } from "react-router-dom";
import { Turnstile } from "@marsidev/react-turnstile";
import {
  fetchTopicDetail,
  earnCoinFromEmail,
  initEarn,
  type TopicDetail,
  type TopicEarnResult,
} from "@/lib/api";

const TURNSTILE_SITE_KEY = import.meta.env.VITE_TURNSTILE_SITE_KEY || "1x00000000000000000000AA"; // Dummy testing key if env not configured

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString("ko-KR", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

// ── Toast types ───────────────────────────────────────────────────────────────

type ToastState =
  | { kind: "countdown"; requiredSeconds: number; topicId: string }
  | { kind: "success"; earn: TopicEarnResult }
  | { kind: "info"; message: string }
  | { kind: "error"; message: string }
  | null;

// ── Countdown Toast (매초 pop 애니메이션 및 활성 탭 검증) ─────────────────────────

function CountdownToast({
  requiredSeconds,
  onComplete,
}: {
  requiredSeconds: number;
  onComplete: (token: string) => void;
}) {
  const [remaining, setRemaining] = useState(requiredSeconds);
  const [pop, setPop] = useState(false);
  const [isPaused, setIsPaused] = useState(false);
  const [turnstileToken, setTurnstileToken] = useState<string>("");
  
  const requestRef = useRef<number>(0);
  const lastTimeRef = useRef<number>(0);
  const elapsedRef = useRef<number>(0);

  useEffect(() => {
    // 탭 숨김 처리 핸들러
    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        setIsPaused(true);
      } else {
        setIsPaused(false);
        // 복귀 시 시간 기준점 갱신 (오프라인 동안 시간이 흐르지 않게 함)
        lastTimeRef.current = performance.now();
      }
    };
    
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => document.removeEventListener("visibilitychange", handleVisibilityChange);
  }, []);

  const animate = (time: number) => {
    if (lastTimeRef.current !== undefined && !isPaused) {
      const deltaTime = time - lastTimeRef.current;
      elapsedRef.current += deltaTime;

      if (elapsedRef.current >= 1000) {
        elapsedRef.current -= 1000;
        setRemaining((prev) => {
          const next = prev - 1;
          if (next <= 0) {
            if (turnstileToken) {
              onComplete(turnstileToken);
            } else {
              // Should theoretically not happen unless extremely fast
              console.warn("Timer completed but turnstile token is missing!");
              onComplete(""); 
            }
            return 0;
          }
          return next;
        });
        
        // 초가 바뀔 때 애니메이션 트리거
        setPop(true);
        setTimeout(() => setPop(false), 280);
      }
    }
    
    // 계속해서 타이머를 잼 (백그라운드일 때는 requestAnimationFrame이 거의 호출되지 않거나 멈춤)
    lastTimeRef.current = time;
    if (remaining > 0) {
      requestRef.current = requestAnimationFrame(animate);
    }
  };

  useEffect(() => {
    lastTimeRef.current = performance.now();
    requestRef.current = requestAnimationFrame(animate);
    return () => {
      if (requestRef.current) cancelAnimationFrame(requestRef.current);
    };
  }, [remaining, isPaused, turnstileToken]); // 의존성에 remaining을 넣으면 onComplete가 제대로 제어됩니다

  if (remaining <= 0) return null;

  return (
    <div
      style={{
        position: "fixed",
        bottom: "28px",
        left: "50%",
        transform: `translateX(-50%) scale(${pop ? 1.12 : 1})`,
        transition: "transform 0.25s cubic-bezier(0.34, 1.56, 0.64, 1)",
        zIndex: 50,
        background: isPaused ? "var(--color-text-secondary)" : "var(--color-card-bg)",
        color: isPaused ? "var(--color-bg)" : "var(--color-button-primary)",
        border: "1px solid var(--color-button-primary)",
        borderRadius: "12px",
        padding: "10px 22px",
        fontSize: "14px",
        fontWeight: 600,
        whiteSpace: "nowrap",
        boxShadow: "0 4px 24px rgba(0,0,0,0.4)",
        pointerEvents: "none", // To not block clicks underneath
        opacity: isPaused ? 0.7 : 1,
      }}
    >
      {/* Invisible Turnstile widget */}
      <div style={{ position: "absolute", opacity: 0, pointerEvents: "none" }}>
        <Turnstile
          siteKey={TURNSTILE_SITE_KEY}
          onSuccess={setTurnstileToken}
          options={{ theme: "auto" }}
        />
      </div>

      {isPaused 
        ? "⏸️ 화면을 띄워야 시간이 줄어들어요!" 
        : `🕐 ${remaining}초 뒤 코인 획득 가능`
      }
    </div>
  );
}

// ── Static Toast (성공 / 실패 / 안내) ────────────────────────────────────────

function StaticToast({ state }: { state: Exclude<ToastState, { kind: "countdown" } | null> }) {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const show = setTimeout(() => setVisible(true), 50);
    const hide = setTimeout(() => setVisible(false), 2500);
    return () => { clearTimeout(show); clearTimeout(hide); };
  }, []);

  let message: string;
  let borderColor: string;
  let textColor: string;

  if (state.kind === "success") {
    const { earn } = state;
    if (earn.earned) {
      message = earn.leveled_up
        ? `🎊 +${earn.coins_earned}코인 획득! Lv.${earn.new_level} 레벨 업!`
        : `🎊 +${earn.coins_earned}코인 획득!`;
      borderColor = "var(--color-button-primary)";
      textColor = "var(--color-button-primary)";
    } else {
      // earn failed even after dwell (DUPLICATE etc)
      message =
        earn.reason === "EXPIRED"
          ? "코인 획득 기간이 지났어요."
          : "이미 이 주제의 코인을 받았어요.";
      borderColor = "var(--color-text-secondary)";
      textColor = "var(--color-text-secondary)";
    }
  } else if (state.kind === "info") {
    message = state.message;
    borderColor = "var(--color-text-secondary)";
    textColor = "var(--color-text-secondary)";
  } else {
    // error
    message = state.message;
    borderColor = "var(--color-error)";
    textColor = "var(--color-error)";
  }

  return (
    <div
      style={{
        position: "fixed",
        bottom: "28px",
        left: "50%",
        transform: "translateX(-50%)",
        zIndex: 50,
        background: "var(--color-card-bg)",
        color: textColor,
        border: `1px solid ${borderColor}`,
        borderRadius: "12px",
        padding: "10px 22px",
        fontSize: "14px",
        fontWeight: 600,
        whiteSpace: "nowrap",
        boxShadow: "0 4px 24px rgba(0,0,0,0.4)",
        opacity: visible ? 1 : 0,
        transition: "opacity 0.4s ease",
        pointerEvents: "none",
      }}
    >
      {message}
    </div>
  );
}

// ── Page ─────────────────────────────────────────────────────────────────────

export function TopicPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams] = useSearchParams();
  const [topic, setTopic] = useState<TopicDetail | null>(null);
  const [toast, setToast] = useState<ToastState>(null);
  const [error, setError] = useState<"not_found" | "server_error" | null>(null);
  const [loading, setLoading] = useState(true);

  const uid = searchParams.get("uid") ?? undefined;
  const rid = searchParams.get("rid") ?? undefined;

  // Refs for timer management
  const toastClearRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearTimers = () => {
    if (toastClearRef.current) clearTimeout(toastClearRef.current);
  };

  // Triggered when CountdownToast finishes naturally via rAF
  const handleCountdownComplete = (topicId: string, turnstileToken: string) => {
    if (!uid || !rid) return;
    
    earnCoinFromEmail(uid, rid, topicId, turnstileToken)
      .then((result) => {
        setToast({ kind: "success", earn: result });
        toastClearRef.current = setTimeout(() => setToast(null), 3200);
      })
      .catch((err) => {
        setToast({
          kind: "error",
          message: err?.message === "bot verification failed" 
            ? "자동화된 접근이 감지되었습니다." 
            : "잠시 후 다시 시도해 주세요.",
        });
        toastClearRef.current = setTimeout(() => setToast(null), 3200);
      });
  };

  const startCountdown = (requiredSeconds: number, topicId: string) => {
    setToast({ kind: "countdown", requiredSeconds, topicId });
  };

  useEffect(() => {
    if (!id) return;

    // 1. Topic fetch (always)
    fetchTopicDetail(id)
      .then(setTopic)
      .catch((e: Error) => {
        setError(e.message === "not_found" ? "not_found" : "server_error");
      })
      .finally(() => setLoading(false));

    // 2. Init-earn (only when uid + rid provided)
    if (uid && rid) {
      initEarn(uid, rid, id)
        .then((result) => {
          switch (result.status) {
            case "PENDING": {
              const n = result.required_seconds ?? 10;
              startCountdown(n, id);
              break;
            }
            case "EXPIRED":
              setToast({ kind: "info", message: "코인 획득 기간이 지났어요." });
              toastClearRef.current = setTimeout(() => setToast(null), 3200);
              break;
            case "DUPLICATE":
              setToast({ kind: "info", message: "이미 이 주제의 코인을 받았어요." });
              toastClearRef.current = setTimeout(() => setToast(null), 3200);
              break;
            case "DAILY_LIMIT":
              setToast({ kind: "info", message: "오늘 코인 획득 한도를 채웠어요." });
              toastClearRef.current = setTimeout(() => setToast(null), 3200);
              break;
          }
        })
        .catch(() => {
          setToast({ kind: "error", message: "잠시 후 다시 시도해 주세요." });
          toastClearRef.current = setTimeout(() => setToast(null), 3200);
        });
    }

    return () => clearTimers();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

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
        <div>
          <p className="text-sm mb-3" style={{ color: "var(--color-text-secondary)" }}>
            {formatDate(topic.created_at)}
          </p>
          {topic.buzz_score > 0 && (
            <p className="text-sm font-bold mb-2" style={{ color: "var(--color-error)" }}>
              🔥 화제도 {topic.buzz_score}
            </p>
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

      {/* Toast layer */}
      {toast?.kind === "countdown" && (
        <CountdownToast
          requiredSeconds={toast.requiredSeconds}
          onComplete={(token) => handleCountdownComplete(toast.topicId, token)}
        />
      )}
      {toast && toast.kind !== "countdown" && <StaticToast state={toast} />}
    </div>
  );
}
