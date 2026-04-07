import React from "react";
import { View, Text, Image, Pressable, StyleSheet } from "react-native";
import { TopicPreview, EarnStatusItem } from "../../../packages/shared/src/types";
import { formatDate } from "../../../packages/shared/src/utils";

const DEFAULT_IMAGE = "https://server.mindhacker.club/static/default.png";

interface TopicCardProps {
  topic: TopicPreview;
  earnStatus?: EarnStatusItem;
  brainCategoryMap?: Record<string, { emoji: string; label: string; accent_color?: string }>;
  onPress: () => void;
}

function CoinStatusTag({ status }: { status: EarnStatusItem }) {
  let label: string;
  let color: string;
  let bgColor: string;

  switch (status.status) {
    case "PENDING":
      label = "획득 가능";
      color = "#43b9d6";
      bgColor = "rgba(67,185,214,0.15)";
      break;
    case "DUPLICATE":
      label = "획득 완료";
      color = "#888";
      bgColor = "#e8e8e8";
      break;
    case "EXPIRED":
      label = "기간 경과";
      color = "#888";
      bgColor = "#e8e8e8";
      break;
    case "DAILY_LIMIT":
      label = "일일 한도";
      color = "#888";
      bgColor = "#e8e8e8";
      break;
    default:
      return null;
  }

  return (
    <View style={[styles.tag, { backgroundColor: bgColor }]}>
      <Text style={[styles.tagText, { color }]}>{label}</Text>
    </View>
  );
}

export function TopicCard({ topic, earnStatus, brainCategoryMap, onPress }: TopicCardProps) {
  const brainCat = topic.brain_category && brainCategoryMap
    ? brainCategoryMap[topic.brain_category]
    : undefined;

  const accentColor = (brainCat as { accent_color?: string } | undefined)?.accent_color ?? "#43b9d6";

  return (
    <Pressable style={styles.card} onPress={onPress} android_ripple={{ color: "#00000010" }}>
      <Image
        source={{ uri: topic.image_url ?? DEFAULT_IMAGE }}
        style={styles.image}
        defaultSource={require("../../assets/icon.png")}
      />
      <View style={styles.content}>
        <View style={styles.metaRow}>
          {topic.created_at && (
            <Text style={styles.date}>{formatDate(topic.created_at)}</Text>
          )}
          {brainCat && (
            <View style={[styles.tag, { backgroundColor: `${accentColor}20` }]}>
              <Text style={[styles.tagText, { color: accentColor }]}>
                {brainCat.emoji} {brainCat.label}
              </Text>
            </View>
          )}
          {earnStatus && <CoinStatusTag status={earnStatus} />}
        </View>
        <Text style={styles.title} numberOfLines={2}>
          {topic.topic}
        </Text>
        <Text style={styles.summary} numberOfLines={2}>
          {topic.summary}
        </Text>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: {
    flexDirection: "row",
    backgroundColor: "#fff",
    borderRadius: 10,
    borderWidth: 1,
    borderColor: "#231815",
    overflow: "hidden",
    marginBottom: 12,
  },
  image: {
    width: 100,
    height: 90,
    backgroundColor: "#f0ece0",
  },
  content: {
    flex: 1,
    padding: 10,
    justifyContent: "center",
  },
  metaRow: {
    flexDirection: "row",
    flexWrap: "wrap",
    alignItems: "center",
    gap: 6,
    marginBottom: 4,
  },
  date: {
    fontSize: 11,
    fontWeight: "700",
    color: "#231815",
  },
  tag: {
    borderRadius: 20,
    paddingHorizontal: 8,
    paddingVertical: 2,
  },
  tagText: {
    fontSize: 10,
    fontWeight: "700",
  },
  title: {
    fontSize: 14,
    fontWeight: "700",
    color: "#231815",
    lineHeight: 20,
    marginBottom: 4,
  },
  summary: {
    fontSize: 12,
    color: "rgba(35,24,21,0.65)",
    lineHeight: 17,
  },
});
