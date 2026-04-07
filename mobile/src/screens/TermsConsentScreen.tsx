// Ported from: web/src/pages/terms-consent.tsx

import React, { useEffect, useState } from "react";
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  TextInput,
  ActivityIndicator,
  Linking,
  Modal,
  KeyboardAvoidingView,
  Platform,
} from "react-native";
import { useNavigation, useRoute } from "@react-navigation/native";
import type { NativeStackNavigationProp, NativeStackScreenProps } from "@react-navigation/native-stack";
import { api } from "../lib/api";
import { useAuth } from "../contexts/auth-context";
import type { Term } from "../../../packages/shared/src/types";

type RootStackParamList = {
  Landing: undefined;
  Latest: undefined;
  TermsConsent: { signupKey: string };
  EmailVerification: { auto_subscribe?: boolean };
};

type ModalStep = "none" | "email-input" | "code-input" | "skip-nudge";

type Props = NativeStackScreenProps<RootStackParamList, "TermsConsent">;

export function TermsConsentScreen() {
  const navigation = useNavigation<NativeStackNavigationProp<RootStackParamList>>();
  const route = useRoute<Props["route"]>();
  const signupKey = route.params?.signupKey;
  const { refreshUser } = useAuth();

  const [terms, setTerms] = useState<Term[]>([]);
  const [agreed, setAgreed] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Email verification modal
  const [modalStep, setModalStep] = useState<ModalStep>("none");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [modalLoading, setModalLoading] = useState(false);
  const [modalError, setModalError] = useState("");

  useEffect(() => {
    if (!signupKey) {
      navigation.replace("Landing");
      return;
    }
    api
      .getActiveTerms()
      .then((list) => {
        const sorted = [...list].sort((a, b) => {
          if (a.required !== b.required) return a.required ? -1 : 1;
          return new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
        });
        setTerms(sorted);
        setLoading(false);
      })
      .catch(() => {
        setError("약관 목록을 불러올 수 없습니다");
        setLoading(false);
      });
  }, [signupKey, navigation]);

  const requiredIds = terms.filter((t) => t.required).map((t) => t.id);
  const allRequiredAgreed = requiredIds.every((id) => agreed.has(id));

  const toggleTerm = (id: string) => {
    setAgreed((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const goToFinalDestination = () => {
    navigation.replace("Latest");
  };

  const handleSubmit = async () => {
    if (!signupKey || !allRequiredAgreed) return;
    setSubmitting(true);
    setError(null);
    try {
      await api.completeSignup(signupKey, Array.from(agreed));
      await refreshUser();
      setModalStep("email-input");
    } catch (e) {
      setError(e instanceof Error ? e.message : "회원가입에 실패했습니다");
      setSubmitting(false);
    }
  };

  const handleSendCode = async () => {
    const emailRegex = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/;
    if (!emailRegex.test(email)) {
      setModalError("올바른 이메일 형식을 입력해주세요");
      return;
    }
    setModalLoading(true);
    setModalError("");
    try {
      await api.sendVerificationCode(email);
      setModalStep("code-input");
    } catch (err) {
      setModalError(
        err instanceof Error ? err.message : "인증 코드 전송에 실패했습니다"
      );
    } finally {
      setModalLoading(false);
    }
  };

  const handleVerifyCode = async () => {
    if (code.length !== 6) {
      setModalError("6자리 인증 코드를 입력해주세요");
      return;
    }
    setModalLoading(true);
    setModalError("");
    try {
      await api.verifyEmailCode(code);
      await refreshUser();
      goToFinalDestination();
    } catch (err) {
      setModalError(
        err instanceof Error ? err.message : "인증 코드 확인에 실패했습니다"
      );
    } finally {
      setModalLoading(false);
    }
  };

  const openTermUrl = (term: Term) => {
    if (!term.url) return;
    const url = term.url.match(/^https?:\/\//) ? term.url : `https://${term.url}`;
    Linking.openURL(url);
  };

  if (loading) {
    return (
      <View style={styles.centered}>
        <ActivityIndicator color="#43b9d6" />
      </View>
    );
  }

  return (
    <KeyboardAvoidingView
      style={styles.screen}
      behavior={Platform.OS === "ios" ? "padding" : undefined}
    >
      <ScrollView contentContainerStyle={styles.content}>
        <View style={styles.card}>
          <Text style={styles.title}>위즈레터 시작하기</Text>

          {error && (
            <View style={styles.errorBox}>
              <Text style={styles.errorBoxText}>{error}</Text>
            </View>
          )}

          <View style={styles.termsList}>
            {terms.map((term) => (
              <TouchableOpacity
                key={term.id}
                onPress={() => toggleTerm(term.id)}
                style={styles.termRow}
                activeOpacity={0.7}
              >
                <View
                  style={[
                    styles.checkbox,
                    agreed.has(term.id) && styles.checkboxChecked,
                  ]}
                >
                  {agreed.has(term.id) && (
                    <Text style={styles.checkmark}>✓</Text>
                  )}
                </View>
                <View style={styles.termTextContainer}>
                  <Text style={styles.termText}>
                    {term.url ? (
                      <Text
                        style={styles.termLink}
                        onPress={(e) => {
                          e.stopPropagation?.();
                          openTermUrl(term);
                        }}
                      >
                        {term.title}
                      </Text>
                    ) : (
                      term.title
                    )}{" "}
                    <Text style={styles.termRequired}>
                      ({term.required ? "필수" : "선택"})
                    </Text>
                  </Text>
                  {term.description ? (
                    <Text style={styles.termDescription}>{term.description}</Text>
                  ) : null}
                </View>
              </TouchableOpacity>
            ))}
          </View>

          {!allRequiredAgreed && terms.length > 0 && (
            <Text style={styles.requiredHint}>필수 약관에 모두 동의해주세요.</Text>
          )}

          <View style={styles.actions}>
            <TouchableOpacity
              onPress={handleSubmit}
              disabled={!allRequiredAgreed || submitting}
              style={[
                styles.primaryButton,
                (!allRequiredAgreed || submitting) && styles.primaryButtonDisabled,
              ]}
            >
              <Text style={styles.primaryButtonText}>
                {submitting ? "처리 중..." : "동의하고 가입하기"}
              </Text>
            </TouchableOpacity>
            <TouchableOpacity
              onPress={() => navigation.replace("Landing")}
              style={styles.secondaryButton}
            >
              <Text style={styles.secondaryButtonText}>돌아가기</Text>
            </TouchableOpacity>
          </View>
        </View>
      </ScrollView>

      {/* Email verification modal */}
      <Modal visible={modalStep !== "none"} transparent animationType="fade">
        <View style={styles.modalOverlay}>
          {modalStep === "email-input" && (
            <View style={styles.modalCard}>
              <TouchableOpacity
                style={styles.modalClose}
                onPress={() => setModalStep("skip-nudge")}
              >
                <Text style={styles.modalCloseText}>✕</Text>
              </TouchableOpacity>
              <Text style={styles.modalTitle}>이메일 인증하기</Text>
              <View style={styles.modalField}>
                <Text style={styles.modalLabel}>이메일 주소</Text>
                <TextInput
                  value={email}
                  onChangeText={setEmail}
                  placeholder="example@email.com"
                  placeholderTextColor="#96a0ad"
                  keyboardType="email-address"
                  autoCapitalize="none"
                  style={styles.modalInput}
                  editable={!modalLoading}
                />
              </View>
              {modalError ? <Text style={styles.modalError}>{modalError}</Text> : null}
              <TouchableOpacity
                onPress={handleSendCode}
                disabled={modalLoading}
                style={[styles.modalPrimaryButton, modalLoading && styles.modalButtonDisabled]}
              >
                <Text style={styles.modalPrimaryButtonText}>
                  {modalLoading ? "전송 중..." : "인증번호 보내기"}
                </Text>
              </TouchableOpacity>
              <TouchableOpacity
                onPress={() => setModalStep("skip-nudge")}
                style={styles.modalSkipButton}
              >
                <Text style={styles.modalSkipText}>건너뛰기</Text>
              </TouchableOpacity>
            </View>
          )}

          {modalStep === "code-input" && (
            <View style={styles.modalCard}>
              <TouchableOpacity
                style={styles.modalClose}
                onPress={goToFinalDestination}
              >
                <Text style={styles.modalCloseText}>✕</Text>
              </TouchableOpacity>
              <Text style={styles.modalTitle}>이메일 인증하기</Text>
              <Text style={styles.modalSubtext}>{email}로 인증 코드를 전송했습니다</Text>
              <View style={styles.modalField}>
                <Text style={styles.modalLabel}>인증 코드 (6자리)</Text>
                <TextInput
                  value={code}
                  onChangeText={(v) => setCode(v.replace(/\D/g, "").slice(0, 6))}
                  placeholder="000000"
                  placeholderTextColor="#96a0ad"
                  keyboardType="numeric"
                  maxLength={6}
                  style={styles.modalInput}
                  editable={!modalLoading}
                />
              </View>
              {modalError ? <Text style={styles.modalError}>{modalError}</Text> : null}
              <TouchableOpacity
                onPress={handleVerifyCode}
                disabled={modalLoading || code.length !== 6}
                style={[
                  styles.modalPrimaryButton,
                  (modalLoading || code.length !== 6) && styles.modalButtonDisabled,
                ]}
              >
                <Text style={styles.modalPrimaryButtonText}>
                  {modalLoading ? "확인 중..." : "인증 완료"}
                </Text>
              </TouchableOpacity>
              <TouchableOpacity
                onPress={() => {
                  setModalStep("email-input");
                  setCode("");
                  setModalError("");
                }}
                disabled={modalLoading}
                style={styles.modalSecondaryButton}
              >
                <Text style={styles.modalSecondaryButtonText}>이메일 다시 입력하기</Text>
              </TouchableOpacity>
            </View>
          )}

          {modalStep === "skip-nudge" && (
            <View style={styles.nudgeCard}>
              <TouchableOpacity
                style={styles.modalClose}
                onPress={goToFinalDestination}
              >
                <Text style={styles.modalCloseText}>✕</Text>
              </TouchableOpacity>
              <Text style={styles.nudgeTitle}>
                {"잠깐!\n이메일을 인증하지 않으면\n매일 아침 최신 소식을 받을 수 없어요!"}
              </Text>
              <Text style={styles.nudgeBody}>
                위즈레터를 구독하시면 복잡한 소식을 간결하게 요약해서, 매일 아침 7시에 보내드립니다.
                이메일 인증을 완료하고 슬기로운 아침을 시작해 보세요!
              </Text>
              <View style={styles.nudgeActions}>
                <TouchableOpacity
                  onPress={goToFinalDestination}
                  style={styles.nudgeSecondaryButton}
                >
                  <Text style={styles.nudgeSecondaryButtonText}>닫기</Text>
                </TouchableOpacity>
                <TouchableOpacity
                  onPress={() => {
                    setModalStep("email-input");
                    setModalError("");
                  }}
                  style={styles.nudgePrimaryButton}
                >
                  <Text style={styles.nudgePrimaryButtonText}>이메일 인증하기</Text>
                </TouchableOpacity>
              </View>
            </View>
          )}
        </View>
      </Modal>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: "#f5f5f5" },
  content: { padding: 16, alignItems: "center", paddingBottom: 40 },
  centered: { flex: 1, alignItems: "center", justifyContent: "center" },
  card: {
    width: "100%",
    maxWidth: 480,
    backgroundColor: "#fff",
    borderRadius: 20,
    borderWidth: 2.5,
    borderColor: "#231815",
    padding: 24,
    gap: 20,
  },
  title: {
    fontSize: 24,
    fontWeight: "600",
    color: "#231815",
    textAlign: "center",
  },
  errorBox: {
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "rgba(255,84,66,0.3)",
    backgroundColor: "rgba(255,84,66,0.1)",
    padding: 12,
  },
  errorBoxText: { fontSize: 13, color: "#ff5442" },
  termsList: { gap: 16 },
  termRow: { flexDirection: "row", alignItems: "flex-start", gap: 10 },
  checkbox: {
    width: 22,
    height: 22,
    borderRadius: 6,
    borderWidth: 1.5,
    borderColor: "#a8a8a8",
    alignItems: "center",
    justifyContent: "center",
    marginTop: 1,
    flexShrink: 0,
  },
  checkboxChecked: {
    backgroundColor: "#231815",
    borderColor: "#231815",
  },
  checkmark: { color: "#fff", fontSize: 12, fontWeight: "700" },
  termTextContainer: { flex: 1 },
  termText: { fontSize: 13, color: "#231815", lineHeight: 18 },
  termLink: { textDecorationLine: "underline" },
  termRequired: { color: "#525252" },
  termDescription: { fontSize: 11, color: "#525252", marginTop: 2 },
  requiredHint: {
    fontSize: 11,
    color: "#ff5442",
    textAlign: "center",
  },
  actions: { gap: 10 },
  primaryButton: {
    paddingVertical: 14,
    borderRadius: 12,
    backgroundColor: "#43b9d6",
    alignItems: "center",
  },
  primaryButtonDisabled: { opacity: 0.5 },
  primaryButtonText: { fontSize: 15, fontWeight: "600", color: "#231815" },
  secondaryButton: {
    paddingVertical: 14,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: "#a8a8a8",
    backgroundColor: "#fff",
    alignItems: "center",
  },
  secondaryButtonText: { fontSize: 15, fontWeight: "600", color: "#525252" },

  // Modal
  modalOverlay: {
    flex: 1,
    backgroundColor: "rgba(0,0,0,0.5)",
    alignItems: "center",
    justifyContent: "center",
    padding: 16,
  },
  modalCard: {
    width: "100%",
    maxWidth: 440,
    backgroundColor: "#fff",
    borderRadius: 24,
    padding: 32,
    gap: 16,
  },
  modalClose: {
    position: "absolute",
    top: 16,
    right: 16,
    width: 36,
    height: 36,
    alignItems: "center",
    justifyContent: "center",
  },
  modalCloseText: { fontSize: 18, color: "#231815" },
  modalTitle: {
    fontSize: 26,
    fontWeight: "600",
    color: "#231815",
    marginTop: 8,
  },
  modalSubtext: { fontSize: 14, color: "#231815" },
  modalField: { gap: 6 },
  modalLabel: { fontSize: 13, fontWeight: "500", color: "#231815" },
  modalInput: {
    paddingHorizontal: 16,
    paddingVertical: 14,
    borderWidth: 1,
    borderColor: "#bdc4cd",
    borderRadius: 12,
    fontSize: 15,
    color: "#231815",
    backgroundColor: "#fff",
  },
  modalError: { fontSize: 13, color: "#ff5442" },
  modalPrimaryButton: {
    paddingVertical: 14,
    borderRadius: 999,
    backgroundColor: "#43b9d6",
    borderWidth: 2,
    borderColor: "#231815",
    alignItems: "center",
  },
  modalButtonDisabled: { opacity: 0.5 },
  modalPrimaryButtonText: { fontSize: 15, fontWeight: "600", color: "#231815" },
  modalSecondaryButton: {
    paddingVertical: 14,
    borderRadius: 999,
    backgroundColor: "#fff",
    borderWidth: 2,
    borderColor: "#231815",
    alignItems: "center",
  },
  modalSecondaryButtonText: { fontSize: 15, fontWeight: "600", color: "#231815" },
  modalSkipButton: { alignItems: "center" },
  modalSkipText: { fontSize: 13, fontWeight: "500", color: "#3d3d3d" },

  // Skip nudge
  nudgeCard: {
    width: "100%",
    maxWidth: 480,
    backgroundColor: "#fff",
    borderRadius: 32,
    padding: 32,
    gap: 16,
  },
  nudgeTitle: {
    fontSize: 22,
    fontWeight: "600",
    color: "#000",
    lineHeight: 32,
    marginTop: 8,
  },
  nudgeBody: {
    fontSize: 15,
    color: "#000",
    lineHeight: 24,
  },
  nudgeActions: {
    flexDirection: "row",
    gap: 12,
    marginTop: 8,
  },
  nudgeSecondaryButton: {
    flex: 1,
    paddingVertical: 14,
    borderRadius: 999,
    backgroundColor: "#fff",
    borderWidth: 2,
    borderColor: "#231815",
    alignItems: "center",
  },
  nudgeSecondaryButtonText: { fontSize: 15, fontWeight: "600", color: "#231815" },
  nudgePrimaryButton: {
    flex: 1,
    paddingVertical: 14,
    borderRadius: 999,
    backgroundColor: "#43b9d6",
    borderWidth: 2,
    borderColor: "#231815",
    alignItems: "center",
  },
  nudgePrimaryButtonText: { fontSize: 15, fontWeight: "600", color: "#231815" },
});
