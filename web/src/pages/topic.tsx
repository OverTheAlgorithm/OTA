import { useEffect, useState } from "react";
import { useParams, useSearchParams, Link } from "react-router-dom";
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
  if (earn.earned) {
    if (earn.leveled_up) {
      message = `🎉 +${earn.points_earned}pt 획득! Lv.${earn.new_level} 레벨 업!`;
    } else {
      message = `🌈 +${earn.points_earned}pt 획득!`;
    }
  } else if (earn.reason === "EXPIRED") {
    message = "포인트 획득 기간이 지났어요.";
  } else {
    // DUPLICATE or unknown
    message = "이미 이 주제의 포인트를 받았어요.";
  }

  const bgColor = earn.earned ? "#e8f5e9" : "#f3e5f5";
  const textColor = earn.earned ? "#2e7d32" : "#6b8db5";
  const borderColor = earn.earned ? "#81c784" : "#ce93d8";

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
      <div className="min-h-screen flex items-center justify-center" style={{ background: "white" }}>
        <p style={{ color: "#6b8db5" }}>불러오는 중...</p>
      </div>
    );
  }

  if (error === "not_found") {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ background: "white" }}>
        <p style={{ color: "#6b8db5" }}>존재하지 않는 주제입니다.</p>
      </div>
    );
  }

  if (error || !topic) {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ background: "white" }}>
        <p style={{ color: "#6b8db5" }}>불러오기에 실패했습니다. 잠시 후 다시 시도해 주세요.</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen" style={{ background: "white" }}>
      <header className="sticky top-0 z-10 border-b border-[#d4e6f5] bg-white/90 backdrop-blur-lg">
        <div className="max-w-2xl mx-auto px-6 h-16 flex items-center">
          <Link to="/">
            <img src="/OTA_logo.png" alt="OTA" className="w-[63px] h-[42px]" />
          </Link>
        </div>
      </header>

      <div className="max-w-2xl mx-auto px-6 py-8 space-y-6">
        <div>
          <p className="text-sm mb-3" style={{ color: "#6b8db5" }}>
            {formatDate(topic.created_at)}
          </p>
          {topic.buzz_score > 0 && (
            <p className="text-sm font-bold mb-2" style={{ color: "#ff5442" }}>
              🔥 화제도 {topic.buzz_score}
            </p>
          )}
          <h1 className="text-2xl font-bold mb-6 leading-snug" style={{ color: "#1e3a5f" }}>
            {topic.topic}
          </h1>
          {topic.details && topic.details.length > 0 ? (
            <div className="space-y-5">
              {topic.details.map((detail, i) => {
                const title = typeof detail === "string" ? detail : detail?.title;
                const content = typeof detail === "string" ? "" : detail?.content;
                if (!title && !content) return null;
                return (
                  <div key={i} className="border-l-2 pl-4" style={{ borderColor: "#d4e6f5" }}>
                    {title && (
                      <h3 className="text-base font-semibold leading-snug mb-1.5" style={{ color: "#1e3a5f" }}>
                        {title}
                      </h3>
                    )}
                    {content && (
                      <p className="text-sm leading-relaxed" style={{ color: "#6b8db5" }}>
                        {content}
                      </p>
                    )}
                  </div>
                );
              })}
            </div>
          ) : topic.detail ? (
            <p className="text-base leading-relaxed" style={{ color: "#335071" }}>
              {topic.detail}
            </p>
          ) : (
            <p className="text-sm" style={{ color: "#6b8db5" }}>
              추가 정보가 없습니다.
            </p>
          )}
        </div>

        {topic.sources && topic.sources.length > 0 && (
          <div className="rounded-xl border px-5 py-4 space-y-2" style={{ borderColor: "#d4e6f5", background: "#f0f7ff" }}>
            <p className="text-xs font-semibold uppercase tracking-wider" style={{ color: "#6b8db5" }}>
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
                  style={{ color: "#6b8db5", border: "1px solid #d4e6f5" }}
                  onMouseEnter={e => {
                    e.currentTarget.style.color = "#1e3a5f";
                    e.currentTarget.style.borderColor = "#6b8db5";
                  }}
                  onMouseLeave={e => {
                    e.currentTarget.style.color = "#6b8db5";
                    e.currentTarget.style.borderColor = "#d4e6f5";
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
