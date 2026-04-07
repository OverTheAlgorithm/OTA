// Extracted from: web/src/components/login-modal.tsx
// Differences: React Native Modal, no KakaoLoginButton web component (placeholder Pressable)

import {
  Modal,
  View,
  Text,
  Pressable,
  StyleSheet,
  Image,
} from "react-native";

interface LoginModalProps {
  open: boolean;
  onClose: () => void;
  redirectPath?: string;
  error?: boolean;
  onKakaoLogin?: () => void;
}

export function LoginModal({
  open,
  onClose,
  error,
  onKakaoLogin,
}: LoginModalProps) {
  return (
    <Modal
      visible={open}
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

          <View style={styles.textBlock}>
            <Text style={styles.title}>무료로 구독하기</Text>
            <Text style={styles.subtitle}>
              매일 아침 5분, 세상의 흐름을 읽는 위즈레터
            </Text>
          </View>

          {error && (
            <Text style={styles.errorText}>
              로그인에 실패했습니다. 다시 시도해주세요.
            </Text>
          )}

          {/* Kakao login button */}
          <Pressable
            onPress={onKakaoLogin}
            style={styles.kakaoBtn}
          >
            <Text style={styles.kakaoBtnText}>카카오로 시작하기</Text>
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
    gap: 20,
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
    color: "rgba(35,24,21,0.5)",
  },
  logo: {
    width: 80,
    height: 80,
  },
  textBlock: {
    alignItems: "center",
    gap: 4,
  },
  title: {
    fontSize: 20,
    fontWeight: "700",
    color: "#231815",
    textAlign: "center",
  },
  subtitle: {
    fontSize: 14,
    color: "rgba(35,24,21,0.6)",
    textAlign: "center",
  },
  errorText: {
    fontSize: 14,
    color: "#ff5442",
    textAlign: "center",
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
});
