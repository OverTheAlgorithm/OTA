// Ported from: web/src/pages/topic.tsx
// Anti-cheat L4: setInterval + Date.now() (replaces rAF + performance.now())
// Anti-cheat L5: AppState listener (replaces visibilitychange)
import React, { useCallback, useEffect, useRef, useState } from "react";
import {
  View,
  Text,
  Image,
  ScrollView,
  StyleSheet,
  Pressable,
  ActivityIndicator,
  AppState,
  AppStateStatus,
  Linking,
} from "react-native";
import { useRoute, useNavigation, RouteProp } from "@react-navigation/native";
import { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { api } from "../lib/api";
import { TopicDetail, BrainCategory } from "../../../packages/shared/src/types";
import { QuizCard } from "../components/QuizCard";
import { formatDate } from "../../../packages/shared/src/utils";

type RootStackParamList = Record<string, object | undefined> & {
  TopicDetail: { id: string };
};

type TopicDetailRouteProp = RouteProp<RootStackParamList, "TopicDetail">;
type NavProp = NativeStackNavigationProp<RootStackParamList>;

const CATEGORY_LABELS: Record<string, string> = {
  general: "종합",
  entertainment: "연예",
  business: "경제",
  sports: "스포츠",
  technology: "IT",
  science: "과학",
  health: "건강",
};

type CoinTagState =
  | { kind: "expired" }
  | { kind: "not_logged_in" }
  | { kind: "duplicate" }
  | { kind: "daily_limit" }
  | { kind: "server_error" }
  | { kind: "countdown"; remaining: number; isPaused: boolean }
  | { kind: "success"; coins: number; leveledUp: boolean; newLevel: number }
  | { kind: "loading" }
  | null;

function CoinTag({ state }: { state: CoinTagState }) {
  if (!state) return null;

  let label: string;
  let color: string;
  let bgColor: string;

  switch (state.kind) {
    case "loading":
      label = "대기 중...";
      color = "#43b9d6";
      bgColor = "rgba(67,185,214,0.15)";
      break;
    case "expired":
      label = "획득 기간 경과";
      color = "#888";
      bgColor = "#e8e8e8";
      break;
    case "not_logged_in":
      label = "로그인 필요";
      color = "#888";
      bgColor = "#e8e8e8";
      break;
    case "duplicate":
      label = "획득 완료";
      color = "#888";
      bgColor = "#e8e8e8";
      break;
    case "daily_limit":
      label = "일일 한도 도달";
      color = "#888";
      bgColor = "#e8e8e8";
      break;
    case "server_error":
      label = "잠시 후에 다시 시도해주세요";
      color = "#d94040";
      bgColor = "rgba(217,64,64,0.12)";
      break;
    case "countdown":
      label = state.isPaused ? "일시정지" : `${state.remaining}초`;
      color = "#43b9d6";
      bgColor = "rgba(67,185,214,0.15)";
      break;
    case "success":
      label = state.leveledUp
        ? `+${state.coins} Lv.${state.newLevel}!`
        : `+${state.coins} 획득!`;
      color = "#43b9d6";
      bgColor = "rgba(67,185,214,0.15)";
      break;
  }

  return (
    <View style={[styles.coinTag, { backgroundColor: bgColor }]}>
      {state.kind === "countdown" && !state.isPaused && (
        <View style={[styles.coinTagDot, { backgroundColor: color }]} />
      )}
      <Text style={[styles.coinTagText, { color }]}>{label}</Text>
    </View>
  );
}

export function TopicDetailScreen() {
  const route = useRoute<TopicDetailRouteProp>();
  const navigation = useNavigation<NavProp>();
  const { id } = route.params as { id: string };

  const [topic, setTopic] = useState<TopicDetail | null>(null);
  const [brainCategories, setBrainCategories] = useState<BrainCategory[]>([]);
  const [coinTag, setCoinTag] = useState<CoinTagState>(null);
  const [error, setError] = useState<"not_found" | "server_error" | null>(null);
  const [loading, setLoading] = useState(true);

  // Countdown timer state — managed via setInterval + Date.now()
  const [countdownSeconds, setCountdownSeconds] = useState<number | null>(null);
  const isPausedRef = useRef(false);
  const earnCalledRef = useRef(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const lastTickRef = useRef<number>(0);
  const remainingRef = useRef<number>(0);

  const stopTimer = useCallback(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
  }, []);

  const handleEarnComplete = useCallback(
    async (topicId: string) => {
      if (earnCalledRef.current) return;
      earnCalledRef.current = true;
      stopTimer();
      setCountdownSeconds(null);
      setCoinTag({ kind: "loading" });

      try {
        // Pass empty string for turnstile_token — mobile skips Turnstile
        const result = await api.earnCoin(topicId, "");
        if (result.earned) {
          setCoinTag({
            kind: "success",
            coins: result.coins_earned,
            leveledUp: result.leveled_up,
            newLevel: result.new_level,
          });
          setTimeout(() => setCoinTag({ kind: "duplicate" }), 3000);
        } else {
          const reason = result.reason === "EXPIRED" ? "expired" : "duplicate";
          setCoinTag(reason === "expired" ? { kind: "expired" } : { kind: "duplicate" });
        }
      } catch {
        setCoinTag({ kind: "server_error" });
      }
    },
    [stopTimer],
  );

  const startCountdown = useCallback(
    (seconds: number, topicId: string) => {
      remainingRef.current = seconds;
      setCountdownSeconds(seconds);
      lastTickRef.current = Date.now();

      intervalRef.current = setInterval(() => {
        if (isPausedRef.current) {
          // Reset last tick so we don't accumulate paused time
          lastTickRef.current = Date.now();
          return;
        }

        const now = Date.now();
        const elapsed = now - lastTickRef.current;
        lastTickRef.current = now;

        if (elapsed >= 900) {
          remainingRef.current = Math.max(0, remainingRef.current - 1);
          setCountdownSeconds(remainingRef.current);
          setCoinTag({ kind: "countdown", remaining: remainingRef.current, isPaused: false });

          if (remainingRef.current <= 0) {
            handleEarnComplete(topicId);
          }
        }
      }, 1000);
    },
    [handleEarnComplete],
  );

  // Anti-cheat L5: pause timer when app goes to background
  useEffect(() => {
    const subscription = AppState.addEventListener(
      "change",
      (nextState: AppStateStatus) => {
        const paused = nextState !== "active";
        isPausedRef.current = paused;
        if (countdownSeconds !== null && countdownSeconds > 0) {
          setCoinTag({
            kind: "countdown",
            remaining: remainingRef.current,
            isPaused: paused,
          });
        }
      },
    );
    return () => subscription.remove();
  }, [countdownSeconds]);

  // Fetch topic and init earn on mount
  useEffect(() => {
    if (!id) return;
    earnCalledRef.current = false;

    Promise.all([
      api.fetchTopicDetail(id),
      api.getBrainCategories(),
    ])
      .then(([topicData, bcData]) => {
        setTopic(topicData);
        setBrainCategories(bcData);
      })
      .catch((e: Error) => {
        setError(e.message === "not_found" ? "not_found" : "server_error");
      })
      .finally(() => setLoading(false));

    // Try to init earn (will fail silently if not logged in — server returns 401)
    api
      .initEarn(id)
      .then((result) => {
        switch (result.status) {
          case "PENDING": {
            const secs = result.required_seconds ?? 10;
            startCountdown(secs, id);
            setCoinTag({ kind: "countdown", remaining: secs, isPaused: false });
            break;
          }
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
        // Not logged in or other error — show nothing
        setCoinTag(null);
      });

    return () => {
      stopTimer();
    };
  }, [id, startCountdown, stopTimer]);

  if (loading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator size="large" color="#43b9d6" />
      </View>
    );
  }

  if (error === "not_found") {
    return (
      <View style={styles.centered}>
        <Text style={styles.errorText}>존재하지 않는 주제입니다.</Text>
      </View>
    );
  }

  if (error || !topic) {
    return (
      <View style={styles.centered}>
        <Text style={styles.errorText}>불러오기에 실패했습니다. 잠시 후 다시 시도해 주세요.</Text>
      </View>
    );
  }

  const categoryLabel = topic.category ? CATEGORY_LABELS[topic.category] ?? topic.category : "";
  const brainCat = topic.brain_category
    ? brainCategories.find((bc) => bc.key === topic.brain_category)
    : undefined;

  const earnDone =
    coinTag?.kind === "success" || coinTag?.kind === "duplicate";

  return (
    <ScrollView style={styles.screen} contentContainerStyle={styles.content}>
      {/* Hero image */}
      {topic.image_url && (
        <Image
          source={{ uri: topic.image_url }}
          style={styles.heroImage}
          resizeMode="cover"
        />
      )}

      {/* Date */}
      <Text style={styles.date}>{formatDate(topic.created_at)}</Text>

      {/* Category + brain category + coin tag row */}
      <View style={styles.tagRow}>
        {categoryLabel ? (
          <View style={styles.categoryTag}>
            <Text style={styles.categoryTagText}>{categoryLabel}</Text>
          </View>
        ) : null}
        {brainCat && (
          <View
            style={[
              styles.brainTag,
              { backgroundColor: `${brainCat.accent_color}20` },
            ]}
          >
            <Text style={[styles.brainTagText, { color: brainCat.accent_color }]}>
              {brainCat.emoji} {brainCat.label}
            </Text>
          </View>
        )}
        <CoinTag state={coinTag} />
      </View>

      {/* Title */}
      <Text style={styles.title}>{topic.topic}</Text>

      {/* Detail sections */}
      {topic.details && topic.details.length > 0 ? (
        <View style={styles.detailsContainer}>
          {topic.details.map((detail, i) => {
            const titleText = typeof detail === "string" ? detail : detail?.title;
            const content = typeof detail === "string" ? "" : detail?.content;
            if (!titleText && !content) return null;
            return (
              <View key={i} style={styles.detailItem}>
                {titleText ? (
                  <Text style={styles.detailTitle}>{titleText}</Text>
                ) : null}
                {content ? (
                  <Text style={styles.detailContent}>{content}</Text>
                ) : null}
              </View>
            );
          })}
        </View>
      ) : topic.detail ? (
        <Text style={styles.detailFallback}>{topic.detail}</Text>
      ) : (
        <Text style={styles.noDetail}>추가 정보가 없습니다.</Text>
      )}

      {/* Quiz card — shown when has_quiz, not daily_limit, not expired */}
      {topic.has_quiz &&
        coinTag?.kind !== "daily_limit" &&
        coinTag?.kind !== "expired" && (
          <QuizCard
            quiz={topic.quiz}
            hasQuiz={topic.has_quiz}
            earnDone={earnDone}
            contextItemId={topic.id}
          />
        )}

      {/* Sources */}
      {topic.sources && topic.sources.length > 0 && (
        <View style={styles.sourcesContainer}>
          <Text style={styles.sourcesTitle}>출처</Text>
          <View style={styles.sourcesList}>
            {topic.sources.map((src, i) => (
              <Pressable
                key={i}
                style={styles.sourceButton}
                onPress={() => Linking.openURL(src)}
              >
                <Text style={styles.sourceButtonText}>출처 {i + 1}</Text>
              </Pressable>
            ))}
          </View>
        </View>
      )}

      <View style={styles.bottomPad} />
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: "#fdf9ee",
  },
  content: {
    paddingHorizontal: 20,
    paddingTop: 16,
  },
  centered: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "#fdf9ee",
  },
  errorText: {
    fontSize: 14,
    color: "rgba(35,24,21,0.60)",
    textAlign: "center",
    paddingHorizontal: 24,
  },
  heroImage: {
    width: "100%",
    height: 220,
    borderRadius: 12,
    marginBottom: 20,
    backgroundColor: "#f0ece0",
  },
  date: {
    fontSize: 20,
    fontWeight: "700",
    color: "#231815",
    marginBottom: 8,
  },
  tagRow: {
    flexDirection: "row",
    flexWrap: "wrap",
    alignItems: "center",
    gap: 8,
    marginBottom: 12,
  },
  categoryTag: {
    borderRadius: 20,
    paddingHorizontal: 10,
    paddingVertical: 3,
    backgroundColor: "rgba(35,24,21,0.13)",
  },
  categoryTagText: {
    fontSize: 11,
    fontWeight: "700",
    color: "#231815",
  },
  brainTag: {
    borderRadius: 20,
    paddingHorizontal: 10,
    paddingVertical: 3,
  },
  brainTagText: {
    fontSize: 11,
    fontWeight: "700",
  },
  coinTag: {
    flexDirection: "row",
    alignItems: "center",
    borderRadius: 20,
    paddingHorizontal: 10,
    paddingVertical: 3,
  },
  coinTagDot: {
    width: 6,
    height: 6,
    borderRadius: 3,
    marginRight: 4,
  },
  coinTagText: {
    fontSize: 11,
    fontWeight: "700",
  },
  title: {
    fontSize: 24,
    fontWeight: "700",
    color: "#231815",
    lineHeight: 32,
    marginBottom: 24,
  },
  detailsContainer: {
    gap: 20,
    marginBottom: 32,
  },
  detailItem: {
    borderLeftWidth: 3,
    borderLeftColor: "#43b9d6",
    paddingLeft: 16,
  },
  detailTitle: {
    fontSize: 16,
    fontWeight: "700",
    color: "#231815",
    lineHeight: 22,
    marginBottom: 6,
  },
  detailContent: {
    fontSize: 15,
    lineHeight: 24,
    color: "rgba(35,24,21,0.80)",
  },
  detailFallback: {
    fontSize: 15,
    lineHeight: 24,
    color: "#231815",
    marginBottom: 32,
  },
  noDetail: {
    fontSize: 13,
    color: "rgba(35,24,21,0.60)",
    marginBottom: 32,
  },
  sourcesContainer: {
    borderRadius: 16,
    borderWidth: 1,
    borderColor: "rgba(35,24,21,0.20)",
    paddingHorizontal: 20,
    paddingVertical: 16,
    marginBottom: 24,
  },
  sourcesTitle: {
    fontSize: 16,
    fontWeight: "700",
    color: "#231815",
    marginBottom: 12,
  },
  sourcesList: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 10,
  },
  sourceButton: {
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 20,
    borderWidth: 1,
    borderColor: "#231815",
    backgroundColor: "#fff",
  },
  sourceButtonText: {
    fontSize: 13,
    fontWeight: "500",
    color: "#231815",
  },
  bottomPad: {
    height: 40,
  },
});
