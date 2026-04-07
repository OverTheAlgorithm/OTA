// Extracted from: web/src/components/send-briefing-button.tsx
// Differences: React Native View/Text/Pressable instead of HTML, no SVG (text icon)

import { useState } from "react";
import { View, Text, Pressable, StyleSheet } from "react-native";
import { api } from "../lib/api";

type SendState =
  | { type: "idle" }
  | { type: "sending" }
  | { type: "success" }
  | { type: "skipped" }
  | { type: "error"; message: string };

export function SendBriefingButton() {
  const [state, setState] = useState<SendState>({ type: "idle" });

  const handleSend = async () => {
    if (state.type === "sending") return;
    setState({ type: "sending" });

    try {
      const result = await api.sendBriefingNow();

      if (result.success_count > 0) {
        setState({ type: "success" });
      } else if (result.skipped_count > 0) {
        setState({ type: "skipped" });
      } else {
        setState({ type: "error", message: "전송에 실패했습니다. 채널 설정을 확인해주세요." });
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "전송에 실패했습니다.";
      setState({ type: "error", message });
    }
  };

  const isSending = state.type === "sending";

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <View style={styles.iconBox}>
          <Text style={styles.iconText}>✉</Text>
        </View>
        <Text style={styles.title}>지금 브리핑 받기</Text>
      </View>

      <Pressable
        onPress={handleSend}
        disabled={isSending}
        style={[styles.sendBtn, isSending && styles.sendBtnDisabled]}
      >
        <Text style={[styles.sendBtnText, isSending && styles.sendBtnTextDisabled]}>
          {isSending ? "전송 중..." : "브리핑 보내기"}
        </Text>
      </Pressable>

      {state.type === "success" && (
        <View style={[styles.feedback, styles.feedbackSuccess]}>
          <Text style={styles.feedbackTextSuccess}>브리핑이 전송되었습니다!</Text>
        </View>
      )}
      {state.type === "skipped" && (
        <View style={[styles.feedback, styles.feedbackSkipped]}>
          <Text style={styles.feedbackTextSkipped}>이미 오늘의 브리핑이 전송되었습니다.</Text>
        </View>
      )}
      {state.type === "error" && (
        <View style={[styles.feedback, styles.feedbackError]}>
          <Text style={styles.feedbackTextError}>{state.message}</Text>
        </View>
      )}

      <Text style={styles.note}>
        활성화된 채널로 최신 브리핑을 즉시 전송합니다.{"\n"}
        매일 아침 7시에 자동 전송되는 것과 동일한 내용입니다.
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    borderRadius: 16,
    backgroundColor: "#f0f7ff",
    borderWidth: 1,
    borderColor: "#d4e6f5",
    padding: 20,
    gap: 12,
  },
  header: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  iconBox: {
    width: 32,
    height: 32,
    borderRadius: 8,
    backgroundColor: "rgba(74,159,229,0.1)",
    justifyContent: "center",
    alignItems: "center",
  },
  iconText: {
    fontSize: 16,
    color: "#4a9fe5",
  },
  title: {
    fontSize: 16,
    fontWeight: "600",
    color: "#1e3a5f",
  },
  sendBtn: {
    borderRadius: 12,
    backgroundColor: "#26b0ff",
    paddingVertical: 12,
    alignItems: "center",
  },
  sendBtnDisabled: {
    backgroundColor: "#d4e6f5",
  },
  sendBtnText: {
    fontSize: 14,
    fontWeight: "600",
    color: "#fff",
  },
  sendBtnTextDisabled: {
    color: "#6b8db5",
  },
  feedback: {
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 8,
  },
  feedbackSuccess: {
    backgroundColor: "#dcfce7",
    borderWidth: 1,
    borderColor: "#86efac",
  },
  feedbackTextSuccess: {
    fontSize: 12,
    color: "#16a34a",
    textAlign: "center",
  },
  feedbackSkipped: {
    backgroundColor: "rgba(212,230,245,0.5)",
    borderWidth: 1,
    borderColor: "#d4e6f5",
  },
  feedbackTextSkipped: {
    fontSize: 12,
    color: "#6b8db5",
    textAlign: "center",
  },
  feedbackError: {
    backgroundColor: "rgba(255,84,66,0.1)",
    borderWidth: 1,
    borderColor: "rgba(255,84,66,0.2)",
  },
  feedbackTextError: {
    fontSize: 12,
    color: "#ff5442",
    textAlign: "center",
  },
  note: {
    fontSize: 12,
    color: "#6b8db5",
    textAlign: "center",
    lineHeight: 18,
  },
});
