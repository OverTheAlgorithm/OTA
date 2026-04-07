// Ported from: web/src/pages/latest.tsx
import React, { useCallback, useEffect, useRef, useState } from "react";
import {
  View,
  Text,
  SectionList,
  StyleSheet,
  RefreshControl,
  ActivityIndicator,
  Pressable,
} from "react-native";
import { useNavigation } from "@react-navigation/native";
import { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { api } from "../lib/api";
import { TopicPreview, BrainCategory, EarnStatusItem } from "../../../packages/shared/src/types";
import { TopicCard } from "../components/TopicCard";
import { formatDate } from "../../../packages/shared/src/utils";

// RootStackParamList will be defined by worker-3 in navigation setup.
// Using a loose type here to avoid circular dependency.
type NavProp = NativeStackNavigationProp<Record<string, object | undefined>>;

interface TopicSection {
  key: string;
  emoji: string;
  label: string;
  isNonPreferred?: boolean;
  data: TopicPreview[];
}

function isPreferredTopic(priority: string, category: string, subscriptions: string[]): boolean {
  if (priority === "top" || priority === "brief") return true;
  return subscriptions.includes(category);
}

function groupByBrainCategory(
  items: TopicPreview[],
  brainCategories: BrainCategory[],
  keyPrefix = "",
): TopicSection[] {
  const map = new Map<string, TopicPreview[]>();
  for (const item of items) {
    const key = item.brain_category ?? "";
    const arr = map.get(key) ?? [];
    arr.push(item);
    map.set(key, arr);
  }

  const sections: TopicSection[] = [];
  for (const bc of brainCategories) {
    const arr = map.get(bc.key);
    if (arr && arr.length > 0) {
      sections.push({
        key: `${keyPrefix}${bc.key}`,
        emoji: bc.emoji,
        label: bc.label,
        data: arr,
      });
    }
  }
  const ungrouped = map.get("");
  if (ungrouped && ungrouped.length > 0) {
    sections.push({ key: `${keyPrefix}other`, emoji: "📌", label: "기타", data: ungrouped });
  }
  return sections;
}

export function LatestScreen() {
  const navigation = useNavigation<NavProp>();

  const [topics, setTopics] = useState<TopicPreview[]>([]);
  const [brainCategories, setBrainCategories] = useState<BrainCategory[]>([]);
  const [subscriptions, setSubscriptions] = useState<string[]>([]);
  const [earnMap, setEarnMap] = useState<Record<string, EarnStatusItem>>({});
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [unearnedOnly, setUnearnedOnly] = useState(false);

  // Track if user is logged in via presence of authToken (worker-3 sets this)
  // For now we check earnMap — if non-empty, user is logged in
  const isLoggedIn = Object.keys(earnMap).length > 0 || subscriptions.length > 0;

  const brainCategoryMap = brainCategories.reduce<
    Record<string, { emoji: string; label: string; accent_color?: string }>
  >((acc, bc) => {
    acc[bc.key] = { emoji: bc.emoji, label: bc.label, accent_color: bc.accent_color };
    return acc;
  }, {});

  const loadData = useCallback(async () => {
    try {
      const [topicData, bcData] = await Promise.all([
        api.fetchLatestRunTopics(),
        api.getBrainCategories(),
      ]);
      setTopics(topicData);
      setBrainCategories(bcData);

      if (topicData.length > 0) {
        const ids = topicData.map((t) => t.id);
        api.batchEarnStatus(ids)
          .then((statuses) => {
            const map: Record<string, EarnStatusItem> = {};
            for (const s of statuses) map[s.id] = s;
            setEarnMap(map);
          })
          .catch(() => {});
      }

      api.getSubscriptions().then(setSubscriptions).catch(() => {});
    } catch {
      // ignore
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const onRefresh = useCallback(() => {
    setRefreshing(true);
    loadData();
  }, [loadData]);

  const filteredTopics = unearnedOnly
    ? topics.filter((t) => {
        const status = earnMap[t.id];
        return !status || status.status === "PENDING";
      })
    : topics;

  // Split preferred / non-preferred
  const preferredTopics = filteredTopics.filter((t) =>
    isPreferredTopic(t.priority ?? "", t.category ?? "", subscriptions),
  );
  const nonPreferredTopics = filteredTopics.filter(
    (t) => !isPreferredTopic(t.priority ?? "", t.category ?? "", subscriptions),
  );

  const showPreferred = preferredTopics.length > 0 ? preferredTopics : filteredTopics;
  const showNonPreferred = preferredTopics.length > 0 ? nonPreferredTopics : [];

  const preferredSections = groupByBrainCategory(showPreferred, brainCategories, "p-");
  const nonPreferredSections = groupByBrainCategory(showNonPreferred, brainCategories, "np-").map(
    (s) => ({ ...s, isNonPreferred: true }),
  );

  // Insert a divider section before non-preferred if needed
  const dividerSection: TopicSection | null =
    showNonPreferred.length > 0 && nonPreferredSections.length > 0
      ? { key: "__divider__", emoji: "", label: "__divider__", data: [] }
      : null;

  const sections: TopicSection[] = [
    ...preferredSections,
    ...(dividerSection ? [dividerSection] : []),
    ...nonPreferredSections,
  ];

  const runDate =
    topics.length > 0 && topics[0].created_at ? formatDate(topics[0].created_at) : "";

  if (loading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator size="large" color="#43b9d6" />
      </View>
    );
  }

  return (
    <SectionList
      style={styles.list}
      contentContainerStyle={styles.content}
      sections={sections}
      keyExtractor={(item) => item.id}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor="#43b9d6" />
      }
      ListHeaderComponent={
        <View style={styles.header}>
          <Text style={styles.title}>{runDate} 최신 소식 확인하기</Text>
          <Text style={styles.subtitle}>소식은 매일 아침 7시에 새롭게 업데이트됩니다</Text>
          <Pressable
            style={styles.filterRow}
            onPress={() => setUnearnedOnly((v) => !v)}
          >
            <View style={[styles.checkbox, unearnedOnly && styles.checkboxChecked]}>
              {unearnedOnly && <Text style={styles.checkmark}>✓</Text>}
            </View>
            <Text style={styles.filterLabel}>획득 가능만 보기</Text>
          </Pressable>
        </View>
      }
      ListEmptyComponent={
        <View style={styles.empty}>
          <Text style={styles.emptyText}>
            {unearnedOnly ? "모든 포인트를 획득했어요!" : "소식이 없습니다."}
          </Text>
        </View>
      }
      renderSectionHeader={({ section }) => {
        if (section.key === "__divider__") {
          return (
            <View style={styles.dividerBox}>
              <Text style={styles.dividerTitle}>🌱 시야를 넓힐 기회에요</Text>
              <Text style={styles.dividerBody}>
                구독하지 않은 주제예요. 읽으면 더 많은 포인트를 얻어요!
              </Text>
            </View>
          );
        }
        return (
          <View style={styles.sectionHeader}>
            <Text style={styles.sectionTitle}>
              {section.emoji} {section.label}
            </Text>
          </View>
        );
      }}
      renderItem={({ item }) => (
        <TopicCard
          topic={item}
          earnStatus={earnMap[item.id]}
          brainCategoryMap={brainCategoryMap}
          onPress={() =>
            (navigation.navigate as any)("TopicDetail", { id: item.id })
          }
        />
      )}
      ListFooterComponent={
        <View style={styles.footer}>
          <Pressable
            style={styles.allNewsButton}
            onPress={() => navigation.navigate("AllNews" as never)}
          >
            <Text style={styles.allNewsButtonText}>모든 소식 보러가기</Text>
          </Pressable>
        </View>
      }
      stickySectionHeadersEnabled={false}
    />
  );
}

const styles = StyleSheet.create({
  list: {
    flex: 1,
    backgroundColor: "#fdf9ee",
  },
  content: {
    paddingHorizontal: 16,
    paddingBottom: 32,
  },
  centered: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "#fdf9ee",
  },
  header: {
    paddingTop: 20,
    paddingBottom: 16,
  },
  title: {
    fontSize: 20,
    fontWeight: "700",
    color: "#231815",
  },
  subtitle: {
    fontSize: 12,
    color: "rgba(35,24,21,0.50)",
    marginTop: 4,
  },
  filterRow: {
    flexDirection: "row",
    alignItems: "center",
    marginTop: 12,
    gap: 8,
  },
  checkbox: {
    width: 18,
    height: 18,
    borderRadius: 4,
    borderWidth: 1.5,
    borderColor: "#231815",
    alignItems: "center",
    justifyContent: "center",
  },
  checkboxChecked: {
    backgroundColor: "#43b9d6",
    borderColor: "#43b9d6",
  },
  checkmark: {
    color: "#fff",
    fontSize: 11,
    fontWeight: "700",
  },
  filterLabel: {
    fontSize: 14,
    color: "rgba(35,24,21,0.70)",
  },
  empty: {
    paddingVertical: 60,
    alignItems: "center",
  },
  emptyText: {
    fontSize: 14,
    color: "rgba(35,24,21,0.50)",
  },
  sectionHeader: {
    borderBottomWidth: 1,
    borderBottomColor: "#dbdade",
    paddingBottom: 8,
    marginTop: 24,
    marginBottom: 12,
  },
  sectionTitle: {
    fontSize: 15,
    fontWeight: "600",
    color: "#231815",
  },
  dividerBox: {
    borderRadius: 8,
    borderWidth: 1,
    borderColor: "#43b9d6",
    backgroundColor: "#fff",
    paddingHorizontal: 16,
    paddingVertical: 12,
    marginTop: 24,
    marginBottom: 16,
  },
  dividerTitle: {
    fontSize: 14,
    fontWeight: "700",
    color: "#43b9d6",
  },
  dividerBody: {
    fontSize: 12,
    color: "#231815",
    marginTop: 2,
  },
  footer: {
    alignItems: "center",
    paddingTop: 32,
  },
  allNewsButton: {
    paddingHorizontal: 28,
    paddingVertical: 12,
    borderRadius: 50,
    borderWidth: 2,
    borderColor: "#231815",
    backgroundColor: "#43b9d6",
  },
  allNewsButtonText: {
    fontSize: 14,
    fontWeight: "700",
    color: "#231815",
  },
});
