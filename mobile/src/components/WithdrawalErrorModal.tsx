// Extracted from: web/src/components/withdrawal-error-modal.tsx
// Differences: React Native Modal instead of createPortal, Image instead of img

import { Modal, View, Text, Pressable, StyleSheet, Image } from "react-native";

interface Props {
  open: boolean;
  message: string;
  onClose: () => void;
}

export function WithdrawalErrorModal({ open, message, onClose }: Props) {
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
          <Pressable onPress={onClose} style={styles.closeBtn} accessibilityLabel="닫기">
            <Text style={styles.closeBtnText}>✕</Text>
          </Pressable>

          {/* TODO: replace icon.png with wl-piggy.png once brand assets are added to mobile/assets/ */}
          <View style={styles.imageContainer}>
            <Image
              source={require("../../assets/icon.png")}
              style={styles.piggyImage}
              resizeMode="contain"
            />
          </View>

          <Text style={styles.message}>{message}</Text>

          <Pressable onPress={onClose} style={styles.confirmBtn}>
            <Text style={styles.confirmBtnText}>확인</Text>
          </Pressable>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

const styles = StyleSheet.create({
  overlay: {
    flex: 1,
    backgroundColor: "rgba(0,0,0,0.5)",
    justifyContent: "center",
    alignItems: "center",
    paddingHorizontal: 16,
  },
  card: {
    width: "100%",
    maxWidth: 500,
    backgroundColor: "#fff",
    borderRadius: 30,
    padding: 40,
    alignItems: "center",
    gap: 16,
  },
  closeBtn: {
    position: "absolute",
    top: 16,
    right: 16,
    width: 40,
    height: 40,
    justifyContent: "center",
    alignItems: "center",
  },
  closeBtnText: {
    fontSize: 18,
    color: "rgba(35,24,21,0.5)",
  },
  imageContainer: {
    alignItems: "center",
    marginBottom: 8,
  },
  piggyImage: {
    width: 120,
    height: 108,
  },
  message: {
    fontSize: 18,
    color: "#231815",
    textAlign: "center",
    lineHeight: 26,
  },
  confirmBtn: {
    marginTop: 16,
    width: 300,
    maxWidth: "100%",
    height: 50,
    borderRadius: 25,
    borderWidth: 2,
    borderColor: "#231815",
    backgroundColor: "#fff",
    justifyContent: "center",
    alignItems: "center",
  },
  confirmBtnText: {
    fontSize: 18,
    fontWeight: "600",
    color: "#231815",
  },
});
