// Ported from: web/src/components/quiz-card.tsx
import React, { useState } from "react";
import { View, Text, Pressable, StyleSheet, ActivityIndicator } from "react-native";
import { api } from "../lib/api";
import { QuizForUser, QuizSubmitResult } from "../../../packages/shared/src/types";

const OPTION_LABELS = ["①", "②", "③", "④"];

interface CompletedState {
  result: QuizSubmitResult;
  selectedIndex: number;
  options: string[];
}

interface QuizCardProps {
  quiz: QuizForUser | null;
  hasQuiz: boolean;
  earnDone: boolean;
  contextItemId: string;
  onCoinsEarned?: (newTotal: number) => void;
}

export function QuizCard({ quiz, earnDone, contextItemId, onCoinsEarned }: QuizCardProps) {
  const [submitting, setSubmitting] = useState(false);
  const [completed, setCompleted] = useState<CompletedState | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleSelect = async (index: number) => {
    if (submitting || !quiz) return;
    setSubmitting(true);
    setError(null);
    try {
      const result = await api.submitQuizAnswer(contextItemId, index);
      setCompleted({ result, selectedIndex: index, options: quiz.options });
      if (onCoinsEarned) onCoinsEarned(result.total_coins);
    } catch {
      setError("퀴즈 제출에 실패했습니다. 잠시 후 다시 시도해주세요.");
    } finally {
      setSubmitting(false);
    }
  };

  // Locked state: earn not done yet
  if (!earnDone) {
    return (
      <View style={[styles.container, styles.lockedContainer]}>
        <View style={styles.headerRow}>
          <Text style={styles.lockedTitle}>보너스 퀴즈</Text>
          <View style={styles.lockedBadge}>
            <Text style={styles.lockedBadgeText}>잠김</Text>
          </View>
        </View>
        <Text style={styles.lockedBody}>포인트를 획득하면 퀴즈를 풀 수 있어요</Text>
      </View>
    );
  }

  // Completed state
  if (completed) {
    const { result, selectedIndex, options } = completed;
    return (
      <View style={[styles.container, styles.completedContainer]}>
        <View style={styles.headerRow}>
          <Text style={styles.activeTitle}>보너스 퀴즈</Text>
          <View style={result.correct ? styles.correctBadge : styles.wrongBadge}>
            <Text style={result.correct ? styles.correctBadgeText : styles.wrongBadgeText}>
              {result.correct ? "정답!" : "오답"}
            </Text>
          </View>
        </View>

        {result.correct && result.coins_earned > 0 && (
          <View style={styles.bonusBox}>
            <Text style={styles.bonusText}>+{result.coins_earned} 보너스!</Text>
            <Text style={styles.bonusSubText}>포인트가 적립되었습니다</Text>
          </View>
        )}

        {!result.correct && (
          <View style={styles.wrongBox}>
            <Text style={styles.wrongBodyText}>아쉽지만 틀렸어요.</Text>
          </View>
        )}

        <View style={styles.optionsList}>
          {options.map((option, i) => {
            const isSelected = i === selectedIndex;
            let optStyle = styles.optionDefault;
            let textStyle = styles.optionTextDefault;
            if (isSelected && result.correct) {
              optStyle = styles.optionCorrect;
              textStyle = styles.optionTextCorrect;
            } else if (isSelected && !result.correct) {
              optStyle = styles.optionWrong;
              textStyle = styles.optionTextWrong;
            }
            return (
              <View key={i} style={[styles.optionBase, optStyle]}>
                <Text style={[styles.optionLabel, textStyle]}>{OPTION_LABELS[i]}</Text>
                <Text style={[styles.optionText, textStyle]}>{option}</Text>
              </View>
            );
          })}
        </View>
      </View>
    );
  }

  // Active state: earn done, quiz available
  if (!quiz) return null;

  return (
    <View style={[styles.container, styles.activeContainer]}>
      <View style={styles.headerRow}>
        <Text style={styles.activeTitle}>보너스 퀴즈</Text>
        <View style={styles.activeBadge}>
          <Text style={styles.activeBadgeText}>정답 맞히면 보너스 포인트!</Text>
        </View>
      </View>
      <Text style={styles.question}>{quiz.question}</Text>
      <View style={styles.optionsList}>
        {quiz.options.map((option, i) => (
          <Pressable
            key={i}
            style={({ pressed }) => [
              styles.optionBase,
              styles.optionButton,
              pressed && styles.optionButtonPressed,
              submitting && styles.optionDisabled,
            ]}
            onPress={() => handleSelect(i)}
            disabled={submitting}
          >
            <Text style={styles.optionButtonLabel}>{OPTION_LABELS[i]}</Text>
            <Text style={styles.optionButtonText}>{option}</Text>
          </Pressable>
        ))}
      </View>
      {error && <Text style={styles.errorText}>{error}</Text>}
      {submitting && <ActivityIndicator size="small" color="#f5a623" style={{ marginTop: 8 }} />}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    borderRadius: 16,
    paddingHorizontal: 20,
    paddingVertical: 16,
    marginBottom: 24,
  },
  lockedContainer: {
    borderWidth: 1,
    borderColor: "rgba(35,24,21,0.15)",
    backgroundColor: "rgba(35,24,21,0.03)",
  },
  completedContainer: {
    borderWidth: 2,
    borderColor: "rgba(35,24,21,0.20)",
    backgroundColor: "#fff",
  },
  activeContainer: {
    borderWidth: 2,
    borderColor: "#f5a623",
    backgroundColor: "#fffbf2",
  },
  headerRow: {
    flexDirection: "row",
    alignItems: "center",
    flexWrap: "wrap",
    gap: 8,
    marginBottom: 8,
  },
  lockedTitle: {
    fontSize: 16,
    fontWeight: "700",
    color: "rgba(35,24,21,0.30)",
  },
  lockedBadge: {
    borderRadius: 20,
    paddingHorizontal: 8,
    paddingVertical: 2,
    backgroundColor: "rgba(35,24,21,0.10)",
  },
  lockedBadgeText: {
    fontSize: 10,
    fontWeight: "700",
    color: "rgba(35,24,21,0.30)",
  },
  lockedBody: {
    fontSize: 14,
    color: "rgba(35,24,21,0.40)",
    lineHeight: 20,
  },
  activeTitle: {
    fontSize: 16,
    fontWeight: "700",
    color: "#231815",
  },
  activeBadge: {
    borderRadius: 20,
    paddingHorizontal: 8,
    paddingVertical: 2,
    backgroundColor: "rgba(245,166,35,0.15)",
  },
  activeBadgeText: {
    fontSize: 10,
    fontWeight: "700",
    color: "#f5a623",
  },
  correctBadge: {
    borderRadius: 20,
    paddingHorizontal: 8,
    paddingVertical: 2,
    backgroundColor: "#dcfce7",
  },
  correctBadgeText: {
    fontSize: 10,
    fontWeight: "700",
    color: "#15803d",
  },
  wrongBadge: {
    borderRadius: 20,
    paddingHorizontal: 8,
    paddingVertical: 2,
    backgroundColor: "#fee2e2",
  },
  wrongBadgeText: {
    fontSize: 10,
    fontWeight: "700",
    color: "#dc2626",
  },
  bonusBox: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 12,
    backgroundColor: "#f0fdf4",
    borderWidth: 1,
    borderColor: "#bbf7d0",
    marginBottom: 12,
  },
  bonusText: {
    fontSize: 14,
    fontWeight: "700",
    color: "#15803d",
  },
  bonusSubText: {
    fontSize: 12,
    color: "#16a34a",
  },
  wrongBox: {
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 12,
    backgroundColor: "#fff1f2",
    borderWidth: 1,
    borderColor: "#fecdd3",
    marginBottom: 12,
  },
  wrongBodyText: {
    fontSize: 14,
    color: "#dc2626",
  },
  question: {
    fontSize: 15,
    fontWeight: "600",
    color: "#231815",
    lineHeight: 22,
    marginBottom: 14,
  },
  optionsList: {
    gap: 8,
  },
  optionBase: {
    flexDirection: "row",
    alignItems: "center",
    paddingHorizontal: 16,
    paddingVertical: 12,
    borderRadius: 12,
    borderWidth: 2,
  },
  optionDefault: {
    borderColor: "rgba(35,24,21,0.15)",
    backgroundColor: "rgba(35,24,21,0.02)",
  },
  optionCorrect: {
    borderColor: "#4ade80",
    backgroundColor: "#f0fdf4",
  },
  optionWrong: {
    borderColor: "#f87171",
    backgroundColor: "#fff1f2",
  },
  optionButton: {
    borderColor: "rgba(245,166,35,0.40)",
    backgroundColor: "#fff",
  },
  optionButtonPressed: {
    borderColor: "#f5a623",
    backgroundColor: "rgba(245,166,35,0.05)",
  },
  optionDisabled: {
    opacity: 0.5,
  },
  optionLabel: {
    fontSize: 14,
    fontWeight: "700",
    marginRight: 8,
  },
  optionText: {
    fontSize: 14,
    fontWeight: "500",
    flex: 1,
  },
  optionTextDefault: {
    color: "rgba(35,24,21,0.50)",
  },
  optionTextCorrect: {
    color: "#15803d",
  },
  optionTextWrong: {
    color: "#dc2626",
  },
  optionButtonLabel: {
    fontSize: 14,
    fontWeight: "700",
    color: "#f5a623",
    marginRight: 8,
  },
  optionButtonText: {
    fontSize: 14,
    fontWeight: "500",
    color: "#231815",
    flex: 1,
  },
  errorText: {
    marginTop: 10,
    fontSize: 12,
    color: "#d94040",
  },
});
