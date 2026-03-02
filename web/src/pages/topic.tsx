import { useEffect, useState } from "react";
import { useParams, useSearchParams, Link } from "react-router-dom";
import { useTheme } from "@/contexts/theme-context";
import { fetchTopicDetail, type TopicDetail, type TopicEarnResult } from "@/lib/api";


function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString("ko-KR", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

// ── Toast ────────────────────────────────────────────────────────────────────
interface ToastProps {
  earn: TopicEarnResult;
}

function PointToast({ earn }: ToastProps) {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    // 마운트 직후 fade-in
    const showTimer = setTimeout(() => setVisible(true), 50);
    // 2.5초 뒤 fade-out 시작
    const hideTimer = setTimeout(() => setVisible(false), 2500);
    return () => {
      clearTimeout(showTimer);
      clearTimeout(hideTimer);
    };
  }, []);

  let message: string;
  let bgColor: string;
  let textColor: string;
  let borderColor: string;

  if (earn.earned) {
    if (earn.leveled_up) {
      message = `🎉 +${earn.points_earned}pt 획득! Lv.${earn.new_level} 레벨 업!`;
    } else {
      message = `🌈 +${earn.points_earned}pt 획득!`;
    }
    bgColor = "var(--color-card-bg)";
    textColor = "var(--color-button-primary)";
    borderColor = "var(--color-button-primary)";
  } else if (earn.reason === "EXPIRED") {
    message = "포인트 획득 기간이 지났어요.";
    bgColor = "var(--color-card-bg)";
    textColor = "var(--color-text-secondary)";
    borderColor = "var(--color-text-secondary)";
  } else {
    // DUPLICATE or unknown
    message = "이미 이 주제의 포인트를 받았어요.";
    bgColor = "var(--color-card-bg)";
    textColor = "var(--color-text-secondary)";
    borderColor = "var(--color-text-secondary)";
  }

  return (
    <div
      style={{
        position: "fixed",
        bottom: "28px",
        left: "50%",
        transform: "translateX(-50%)",
        zIndex: 50,
        background: bgColor,
        color: textColor,
        border: `1px solid ${borderColor}`,
        borderRadius: "12px",
        padding: "10px 20px",
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
  const { toggleTheme } = useTheme();
  const [topic, setTopic] = useState<TopicDetail | null>(null);
  const [earn, setEarn] = useState<TopicEarnResult | null>(null);
  const [showToast, setShowToast] = useState(false);
  const [error, setError] = useState<"not_found" | "server_error" | null>(null);
  const [loading, setLoading] = useState(true);

  const uid = searchParams.get("uid") ?? undefined;
  const rid = searchParams.get("rid") ?? undefined;
  const pts = searchParams.get("pts") ?? undefined;
  const hasTracking = Boolean(uid && rid);

  useEffect(() => {
    if (!id) return;
    fetchTopicDetail(id, { uid, rid, pts })
      .then((res) => {
        setTopic(res.data);
        if (hasTracking && res.earn_result) {
          setEarn(res.earn_result);
          setShowToast(true);
          // 토스트 컴포넌트 자체 fade-out(2.5s) + 추가 400ms 뒤 언마운트
          setTimeout(() => setShowToast(false), 3000);
        }
      })
      .catch((e: Error) => {
        setError(e.message === "not_found" ? "not_found" : "server_error");
      })
      .finally(() => setLoading(false));
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
      style={{
        backgroundColor: "var(--color-bg)",
        color: "var(--color-fg)"
      }}
    >
      <header
        className="sticky top-0 z-10 border-b bg-opacity-90 backdrop-blur-lg"
        style={{
          borderColor: "var(--color-border)",
          backgroundColor: "var(--color-bg)"
        }}
      >
        <div className="max-w-2xl mx-auto px-6 h-16 flex items-center justify-between">
          <Link to="/">
            <img src="/OTA_logo.png" alt="OTA" className="w-[63px] h-[42px]" />
          </Link>
          <button
            onClick={toggleTheme}
            style={{ color: "var(--color-text-secondary)" }}
            className="text-sm hover:opacity-70 transition-opacity"
            title="테마 전환"
          >
            🌙
          </button>
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

      {/* 포인트 토스트 — uid+rid가 있을 때만 렌더링 */}
      {showToast && earn && <PointToast earn={earn} />}
    </div>
  );
}
