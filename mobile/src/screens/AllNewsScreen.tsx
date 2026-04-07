// Ported from: web/src/pages/allnews.tsx
import React, { useCallback, useEffect, useRef, useState } from "react";
import {
  View,
  Text,
  FlatList,
  ScrollView,
  StyleSheet,
  Pressable,
  ActivityIndicator,
  Image,
} from "react-native";
import { useNavigation } from "@react-navigation/native";
import { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { api } from "../lib/api";
import { TopicPreview, FilterOptions, FilterType, EarnStatusItem } from "../../../packages/shared/src/types";
import { formatDate } from "../../../packages/shared/src/utils";

type NavProp = NativeStackNavigationProp<Record<string, object | undefined>>;

const PAGE_SIZE = 12;
const DEFAULT_IMAGE = "https://server.mindhacker.club/static/default.png";

interface ActiveFilter {
  type: FilterType;
  value: string;
}

interface Tab {
  label: string;
  type: FilterType;
  value: string;
  emoji?: string;
}

function NewsCard({
  topic,
  earnStatus,
  brainCategoryMap,
  onPress,
}: {
  topic: TopicPreview;
  earnStatus?: EarnStatusItem;
  brainCategoryMap: Record<string, { emoji: string; label: string }>;
  onPress: () => void;
}) {
  const brainCat = topic.brain_category ? brainCategoryMap[topic.brain_category] : undefined;

  return (
    <Pressable style={styles.card} onPress={onPress} android_ripple={{ color: "#00000010" }}>
      <Image
        source={{ uri: topic.image_url ?? DEFAULT_IMAGE }}
        style={styles.cardImage}
        resizeMode="cover"
      />
      <View style={styles.cardMeta}>
        {topic.created_at && (
          <Text style={styles.cardDate}>{formatDate(topic.created_at)}</Text>
        )}
        {earnStatus && earnStatus.status === "PENDING" && (
          <View style={styles.coinTag}>
            <Text style={styles.coinTagText}>획득 가능</Text>
          </View>
        )}
      </View>
      {brainCat && (
        <Text style={styles.cardCategory}>
          {brainCat.emoji} {brainCat.label}
        </Text>
      )}
      <Text style={styles.cardTitle} numberOfLines={2}>
        {topic.topic}
      </Text>
      <Text style={styles.cardSummary} numberOfLines={2}>
        {topic.summary}
      </Text>
    </Pressable>
  );
}

export function AllNewsScreen() {
  const navigation = useNavigation<NavProp>();

  const [filterOptions, setFilterOptions] = useState<FilterOptions>({
    categories: [],
    brain_categories: [],
  });
  const [activeFilter, setActiveFilter] = useState<ActiveFilter>({ type: "", value: "" });
  const [topics, setTopics] = useState<TopicPreview[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [earnMap, setEarnMap] = useState<Record<string, EarnStatusItem>>({});
  const offsetRef = useRef(0);

  const brainCategoryMap: Record<string, { emoji: string; label: string }> = {};
  for (const bc of filterOptions.brain_categories) {
    brainCategoryMap[bc.key] = { emoji: bc.emoji, label: bc.label };
  }

  useEffect(() => {
    api.fetchFilterOptions().then(setFilterOptions).catch(() => {});
  }, []);

  const loadTopics = useCallback(
    async (filter: ActiveFilter, append = false) => {
      if (!append) setLoading(true);
      else setLoadingMore(true);

      const offset = append ? offsetRef.current : 0;
      try {
        const { data, has_more } = await api.fetchAllTopics(
          filter.type,
          filter.value,
          PAGE_SIZE,
          offset,
        );
        if (append) {
          setTopics((prev) => [...prev, ...data]);
        } else {
          setTopics(data);
        }
        setHasMore(has_more);
        offsetRef.current = offset + data.length;

        if (data.length > 0) {
          const ids = data.map((t) => t.id);
          api.batchEarnStatus(ids)
            .then((statuses) => {
              setEarnMap((prev) => {
                const next = { ...prev };
                for (const s of statuses) next[s.id] = s;
                return next;
              });
            })
            .catch(() => {});
        }
      } catch {
        if (!append) setTopics([]);
      } finally {
        setLoading(false);
        setLoadingMore(false);
      }
    },
    [],
  );

  useEffect(() => {
    offsetRef.current = 0;
    loadTopics(activeFilter);
  }, [activeFilter, loadTopics]);

  const handleFilterChange = (type: FilterType, value: string) => {
    const next: ActiveFilter =
      activeFilter.type === type && activeFilter.value === value
        ? { type: "", value: "" }
        : { type, value };
    setActiveFilter(next);
  };

  const handleLoadMore = () => {
    if (!loadingMore && hasMore) {
      loadTopics(activeFilter, true);
    }
  };

  const tabs: Tab[] = [
    { label: "전체", type: "", value: "" },
    ...filterOptions.categories.map((c) => ({
      label: c.label,
      type: "category" as FilterType,
      value: c.key,
    })),
    ...filterOptions.brain_categories.map((bc) => ({
      label: bc.label,
      type: "brain_category" as FilterType,
      value: bc.key,
      emoji: bc.emoji,
    })),
  ];

  const isActiveTab = (type: FilterType, value: string) =>
    activeFilter.type === type && activeFilter.value === value;

  return (
    <View style={styles.screen}>
      {/* Filter tabs */}
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        style={styles.tabsContainer}
        contentContainerStyle={styles.tabsContent}
      >
        {tabs.map((tab) => {
          const active = isActiveTab(tab.type, tab.value);
          return (
            <Pressable
              key={`${tab.type}-${tab.value}`}
              style={[styles.tab, active && styles.tabActive]}
              onPress={() => handleFilterChange(tab.type, tab.value)}
            >
              <Text style={[styles.tabText, active && styles.tabTextActive]}>
                {tab.emoji ? `${tab.emoji} ${tab.label}` : tab.label}
              </Text>
              {active && <View style={styles.tabUnderline} />}
            </Pressable>
          );
        })}
      </ScrollView>

      {/* Topic list */}
      {loading ? (
        <View style={styles.centered}>
          <ActivityIndicator size="large" color="#43b9d6" />
        </View>
      ) : topics.length === 0 ? (
        <View style={styles.centered}>
          <Text style={styles.emptyText}>소식이 없습니다.</Text>
        </View>
      ) : (
        <FlatList
          data={topics}
          keyExtractor={(item) => item.id}
          numColumns={2}
          contentContainerStyle={styles.gridContent}
          columnWrapperStyle={styles.gridRow}
          onEndReached={handleLoadMore}
          onEndReachedThreshold={0.3}
          renderItem={({ item }) => (
            <NewsCard
              topic={item}
              earnStatus={earnMap[item.id]}
              brainCategoryMap={brainCategoryMap}
              onPress={() =>
                (navigation.navigate as any)("TopicDetail", { id: item.id })
              }
            />
          )}
          ListFooterComponent={
            loadingMore ? (
              <ActivityIndicator size="small" color="#43b9d6" style={{ marginVertical: 16 }} />
            ) : !hasMore && topics.length > 0 ? (
              <Text style={styles.endText}>모든 소식을 불러왔습니다</Text>
            ) : null
          }
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: "#fdf9ee",
  },
  tabsContainer: {
    flexGrow: 0,
    borderBottomWidth: 1,
    borderBottomColor: "#dbdade",
  },
  tabsContent: {
    paddingHorizontal: 16,
    paddingTop: 4,
  },
  tab: {
    paddingHorizontal: 4,
    paddingBottom: 10,
    marginRight: 16,
    position: "relative",
  },
  tabActive: {},
  tabText: {
    fontSize: 15,
    fontWeight: "500",
    color: "rgba(35,24,21,0.60)",
  },
  tabTextActive: {
    color: "#008fb2",
  },
  tabUnderline: {
    position: "absolute",
    bottom: 0,
    left: 0,
    right: 0,
    height: 3,
    backgroundColor: "#008fb2",
    borderRadius: 2,
  },
  centered: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
  },
  emptyText: {
    fontSize: 14,
    color: "rgba(35,24,21,0.50)",
  },
  gridContent: {
    padding: 12,
    paddingBottom: 32,
  },
  gridRow: {
    justifyContent: "space-between",
  },
  card: {
    width: "48%",
    marginBottom: 20,
  },
  cardImage: {
    width: "100%",
    aspectRatio: 16 / 10,
    borderRadius: 10,
    backgroundColor: "#f0ece0",
    marginBottom: 8,
  },
  cardMeta: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    marginBottom: 4,
  },
  cardDate: {
    fontSize: 12,
    fontWeight: "500",
    color: "#231815",
  },
  coinTag: {
    borderRadius: 20,
    paddingHorizontal: 6,
    paddingVertical: 2,
    backgroundColor: "rgba(67,185,214,0.15)",
  },
  coinTagText: {
    fontSize: 10,
    fontWeight: "700",
    color: "#43b9d6",
  },
  cardCategory: {
    fontSize: 11,
    color: "rgba(35,24,21,0.50)",
    marginBottom: 4,
  },
  cardTitle: {
    fontSize: 13,
    fontWeight: "700",
    color: "#231815",
    lineHeight: 18,
    marginBottom: 4,
  },
  cardSummary: {
    fontSize: 11,
    color: "rgba(35,24,21,0.50)",
    lineHeight: 16,
  },
  endText: {
    textAlign: "center",
    fontSize: 12,
    color: "rgba(35,24,21,0.40)",
    paddingVertical: 16,
  },
});
