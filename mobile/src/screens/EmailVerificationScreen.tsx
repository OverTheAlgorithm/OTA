// Ported from: web/src/pages/email-verification.tsx

import React, { useState } from "react";
import {
  View,
  Text,
  StyleSheet,
  TextInput,
  TouchableOpacity,
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  ActivityIndicator,
} from "react-native";
import { useNavigation, useRoute } from "@react-navigation/native";
import type { NativeStackNavigationProp, NativeStackScreenProps } from "@react-navigation/native-stack";
import { api } from "../lib/api";
import { useAuth } from "../contexts/auth-context";

type RootStackParamList = {
  MyPage: { tab?: string };
  EmailVerification: { auto_subscribe?: boolean };
};

type Props = NativeStackScreenProps<RootStackParamList, "EmailVerification">;

export function EmailVerificationScreen() {
  const navigation = useNavigation<NativeStackNavigationProp<RootStackParamList>>();
  const route = useRoute<Props["route"]>();
  const autoSubscribe = route.params?.auto_subscribe === true;
  const { refreshUser } = useAuth();

  const [step, setStep] = useState<"email" | "code">("email");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSendCode = async () => {
    setError("");
    const emailRegex = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/;
    if (!emailRegex.test(email)) {
      setError("올바른 이메일 형식을 입력해주세요");
      return;
    }
    setLoading(true);
    try {
      await api.sendVerificationCode(email);
      setStep("code");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "인증 코드 전송에 실패했습니다"
      );
    } finally {
      setLoading(false);
    }
  };

  const handleVerifyCode = async () => {
    setError("");
    if (code.length !== 6) {
      setError("6자리 인증 코드를 입력해주세요");
      return;
    }
    setLoading(true);
    try {
      await api.verifyEmailCode(code);
      await refreshUser();
      if (autoSubscribe) {
        const existing = await api.getDeliveryChannels();
        const hasEmail = existing.some((ch) => ch.channel === "email");
        const merged = hasEmail
          ? existing.map((ch) =>
              ch.channel === "email" ? { ...ch, enabled: true } : ch
            )
          : [...existing, { channel: "email", enabled: true }];
        await api.updateDeliveryChannels(merged);
        navigation.replace("MyPage", {});
      } else {
        navigation.replace("MyPage", { tab: "settings" });
      }
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "인증 코드 확인에 실패했습니다"
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <KeyboardAvoidingView
      style={styles.screen}
      behavior={Platform.OS === "ios" ? "padding" : undefined}
    >
      <ScrollView contentContainerStyle={styles.content}>
        <View style={styles.card}>
          {/* Close button */}
          <TouchableOpacity
            style={styles.closeButton}
            onPress={() => navigation.goBack()}
          >
            <Text style={styles.closeButtonText}>✕</Text>
          </TouchableOpacity>

          <Text style={styles.title}>이메일 인증하기</Text>

          {/* Step 1: Email */}
          {step === "email" && (
            <View style={styles.form}>
              <View style={styles.field}>
                <Text style={styles.label}>이메일 주소</Text>
                <TextInput
                  value={email}
                  onChangeText={setEmail}
                  placeholder="example@email.com"
                  placeholderTextColor="#96a0ad"
                  keyboardType="email-address"
                  autoCapitalize="none"
                  style={styles.input}
                  editable={!loading}
                  onSubmitEditing={handleSendCode}
                  returnKeyType="send"
                />
              </View>

              {error ? <Text style={styles.errorText}>{error}</Text> : null}

              <TouchableOpacity
                onPress={handleSendCode}
                disabled={loading}
                style={[styles.primaryButton, loading && styles.buttonDisabled]}
              >
                {loading ? (
                  <ActivityIndicator color="#231815" />
                ) : (
                  <Text style={styles.primaryButtonText}>인증번호 보내기</Text>
                )}
              </TouchableOpacity>
            </View>
          )}

          {/* Step 2: Code */}
          {step === "code" && (
            <View style={styles.form}>
              <Text style={styles.subtext}>
                {email}로 인증 코드를 전송했습니다
              </Text>

              <View style={styles.field}>
                <Text style={styles.label}>인증 코드 (6자리)</Text>
                <TextInput
                  value={code}
                  onChangeText={(v) => setCode(v.replace(/\D/g, "").slice(0, 6))}
                  placeholder="000000"
                  placeholderTextColor="#96a0ad"
                  keyboardType="numeric"
                  maxLength={6}
                  style={styles.input}
                  editable={!loading}
                  onSubmitEditing={handleVerifyCode}
                  returnKeyType="done"
                />
              </View>

              {error ? <Text style={styles.errorText}>{error}</Text> : null}

              <View style={styles.buttonGroup}>
                <TouchableOpacity
                  onPress={handleVerifyCode}
                  disabled={loading || code.length !== 6}
                  style={[
                    styles.primaryButton,
                    (loading || code.length !== 6) && styles.buttonDisabled,
                  ]}
                >
                  {loading ? (
                    <ActivityIndicator color="#231815" />
                  ) : (
                    <Text style={styles.primaryButtonText}>인증 완료</Text>
                  )}
                </TouchableOpacity>

                <TouchableOpacity
                  onPress={() => {
                    setStep("email");
                    setCode("");
                    setError("");
                  }}
                  disabled={loading}
                  style={[styles.secondaryButton, loading && styles.buttonDisabled]}
                >
                  <Text style={styles.secondaryButtonText}>이메일 다시 입력하기</Text>
                </TouchableOpacity>
              </View>
            </View>
          )}
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: "#fdf9ee",
  },
  content: {
    flexGrow: 1,
    alignItems: "center",
    justifyContent: "center",
    padding: 16,
    paddingVertical: 40,
  },
  card: {
    width: "100%",
    maxWidth: 440,
    backgroundColor: "#fff",
    borderRadius: 24,
    padding: 32,
    gap: 20,
  },
  closeButton: {
    position: "absolute",
    top: 16,
    right: 16,
    width: 36,
    height: 36,
    alignItems: "center",
    justifyContent: "center",
    borderRadius: 18,
  },
  closeButtonText: {
    fontSize: 18,
    color: "#231815",
  },
  title: {
    fontSize: 28,
    fontWeight: "600",
    color: "#231815",
    marginTop: 8,
  },
  form: {
    gap: 16,
  },
  subtext: {
    fontSize: 14,
    color: "#231815",
  },
  field: {
    gap: 6,
  },
  label: {
    fontSize: 13,
    fontWeight: "500",
    color: "#231815",
  },
  input: {
    paddingHorizontal: 16,
    paddingVertical: 14,
    borderWidth: 1,
    borderColor: "#bdc4cd",
    borderRadius: 12,
    fontSize: 15,
    color: "#231815",
    backgroundColor: "#fff",
  },
  errorText: {
    fontSize: 13,
    color: "#ff5442",
  },
  buttonGroup: {
    gap: 10,
  },
  primaryButton: {
    paddingVertical: 14,
    borderRadius: 999,
    backgroundColor: "#43b9d6",
    borderWidth: 2,
    borderColor: "#231815",
    alignItems: "center",
  },
  primaryButtonText: {
    fontSize: 15,
    fontWeight: "600",
    color: "#231815",
  },
  secondaryButton: {
    paddingVertical: 14,
    borderRadius: 999,
    backgroundColor: "#fff",
    borderWidth: 2,
    borderColor: "#231815",
    alignItems: "center",
  },
  secondaryButtonText: {
    fontSize: 15,
    fontWeight: "600",
    color: "#231815",
  },
  buttonDisabled: {
    opacity: 0.5,
  },
});
