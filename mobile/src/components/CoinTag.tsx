// Extracted from: web/src/components/coin-tag.tsx
// Differences: React Native View/Text instead of span

import { View, Text, StyleSheet } from "react-native";
import type { EarnStatusItem } from "../lib/api";

export function CoinTag({ status }: { status: EarnStatusItem }) {
  const earnTag = renderEarnTag(status);
  const quizTag = renderQuizTag(status);

  if (!earnTag && !quizTag) return null;

  return (
    <View style={styles.container}>
      {earnTag}
      {quizTag}
    </View>
  );
}

function renderEarnTag(status: EarnStatusItem) {
  if (status.status === "DUPLICATE") {
    return (
      <View style={[styles.tag, styles.tagGray]}>
        <Text style={[styles.tagText, styles.tagTextGray]}>획득!</Text>
      </View>
    );
  }
  if (status.status === "EXPIRED") {
    return (
      <View style={[styles.tag, styles.tagGray]}>
        <Text style={[styles.tagText, styles.tagTextGray]}>획득 기간 경과</Text>
      </View>
    );
  }
  if (status.status === "DAILY_LIMIT") {
    return (
      <View style={[styles.tag, styles.tagGray]}>
        <Text style={[styles.tagText, styles.tagTextGray]}>일일 한도</Text>
      </View>
    );
  }
  if (status.status === "PENDING" && status.coins > 0) {
    return (
      <View style={[styles.tag, styles.tagBlue]}>
        <Text style={[styles.tagText, styles.tagTextBlue]}>+{status.coins}포인트</Text>
      </View>
    );
  }
  return null;
}

function renderQuizTag(status: EarnStatusItem) {
  if (status.status === "DAILY_LIMIT" || status.status === "EXPIRED") return null;
  if (!status.has_quiz) return null;

  if (status.status === "PENDING") {
    return (
      <View style={[styles.tag, styles.tagOrange]}>
        <Text style={[styles.tagText, styles.tagTextOrange]}>보너스 퀴즈</Text>
      </View>
    );
  }

  if (status.status === "DUPLICATE") {
    if (status.quiz_completed) {
      return (
        <View style={[styles.tag, styles.tagOrangeFaded]}>
          <Text style={[styles.tagText, styles.tagTextOrangeFaded]}>퀴즈 풀이 완료</Text>
        </View>
      );
    }
    return (
      <View style={[styles.tag, styles.tagOrange]}>
        <Text style={[styles.tagText, styles.tagTextOrange]}>보너스 퀴즈</Text>
      </View>
    );
  }

  return null;
}

const styles = StyleSheet.create({
  container: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    flexShrink: 0,
  },
  tag: {
    paddingHorizontal: 10,
    paddingVertical: 2,
    borderRadius: 99,
  },
  tagText: {
    fontSize: 10,
    fontWeight: "700",
  },
  tagGray: {
    backgroundColor: "rgba(35,24,21,0.1)",
  },
  tagTextGray: {
    color: "rgba(35,24,21,0.5)",
  },
  tagBlue: {
    backgroundColor: "rgba(67,185,214,0.15)",
  },
  tagTextBlue: {
    color: "#43b9d6",
  },
  tagOrange: {
    backgroundColor: "rgba(245,166,35,0.15)",
  },
  tagTextOrange: {
    color: "#f5a623",
  },
  tagOrangeFaded: {
    backgroundColor: "rgba(245,166,35,0.1)",
  },
  tagTextOrangeFaded: {
    color: "rgba(245,166,35,0.7)",
  },
});
