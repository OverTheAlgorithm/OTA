// Extracted from: web/src/components/bank-account-modal.tsx
// Differences: React Native Modal + TextInput instead of createPortal + input

import { useEffect, useState } from "react";
import {
  Modal,
  View,
  Text,
  Pressable,
  TextInput,
  StyleSheet,
} from "react-native";
import { api } from "../lib/api";
import type { BankAccount } from "../lib/api";

interface Props {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  existing: BankAccount | null;
}

export function BankAccountModal({ open, onClose, onSuccess, existing }: Props) {
  const [bankName, setBankName] = useState("");
  const [accountNumber, setAccountNumber] = useState("");
  const [accountHolder, setAccountHolder] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) return;
    setBankName(existing?.bank_name ?? "");
    setAccountNumber(existing?.account_number ?? "");
    setAccountHolder(existing?.account_holder ?? "");
    setError("");
    setSubmitting(false);
  }, [open, existing]);

  const isValid =
    bankName.trim() !== "" &&
    accountNumber.trim() !== "" &&
    accountHolder.trim() !== "";

  const handleSubmit = async () => {
    if (!isValid || submitting) return;
    setSubmitting(true);
    setError("");
    try {
      await api.saveBankAccount({
        bank_name: bankName.trim(),
        account_number: accountNumber.trim(),
        account_holder: accountHolder.trim(),
      });
      onSuccess();
    } catch (e) {
      setError(e instanceof Error ? e.message : "계좌 등록에 실패했습니다.");
    } finally {
      setSubmitting(false);
    }
  };

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

          <Text style={styles.title}>계좌번호 등록/변경</Text>
          <Text style={styles.description}>
            포인트 교환 시 사용되며, 본인 명의의 계좌만 등록할 수 있습니다.{"\n"}
            입력하신 개인 정보는 안전하게 보관됩니다.
          </Text>

          <View style={styles.inputs}>
            <TextInput
              style={styles.input}
              value={bankName}
              onChangeText={setBankName}
              placeholder="은행 (예: 신한, 카카오뱅크)"
              placeholderTextColor="rgba(35,24,21,0.4)"
            />
            <TextInput
              style={styles.input}
              value={accountNumber}
              onChangeText={setAccountNumber}
              placeholder="계좌번호"
              placeholderTextColor="rgba(35,24,21,0.4)"
              keyboardType="numeric"
            />
            <TextInput
              style={styles.input}
              value={accountHolder}
              onChangeText={setAccountHolder}
              placeholder="예금주"
              placeholderTextColor="rgba(35,24,21,0.4)"
            />
          </View>

          {!!error && <Text style={styles.errorText}>{error}</Text>}

          <Pressable
            onPress={handleSubmit}
            disabled={!isValid || submitting}
            style={[styles.submitBtn, (!isValid || submitting) && styles.submitBtnDisabled]}
          >
            <Text style={styles.submitBtnText}>
              {submitting ? "처리 중..." : "등록"}
            </Text>
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
    maxWidth: 520,
    backgroundColor: "#fff",
    borderRadius: 30,
    paddingHorizontal: 32,
    paddingVertical: 40,
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
  title: {
    fontSize: 24,
    fontWeight: "700",
    color: "#231815",
  },
  description: {
    fontSize: 14,
    color: "rgba(35,24,21,0.7)",
    textAlign: "center",
    lineHeight: 20,
  },
  inputs: {
    gap: 12,
  },
  input: {
    height: 48,
    paddingHorizontal: 16,
    borderWidth: 1,
    borderColor: "#bdc4cd",
    borderRadius: 10,
    fontSize: 16,
    color: "#231815",
  },
  errorText: {
    fontSize: 14,
    color: "#ff5442",
  },
  submitBtn: {
    height: 50,
    borderRadius: 25,
    backgroundColor: "#43b9d6",
    borderWidth: 2,
    borderColor: "#231815",
    justifyContent: "center",
    alignItems: "center",
  },
  submitBtnDisabled: {
    opacity: 0.4,
  },
  submitBtnText: {
    fontSize: 18,
    fontWeight: "600",
    color: "#231815",
  },
});
