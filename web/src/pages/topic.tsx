import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { fetchTopicDetail, type TopicDetail } from "@/lib/api";

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

  useEffect(() => {
    if (!id) return;
    fetchTopicDetail(id)
      .then(setTopic)
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
    <div className="min-h-screen px-6 py-12" style={{ background: "#0f0a19" }}>
      <div className="max-w-2xl mx-auto space-y-6">
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
            <ul className="space-y-3">
              {topic.details.map((detail, i) => (
                <li key={i} className="flex gap-3">
                  <span className="mt-2 w-1.5 h-1.5 rounded-full shrink-0" style={{ backgroundColor: "#9b8bb4" }} />
                  <p className="text-base leading-relaxed" style={{ color: "#d4cee0" }}>
                    {detail}
                  </p>
                </li>
              ))}
            </ul>
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
