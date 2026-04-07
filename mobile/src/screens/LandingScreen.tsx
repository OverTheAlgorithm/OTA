// Extracted from: web/src/pages/landing.tsx — ported to React Native
// Shows recent topics in a FlatList with WizLetter branding and a login CTA.
// Web-specific features omitted: IntersectionObserver fade-ins, scroll restoration,
// hero image sizing sync, LoginModal (Kakao OAuth handled natively).

import { useEffect, useState, useCallback } from "react";
import {
  View,
  Text,
  FlatList,
  TouchableOpacity,
  StyleSheet,
  Image,
  SafeAreaView,
  ActivityIndicator,
  ListRenderItem,
} from "react-native";
import type { NativeStackScreenProps } from "@react-navigation/native-stack";
import type { RootStackParamList } from "../navigation";
import { useAuth } from "../contexts/auth-context";
import { api, defaultImage } from "../lib/api";
import type { TopicPreview } from "../../../packages/shared/src/types";

type Props = NativeStackScreenProps<RootStackParamList, "Landing">;

export function LandingScreen({ navigation }: Props) {
  const { user, logout } = useAuth();
  const [recentTopics, setRecentTopics] = useState<TopicPreview[]>([]);
  const [topicsLoading, setTopicsLoading] = useState(true);

  // Redirect logged-in users to Latest (mirrors web behavior)
  useEffect(() => {
    if (user) {
      navigation.replace("Latest");
    }
  }, [user, navigation]);

  useEffect(() => {
    api
      .fetchRecentTopics()
      .then(setRecentTopics)
      .catch(() => setRecentTopics([]))
      .finally(() => setTopicsLoading(false));
  }, []);

  const handleStart = useCallback(() => {
    // Kakao login via native SDK will be wired up separately
    // For now, navigate to Latest as a placeholder
    navigation.navigate("Latest");
  }, [navigation]);

  const renderTopic: ListRenderItem<TopicPreview> = useCallback(
    ({ item }) => (
      <TouchableOpacity
        style={styles.topicCard}
        onPress={() => navigation.navigate("TopicDetail", { id: item.id })}
        activeOpacity={0.8}
      >
        <Image
          source={{ uri: item.image_url ?? defaultImage }}
          style={styles.topicImage}
          resizeMode="cover"
        />
        <View style={styles.topicContent}>
          <Text style={styles.topicTitle} numberOfLines={2}>
            {item.topic}
          </Text>
          <Text style={styles.topicSummary} numberOfLines={3}>
            {item.summary}
          </Text>
          <Text style={styles.topicReadMore}>자세히 보기 →</Text>
        </View>
      </TouchableOpacity>
    ),
    [navigation],
  );

  const ListHeader = (
    <View>
      {/* Header */}
      <View style={styles.header}>
        <Text style={styles.logo}>WizLetter</Text>
        {user ? (
          <TouchableOpacity onPress={logout}>
            <Text style={styles.headerAction}>로그아웃</Text>
          </TouchableOpacity>
        ) : (
          <TouchableOpacity onPress={handleStart}>
            <Text style={styles.headerAction}>시작하기</Text>
          </TouchableOpacity>
        )}
      </View>

      {/* Hero */}
      <View style={styles.hero}>
        <Text style={styles.heroTitle}>
          개인화에 갇힌 알고리즘 너머{"\n"}진짜 세상을 읽고 수익까지
        </Text>
        <Text style={styles.heroSubtitle}>
          세상이 돌아가는 이야기를 빠르게 파악하세요.{"\n"}
          내가 관심 없는 주제를 읽으면 더 많은 포인트를 얻을 수 있어요.
        </Text>
        <TouchableOpacity style={styles.ctaButton} onPress={handleStart} activeOpacity={0.85}>
          <Text style={styles.ctaText}>카카오로 시작하기</Text>
        </TouchableOpacity>
      </View>

      {/* Recent topics heading */}
      {recentTopics.length > 0 && (
        <View style={styles.sectionHeader}>
          <Text style={styles.sectionTitle}>최신 소식 바로 확인하기</Text>
        </View>
      )}

      {topicsLoading && (
        <View style={styles.loadingContainer}>
          <ActivityIndicator size="large" color="#43b9d6" />
        </View>
      )}
    </View>
  );

  const ListFooter = (
    <TouchableOpacity
      style={styles.moreButton}
      onPress={() => navigation.navigate("AllNews")}
      activeOpacity={0.85}
    >
      <Text style={styles.moreButtonText}>더 많은 소식 보기</Text>
    </TouchableOpacity>
  );

  return (
    <SafeAreaView style={styles.container}>
      <FlatList
        data={recentTopics}
        keyExtractor={(item) => item.id}
        renderItem={renderTopic}
        ListHeaderComponent={ListHeader}
        ListFooterComponent={recentTopics.length > 0 ? ListFooter : null}
        contentContainerStyle={styles.listContent}
      />
    </SafeAreaView>
  );
}

