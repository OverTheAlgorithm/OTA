import { useEffect, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { fetchTopicDetail, earnPoint, type TopicDetail, type EarnResult } from "@/lib/api";

function formatDate(iso: string): string {
  const d = new Date(iso);  
  return d.toLocaleDateString("ko-KR", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

export function TopicPage() {
  const { id } = useParams<{ id: string }>();
  const [topic, setTopic] = useState<TopicDetail | null>(null);
  const [error, setError] = useState<"not_found" | "server_error" | null>(null);
  const [loading, setLoading] = useState(true);
  const [earnResult, setEarnResult] = useState<EarnResult | null>(null);

  useEffect(() => {
    if (!id) return;
    fetchTopicDetail(id)
      .then((data) => {
        setTopic(data);
        if (data.brain_category === "over_the_algorithm") {
          earnPoint(id).then(setEarnResult).catch(() => {});
        }
      })
      .catch((e: Error) => {
        setError(e.message === "not_found" ? "not_found" : "server_error");
      })
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ background: "#0f0a19" }}>
        <p style={{ color: "#9b8bb4" }}>불러오는 중...</p>
      </div>
    );
  }

  if (error === "not_found") {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ background: "#0f0a19" }}>
        <p style={{ color: "#9b8bb4" }}>존재하지 않는 주제입니다.</p>
      </div>
    );
  }

  if (error || !topic) {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ background: "#0f0a19" }}>
        <p style={{ color: "#9b8bb4" }}>불러오기에 실패했습니다. 잠시 후 다시 시도해 주세요.</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen" style={{ background: "#0f0a19" }}>
      <header className="sticky top-0 z-10 border-b border-[#2d1f42] bg-[#0f0a19]/90 backdrop-blur-lg">
        <div className="max-w-2xl mx-auto px-6 h-16 flex items-center">
          <Link to="/">
            <img src="/OTA_logo.png" alt="OTA" className="w-[63px] h-[42px]" />
          </Link>
        </div>
      </header>
      <div className="max-w-2xl mx-auto px-6 py-8 space-y-6">
        {earnResult?.leveled_up && (
          <div
            className="rounded-xl px-4 py-3 text-center text-sm font-semibold"
            style={{ background: "#1a2e1a", color: "#7bc67e", border: "1px solid #2d4a2d" }}
          >
            🎉 레벨 업! Lv.{earnResult.level} 달성!
          </div>
        )}
        <div>
          <p className="text-sm mb-3" style={{ color: "#9b8bb4" }}>
            {formatDate(topic.created_at)}
          </p>
          {topic.buzz_score > 0 && (
            <p className="text-sm font-bold mb-2" style={{ color: "#e84d3d" }}>
              🔥 화제도 {topic.buzz_score}
            </p>
          )}
          <h1 className="text-2xl font-bold mb-6 leading-snug" style={{ color: "#f5f0ff" }}>
            {topic.topic}
          </h1>
          {topic.details && topic.details.length > 0 ? (
            <div className="space-y-5">
              {topic.details.map((detail, i) => {
                // Backward compat: old data may have plain strings converted to {title, content:""}
                const title = typeof detail === "string" ? detail : detail?.title;
                const content = typeof detail === "string" ? "" : detail?.content;
                if (!title && !content) return null;
                return (
                  <div key={i} className="border-l-2 pl-4" style={{ borderColor: "#2d1f42" }}>
                    {title && (
                      <h3 className="text-base font-semibold leading-snug mb-1.5" style={{ color: "#f5f0ff" }}>
                        {title}
                      </h3>
                    )}
                    {content && (
                      <p className="text-sm leading-relaxed" style={{ color: "#9b8bb4" }}>
                        {content}
                      </p>
                    )}
                  </div>
                );
              })}
            </div>
          ) : topic.detail ? (
            <p className="text-base leading-relaxed" style={{ color: "#d4cee0" }}>
              {topic.detail}
            </p>
          ) : (
            <p className="text-sm" style={{ color: "#9b8bb4" }}>
              추가 정보가 없습니다.
            </p>
          )}
        </div>

        {topic.sources && topic.sources.length > 0 && (
          <div className="rounded-xl border px-5 py-4 space-y-2" style={{ borderColor: "#2d1f42", background: "#1a1229" }}>
            <p className="text-xs font-semibold uppercase tracking-wider" style={{ color: "#9b8bb4" }}>
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
                  style={{ color: "#9b8bb4", border: "1px solid #2d1f42" }}
                  onMouseEnter={e => {
                    e.currentTarget.style.color = "#f5f0ff";
                    e.currentTarget.style.borderColor = "#9b8bb4";
                  }}
                  onMouseLeave={e => {
                    e.currentTarget.style.color = "#9b8bb4";
                    e.currentTarget.style.borderColor = "#2d1f42";
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

