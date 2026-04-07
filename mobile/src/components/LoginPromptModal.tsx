// Extracted from: web/src/components/login-prompt-modal.tsx
// Differences: AsyncStorage instead of localStorage for dismiss state

import { Modal, View, Text, Pressable, StyleSheet, Image } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";

const DISMISS_KEY = "wl_login_prompt_dismiss";

export async function isLoginPromptDismissed(): Promise<boolean> {
  const dismissed = await AsyncStorage.getItem(DISMISS_KEY).catch(() => null);
  if (!dismissed) return false;
  return Date.now() < Number(dismissed);
}

async function dismissForOneDay() {
  const oneDayMs = 24 * 60 * 60 * 1000;
  await AsyncStorage.setItem(DISMISS_KEY, String(Date.now() + oneDayMs)).catch(() => {});
}

interface LoginPromptModalProps {
  visible: boolean;
  onClose: () => void;
  onKakaoLogin?: () => void;
}

export function LoginPromptModal({
  visible,
  onClose,
  onKakaoLogin,
}: LoginPromptModalProps) {
  const handleDismissOneDay = async () => {
    await dismissForOneDay();
    onClose();
  };

  return (
    <Modal
      visible={visible}
      transparent
      animationType="fade"
      onRequestClose={onClose}
    >
      <Pressable style={styles.overlay} onPress={onClose}>
        <Pressable style={styles.card} onPress={() => {}}>
          {/* Close button */}
          <Pressable onPress={onClose} style={styles.closeBtn}>
            <Text style={styles.closeBtnText}>✕</Text>
          </Pressable>

          {/* TODO: replace icon.png with wl-logo-square.png once brand assets are added to mobile/assets/ */}
          <Image
            source={require("../../assets/icon.png")}
            style={styles.logo}
            resizeMode="contain"
          />

          <Text style={styles.title}>
            {"로그인하고 기사를 읽으면\n포인트를 획득할 수 있어요!"}
          </Text>

          {/* Kakao login */}
          <Pressable onPress={onKakaoLogin} style={styles.kakaoBtn}>
            <Text style={styles.kakaoBtnText}>카카오로 시작하기</Text>
          </Pressable>

          <Pressable onPress={handleDismissOneDay}>
            <Text style={styles.dismissText}>하루 동안 보지 않기</Text>
          </Pressable>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

const styles = StyleSheet.create({
  overlay: {
    flex: 1,
    backgroundColor: "rgba(0,0,0,0.6)",
    justifyContent: "center",
    alignItems: "center",
    paddingHorizontal: 16,
  },
  card: {
    width: "100%",
    maxWidth: 360,
    backgroundColor: "#fdf9ee",
    borderWidth: 3,
    borderColor: "#231815",
    borderRadius: 20,
    padding: 32,
    alignItems: "center",
    gap: 16,
  },
  closeBtn: {
    position: "absolute",
    top: 16,
    right: 16,
    width: 32,
    height: 32,
    justifyContent: "center",
    alignItems: "center",
  },
  closeBtnText: {
    fontSize: 18,
    color: "rgba(35,24,21,0.6)",
  },
  logo: {
    width: 56,
    height: 56,
    borderRadius: 12,
  },
  title: {
    fontSize: 18,
    fontWeight: "700",
    color: "#231815",
    textAlign: "center",
    lineHeight: 26,
  },
  kakaoBtn: {
    width: "100%",
    height: 48,
    borderRadius: 24,
    backgroundColor: "#FEE500",
    justifyContent: "center",
    alignItems: "center",
    borderWidth: 2,
    borderColor: "#231815",
  },
  kakaoBtnText: {
    fontSize: 16,
    fontWeight: "600",
    color: "#231815",
  },
  dismissText: {
    fontSize: 12,
    color: "rgba(35,24,21,0.4)",
  },
});
