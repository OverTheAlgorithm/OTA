// Extracted from: web/src/components/history-section.tsx
// Differences: FlatList for infinite scroll instead of IntersectionObserver,
//              useNavigation instead of Link, no window.scrollTo

import { useCallback, useEffect, useRef, useState } from "react";
import {
  View,
  Text,
  Pressable,
  FlatList,
  StyleSheet,
  ActivityIndicator,
} from "react-native";
import { useNavigation } from "@react-navigation/native";
import type { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { api } from "../lib/api";
import type { HistoryEntry, HistoryItem, BrainCategory } from "../lib/api";
import { formatDate } from "../../../packages/shared/src/utils";

type RootStackParamList = {
  TopicDetail: { id: string };
  [key: string]: object | undefined;
};

const PAGE_SIZE = 10;

interface Props {
  subscriptions: string[];
  onFirstLoad?: () => void;
}

function BuzzBadge({ score }: { score: number }) {
  if (!score) return null;
  return (
    <Text style={styles.buzzBadge}>🔥 화제도 {score}</Text>
  );
}

function TopicRow({
  item,
  accent,
}: {
  item: HistoryItem;
  accent?: string;
}) {
  const navigation = useNavigation<NativeStackNavigationProp<RootStackParamList>>();
  const dotColor = accent ?? "#6b8db5";
  const hasDetails = item.details && item.details.length > 0;

  return (
    <View style={styles.topicRow}>
      <View style={[styles.topicDot, { backgroundColor: dotColor + "99" }]} />
      <View style={styles.topicContent}>
        <BuzzBadge score={item.buzz_score} />
        <Text style={styles.topicTitle}>{item.topic}</Text>
        <Text style={styles.topicSummary}>{item.summary}</Text>
        {hasDetails && (
          <Pressable
            onPress={() => navigation.navigate("TopicDetail", { id: item.id })}
          >
            <Text style={styles.topicLink}>
              {item.details.length}개의 추가 정보가 있어요 →
            </Text>
          </Pressable>
        )}
      </View>
    </View>
  );
}

function groupByBrainCategory(items: HistoryItem[]): Record<string, HistoryItem[]> {
  const groups: Record<string, HistoryItem[]> = {};
  for (const item of items) {
    const key = item.brain_category || "";
    if (!groups[key]) groups[key] = [];
    groups[key].push(item);
  }
  return groups;
}

function HistoryCard({
  entry,
  subscriptions,
  brainCategories,
  defaultOpen,
}: {
  entry: HistoryEntry;
  subscriptions: string[];
  brainCategories: BrainCategory[];
  defaultOpen: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);

  const subSet = new Set(subscriptions);
  const selectedItems = entry.items.filter(
    (i) => i.priority === "top" || i.priority === "brief" || subSet.has(i.category),
  );

  const bcGroups = groupByBrainCategory(selectedItems);

  return (
    <View style={styles.historyCard}>
      <Pressable
        onPress={() => setOpen((o) => !o)}
        style={[styles.historyCardHeader, open && styles.historyCardHeaderOpen]}
      >
        <Text style={styles.historyCardDate}>{formatDate(entry.date)}</Text>
        <View style={styles.historyCardMeta}>
          <View style={styles.countBadge}>
            <Text style={styles.countBadgeText}>{selectedItems.length}개 토픽</Text>
          </View>
          <Text style={styles.chevron}>{open ? "▲" : "▼"}</Text>
        </View>
      </Pressable>

      {open && (
        <View style={styles.historyCardBody}>
          {brainCategories.map((bc) => {
            const items = bcGroups[bc.key];
            if (!items || items.length === 0) return null;
            return (
              <View key={bc.key} style={styles.bcGroup}>
                <View style={styles.bcGroupHeader}>
                  <View
                    style={[
                      styles.bcIconBox,
                      { backgroundColor: bc.accent_color + "15" },
                    ]}
                  >
                    <Text style={styles.bcEmoji}>{bc.emoji}</Text>
                  </View>
                  <Text style={[styles.bcLabel, { color: bc.accent_color }]}>
                    {bc.label}
                  </Text>
                </View>
                {items.map((item, i) => (
                  <TopicRow key={i} item={item} accent={bc.accent_color} />
                ))}
              </View>
            );
          })}

          {bcGroups[""] && bcGroups[""].length > 0 && (
            <View style={styles.bcGroup}>
              <View style={styles.bcGroupHeader}>
                <View style={[styles.bcIconBox, { backgroundColor: "rgba(107,141,181,0.1)" }]}>
                  <Text style={styles.bcEmoji}>📌</Text>
                </View>
                <Text style={[styles.bcLabel, { color: "#6b8db5" }]}>기타</Text>
              </View>
              {bcGroups[""].map((item, i) => (
                <TopicRow key={i} item={item} accent="#9b8bb4" />
              ))}
            </View>
          )}
        </View>
      )}
    </View>
  );
}

export function HistorySection({ subscriptions, onFirstLoad }: Props) {
  const [entries, setEntries] = useState<HistoryEntry[]>([]);
  const [brainCategories, setBrainCategories] = useState<BrainCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [offset, setOffset] = useState(0);
  const loadingMoreRef = useRef(false);

  useEffect(() => {
    api.getBrainCategories().then(setBrainCategories).catch(() => {});
    api.getContextHistory(PAGE_SIZE, 0)
      .then(({ data, has_more }) => {
        setEntries(data);
        setHasMore(has_more);
        setOffset(data.length);
      })
      .catch(() => {})
      .finally(() => {
        setLoading(false);
        onFirstLoad?.();
      });
  }, []);

  const loadMore = useCallback(async () => {
    if (loadingMoreRef.current || !hasMore) return;
    loadingMoreRef.current = true;
    setLoadingMore(true);
    try {
      const { data, has_more } = await api.getContextHistory(PAGE_SIZE, offset);
      setEntries((prev) => [...prev, ...data]);
      setHasMore(has_more);
      setOffset((prev) => prev + data.length);
    } catch {
      // silently fail — user can scroll again to retry
    } finally {
      loadingMoreRef.current = false;
      setLoadingMore(false);
    }
  }, [hasMore, offset]);

  const renderItem = ({ item, index }: { item: HistoryEntry; index: number }) => (
    <HistoryCard
      entry={item}
      subscriptions={subscriptions}
      brainCategories={brainCategories}
      defaultOpen={index === 0}
    />
  );

  const renderFooter = () => {
    if (loadingMore) {
      return (
        <View style={styles.loadingMoreRow}>
          <ActivityIndicator size="small" color="#6b8db5" />
          <Text style={styles.loadingMoreText}>이전 소식을 불러오는 중...</Text>
        </View>
      );
    }
    if (!hasMore && entries.length > 0) {
      return (
        <Text style={styles.allLoadedText}>모든 소식을 불러왔습니다</Text>
      );
    }
    return null;
  };

  return (
    <View style={styles.section}>
      <View style={styles.sectionHeader}>
        <View style={styles.sectionIconBox}>
          <Text style={styles.sectionIcon}>📄</Text>
        </View>
        <Text style={styles.sectionTitle}>받아본 소식</Text>
      </View>

      {loading ? (
        <View style={styles.skeletonList}>
          {[1, 2].map((i) => (
            <View key={i} style={styles.skeletonCard}>
              <View style={styles.skeletonLine} />
              <View style={[styles.skeletonLine, styles.skeletonLineShort]} />
            </View>
          ))}
        </View>
      ) : entries.length === 0 ? (
        <View style={styles.emptyCard}>
          <Text style={styles.emptyText}>아직 받은 소식이 없습니다.</Text>
          <Text style={styles.emptySubtext}>매일 아침 7시에 첫 브리핑이 전달됩니다.</Text>
        </View>
      ) : (
        <FlatList
          data={entries}
          keyExtractor={(item) => item.date}
          renderItem={renderItem}
          onEndReached={loadMore}
          onEndReachedThreshold={0.5}
          ListFooterComponent={renderFooter}
          scrollEnabled={false}
          ItemSeparatorComponent={() => <View style={styles.separator} />}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  section: {
    gap: 12,
  },
  sectionHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    marginBottom: 4,
  },
  sectionIconBox: {
    width: 32,
    height: 32,
    borderRadius: 8,
    backgroundColor: "rgba(74,159,229,0.1)",
    justifyContent: "center",
    alignItems: "center",
  },
  sectionIcon: {
    fontSize: 16,
  },
  sectionTitle: {
    fontSize: 16,
    fontWeight: "600",
    color: "#1e3a5f",
  },
  historyCard: {
    borderRadius: 16,
    backgroundColor: "#f0f7ff",
    borderWidth: 1,
    borderColor: "#d4e6f5",
    overflow: "hidden",
  },
  historyCardHeader: {
    paddingHorizontal: 16,
    paddingVertical: 16,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  historyCardHeaderOpen: {
    borderBottomWidth: 1,
    borderBottomColor: "#d4e6f5",
  },
  historyCardDate: {
    fontSize: 16,
    fontWeight: "600",
    color: "#1e3a5f",
  },
  historyCardMeta: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  countBadge: {
    backgroundColor: "#fff",
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: 99,
    borderWidth: 1,
    borderColor: "#d4e6f5",
  },
  countBadgeText: {
    fontSize: 12,
    color: "#6b8db5",
  },
  chevron: {
    fontSize: 12,
    color: "#6b8db5",
  },
  historyCardBody: {
    padding: 16,
    gap: 20,
  },
  bcGroup: {
    gap: 4,
  },
  bcGroupHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    marginBottom: 8,
  },
  bcIconBox: {
    width: 24,
    height: 24,
    borderRadius: 6,
    justifyContent: "center",
    alignItems: "center",
  },
  bcEmoji: {
    fontSize: 14,
  },
  bcLabel: {
    fontSize: 12,
    fontWeight: "600",
    letterSpacing: 0.5,
  },
  topicRow: {
    flexDirection: "row",
    gap: 12,
    paddingVertical: 10,
    borderBottomWidth: 1,
    borderBottomColor: "rgba(212,230,245,0.6)",
  },
  topicDot: {
    width: 6,
    height: 6,
    borderRadius: 3,
    marginTop: 8,
    flexShrink: 0,
  },
  topicContent: {
    flex: 1,
    gap: 2,
  },
  buzzBadge: {
    fontSize: 12,
    fontWeight: "700",
    color: "#ff5442",
  },
  topicTitle: {
    fontSize: 14,
    fontWeight: "600",
    color: "#1e3a5f",
    lineHeight: 20,
  },
  topicSummary: {
    fontSize: 12,
    color: "#6b8db5",
    marginTop: 4,
    lineHeight: 18,
  },
  topicLink: {
    fontSize: 12,
    color: "#6b8db5",
    marginTop: 4,
  },
  separator: {
    height: 12,
  },
  loadingMoreRow: {
    flexDirection: "row",
    justifyContent: "center",
    alignItems: "center",
    gap: 8,
    paddingVertical: 16,
  },
  loadingMoreText: {
    fontSize: 14,
    color: "#6b8db5",
  },
  allLoadedText: {
    fontSize: 12,
    color: "rgba(107,141,181,0.6)",
    textAlign: "center",
    paddingVertical: 8,
  },
  skeletonList: {
    gap: 16,
  },
  skeletonCard: {
    borderRadius: 16,
    backgroundColor: "#f0f7ff",
    borderWidth: 1,
    borderColor: "#d4e6f5",
    padding: 24,
    gap: 8,
  },
  skeletonLine: {
    height: 12,
    backgroundColor: "#d4e6f5",
    borderRadius: 6,
    width: "100%",
  },
  skeletonLineShort: {
    width: "75%",
  },
  emptyCard: {
    borderRadius: 16,
    backgroundColor: "#f0f7ff",
    borderWidth: 1,
    borderColor: "#d4e6f5",
    padding: 48,
    alignItems: "center",
    gap: 4,
  },
  emptyText: {
    fontSize: 14,
    color: "#6b8db5",
  },
  emptySubtext: {
    fontSize: 12,
    color: "rgba(107,141,181,0.6)",
  },
});
