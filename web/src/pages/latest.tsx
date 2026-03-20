import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { UserLevelCard } from "@/components/user-level-card";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import {
  fetchLatestRunTopics,
  getBrainCategories,
  getSubscriptions,
  batchEarnStatus,
  getDefaultImage,
  getPicsumImage,
  type TopicPreview,
  type BrainCategory,
  type EarnStatusItem,
} from "@/lib/api";

function formatDate(iso: string): string {
  const d = new Date(iso);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}.${m}.${day}`;
}

const CATEGORY_LABELS: Record<string, string> = {
  general: "📰 일반",
  entertainment: "🎬 연예/오락",
  business: "💰 경제/비즈니스",
  sports: "⚽ 스포츠",
  technology: "💻 기술",
  science: "🔬 과학",
  health: "🏥 건강",
};

function CoinTag({ status }: { status: EarnStatusItem }) {
  if (status.status === "DUPLICATE") {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/50">
        획득!
      </span>
    );
  }
  if (status.status === "EXPIRED") {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/50">
        획득 기간 경과
      </span>
    );
  }
  if (status.status === "DAILY_LIMIT") {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#231815]/10 text-[#231815]/50">
        일일 한도
      </span>
    );
  }
  if (status.status === "PENDING" && status.coins > 0) {
    return (
      <span className="px-2.5 py-0.5 rounded-full text-[10px] font-bold bg-[#43b9d6]/15 text-[#43b9d6]">
        +{status.coins}포인트
      </span>
    );
  }
  return null;
}

function isPreferredTopic(priority: string, category: string, subscriptions: string[]): boolean {
  if (priority === "top" || priority === "brief") return true;
  return subscriptions.includes(category);
}

interface TopicGroup {
  key: string;
  emoji: string;
  label: string;
  items: TopicPreview[];
}

function groupByBrainCategory(
  items: TopicPreview[],
  brainCategories: BrainCategory[],
): TopicGroup[] {
  const map = new Map<string, TopicPreview[]>();
  for (const item of items) {
    const key = item.brain_category ?? "";
    const arr = map.get(key) ?? [];
    arr.push(item);
    map.set(key, arr);
  }

  const groups: TopicGroup[] = [];
  for (const bc of brainCategories) {
    const arr = map.get(bc.key);
    if (arr && arr.length > 0) {
      groups.push({ key: bc.key, emoji: bc.emoji, label: bc.label, items: arr });
    }
  }
  // Ungrouped items
  const ungrouped = map.get("");
  if (ungrouped && ungrouped.length > 0) {
    groups.push({ key: "", emoji: "📌", label: "기타", items: ungrouped });
  }
  return groups;
}

function NewsItem({
  topic,
  earnStatus,
}: {
  topic: TopicPreview;
  earnStatus?: EarnStatusItem;
}) {
  const categoryLabel = topic.category ? CATEGORY_LABELS[topic.category] ?? topic.category : "";

  // Truncate summary
  const summary = topic.summary.length > 120
    ? topic.summary.slice(0, 117) + "..."
    : topic.summary;

  return (
    <Link
      to={`/topic/${topic.id}`}
      className="group block border border-[#231815] rounded-lg overflow-hidden hover:shadow-md transition-shadow"
    >
      <div className="flex flex-col sm:flex-row">
        <div className="sm:w-[180px] sm:flex-shrink-0 aspect-[16/10] sm:aspect-auto overflow-hidden bg-[#f0ece0]">
          <img
            src={topic.image_url || getPicsumImage(topic.id)}
            alt=""
            className="w-full h-full object-cover [image-rendering:-webkit-optimize-contrast] [will-change:transform]"
            onError={(e) => {
              const fallback = getDefaultImage(topic.id);
              if (e.currentTarget.src !== fallback) e.currentTarget.src = fallback;
            }}
          />
        </div>
        <div className="flex-1 p-4">
          <div className="flex items-center justify-between gap-2 mb-1">
            <div className="flex items-center gap-2">
              {topic.created_at && (
                <span className="text-xs font-bold text-[#231815]">
                  {formatDate(topic.created_at)}
                </span>
              )}
              {categoryLabel && (
                <span className="text-xs font-bold text-[#231815]">
                  {categoryLabel}
                </span>
              )}
            </div>
            {earnStatus && <CoinTag status={earnStatus} />}
          </div>
          <h3 className="text-[15px] font-semibold text-[#231815] leading-snug line-clamp-2 group-hover:opacity-70 transition-opacity">
            {topic.topic}
          </h3>
          <p className="text-[13px] text-[#231815]/70 mt-1 leading-relaxed line-clamp-2">
            {summary}
          </p>
        </div>
      </div>
    </Link>
  );
}

export function LatestPage() {
  const { user } = useAuth();
  const navigate = useNavigate();

  const [topics, setTopics] = useState<TopicPreview[]>([]);
  const [brainCategories, setBrainCategories] = useState<BrainCategory[]>([]);
  const [subscriptions, setSubscriptions] = useState<string[]>([]);
  const [earnMap, setEarnMap] = useState<Record<string, EarnStatusItem>>({});
  const [loading, setLoading] = useState(true);
  const [unearnedOnly, setUnearnedOnly] = useState(false);

  useEffect(() => {
    Promise.all([
      fetchLatestRunTopics(),
      getBrainCategories(),
    ])
      .then(([topicData, bcData]) => {
        setTopics(topicData);
        setBrainCategories(bcData);

        if (user && topicData.length > 0) {
          const ids = topicData.map((t) => t.id);
          batchEarnStatus(ids)
            .then((statuses) => {
              const map: Record<string, EarnStatusItem> = {};
              for (const s of statuses) map[s.id] = s;
              setEarnMap(map);
            })
            .catch(() => {});
        }
      })
      .catch(() => {})
      .finally(() => setLoading(false));

    if (user) {
      getSubscriptions().then(setSubscriptions).catch(() => {});
    }
  }, [user]);

  // Derive the run date from topics
  const runDate = topics.length > 0 && topics[0].created_at
    ? formatDate(topics[0].created_at)
    : "";

  // Filter topics if "포인트 획득 전" is checked
  const filteredTopics = unearnedOnly
    ? topics.filter((t) => {
        const status = earnMap[t.id];
        return !status || status.status === "PENDING";
      })
    : topics;

  // Split into preferred/non-preferred (only for logged-in users)
  const preferredTopics = user
    ? filteredTopics.filter((t) =>
        isPreferredTopic(t.priority ?? "", t.category ?? "", subscriptions),
      )
    : filteredTopics;

  const nonPreferredTopics = user
    ? filteredTopics.filter(
        (t) => !isPreferredTopic(t.priority ?? "", t.category ?? "", subscriptions),
      )
    : [];

  // If all are non-preferred (no subscriptions match), show all as preferred
  const showPreferred = preferredTopics.length > 0 ? preferredTopics : filteredTopics;
  const showNonPreferred = preferredTopics.length > 0 ? nonPreferredTopics : [];

  const preferredGroups = groupByBrainCategory(showPreferred, brainCategories);
  const nonPreferredGroups = groupByBrainCategory(showNonPreferred, brainCategories);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#fdf9ee]">
        <p className="text-[#231815]/60">불러오는 중...</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex flex-col bg-[#fdf9ee]">
      <Header />

      <main className="flex-1">
        <div className="max-w-[900px] mx-auto px-6 py-8">
          {/* Level Card */}
          <div className="mb-8">
            <UserLevelCard />
          </div>

          {/* Title + Filter */}
          <div className="flex items-center gap-4 mb-8 flex-wrap">
            <div>
              <h1 className="text-2xl font-bold text-[#231815]">
                {runDate} 최신 소식 확인하기
              </h1>
              <p className="text-xs text-[#231815]/50 mt-1">
                소식은 매일 아침 7시에 새롭게 업데이트됩니다
              </p>
            </div>
            {user && (
              <label className="flex items-center gap-2 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={unearnedOnly}
                  onChange={(e) => setUnearnedOnly(e.target.checked)}
                  className="w-4 h-4 rounded border-[#231815] text-[#43b9d6] focus:ring-[#43b9d6] cursor-pointer"
                />
                <span className="text-sm text-[#231815]/70">포인트 획득 전</span>
              </label>
            )}
          </div>

          {filteredTopics.length === 0 ? (
            <div className="text-center py-20">
              <p className="text-[#231815]/50 text-sm">
                {unearnedOnly ? "모든 포인트를 획득했어요!" : "소식이 없습니다."}
              </p>
            </div>
          ) : (
            <div className="space-y-10">
              {/* Preferred sections */}
              {preferredGroups.map((group) => (
                <section key={group.key}>
                  <div className="border-b border-[#dbdade] mb-4">
                    <div className="inline-block pb-2 border-b-[3px] border-[#008fb2]">
                      <h2 className="text-base font-medium text-[#231815]">
                        {group.emoji} {group.label}
                      </h2>
                    </div>
                  </div>
                  <div className="space-y-3">
                    {group.items.map((topic) => (
                      <NewsItem
                        key={topic.id}
                        topic={topic}
                        earnStatus={earnMap[topic.id]}
                      />
                    ))}
                  </div>
                </section>
              ))}

              {/* Non-preferred divider + sections */}
              {showNonPreferred.length > 0 && nonPreferredGroups.length > 0 && (
                <>
                  <div className="rounded-lg border border-[#43b9d6] bg-white px-5 py-3.5">
                    <p className="text-sm font-bold text-[#43b9d6]">
                      🌱 시야를 넓힐 기회에요
                    </p>
                    <p className="text-xs text-[#231815] mt-0.5">
                      구독하지 않은 주제예요. 읽으면 더 많은 포인트를 얻어요!
                    </p>
                  </div>

                  {nonPreferredGroups.map((group) => (
                    <section key={`np-${group.key}`}>
                      <div className="border-b border-[#dbdade] mb-4">
                        <div className="inline-block pb-2 border-b-[3px] border-[#008fb2]">
                          <h2 className="text-base font-medium text-[#231815]">
                            {group.emoji} {group.label}
                          </h2>
                        </div>
                      </div>
                      <div className="space-y-3">
                        {group.items.map((topic) => (
                          <NewsItem
                            key={topic.id}
                            topic={topic}
                            earnStatus={earnMap[topic.id]}
                          />
                        ))}
                      </div>
                    </section>
                  ))}
                </>
              )}
            </div>
          )}

          {/* CTA to all news */}
          <div className="flex justify-center mt-12">
            <button
              onClick={() => navigate("/allnews")}
              className="px-8 py-3 rounded-full text-sm font-semibold text-[#231815] border-[2px] border-[#231815] bg-[#43b9d6] hover:brightness-110 transition-all"
            >
              모든 소식 보러가기
            </button>
          </div>
        </div>
      </main>

      <Footer />
    </div>
  );
}