const BRAND_BG = "#fdf9ee";
const BRAND_TEXT = "#231815";
const BRAND_ACCENT = "#43b9d6";
const BORDER = "#231815";

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: BRAND_BG,
  },
  listContent: {
    paddingBottom: 32,
  },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: 20,
    paddingVertical: 14,
    borderBottomWidth: 3,
    borderBottomColor: BORDER,
    backgroundColor: BRAND_BG,
  },
  logo: {
    fontSize: 22,
    fontWeight: "700",
    color: BRAND_TEXT,
    letterSpacing: -0.5,
  },
  headerAction: {
    fontSize: 14,
    fontWeight: "600",
    color: BRAND_TEXT,
  },
  hero: {
    paddingHorizontal: 20,
    paddingTop: 32,
    paddingBottom: 36,
    backgroundColor: BRAND_BG,
  },
  heroTitle: {
    fontSize: 26,
    fontWeight: "700",
    color: BRAND_TEXT,
    lineHeight: 36,
    marginBottom: 16,
  },
  heroSubtitle: {
    fontSize: 16,
    fontWeight: "500",
    color: BRAND_TEXT + "cc",
    lineHeight: 24,
    marginBottom: 28,
  },
  ctaButton: {
    backgroundColor: BRAND_ACCENT,
    borderWidth: 2.5,
    borderColor: BORDER,
    borderRadius: 999,
    paddingVertical: 14,
    paddingHorizontal: 40,
    alignSelf: "flex-start",
  },
  ctaText: {
    fontSize: 16,
    fontWeight: "700",
    color: BRAND_TEXT,
  },
  sectionHeader: {
    paddingHorizontal: 20,
    paddingTop: 24,
    paddingBottom: 12,
    borderTopWidth: 3,
    borderTopColor: BORDER,
  },
  sectionTitle: {
    fontSize: 20,
    fontWeight: "700",
    color: BRAND_TEXT,
    textAlign: "center",
  },
  loadingContainer: {
    paddingVertical: 40,
    alignItems: "center",
  },
  topicCard: {
    borderBottomWidth: 3,
    borderBottomColor: BORDER,
    backgroundColor: BRAND_BG,
  },
  topicImage: {
    width: "100%",
    aspectRatio: 16 / 9,
    backgroundColor: "#f0ece0",
  },
  topicContent: {
    padding: 20,
  },
  topicTitle: {
    fontSize: 20,
    fontWeight: "600",
    color: BRAND_TEXT,
    lineHeight: 28,
    marginBottom: 8,
  },
  topicSummary: {
    fontSize: 14,
    fontWeight: "500",
    color: BRAND_TEXT + "cc",
    lineHeight: 22,
    marginBottom: 14,
  },
  topicReadMore: {
    fontSize: 13,
    fontWeight: "700",
    color: BRAND_ACCENT,
    alignSelf: "flex-end",
  },
  moreButton: {
    backgroundColor: BRAND_ACCENT,
    borderWidth: 2.5,
    borderColor: BORDER,
    borderRadius: 999,
    paddingVertical: 14,
    paddingHorizontal: 40,
    alignSelf: "center",
    marginTop: 24,
    marginBottom: 16,
  },
  moreButtonText: {
    fontSize: 16,
    fontWeight: "700",
    color: BRAND_TEXT,
  },
});
