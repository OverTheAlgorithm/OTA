// Extracted from: web/src/components/withdrawal-modal.tsx
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
import type { WithdrawalInfo } from "../lib/api";

interface Props {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  preCheckInfo: WithdrawalInfo;
}

export function WithdrawalModal({ open, onClose, onSuccess, preCheckInfo }: Props) {
  const unit = preCheckInfo.withdrawal_unit_amount;
  const maxWithdrawable = Math.floor(preCheckInfo.current_balance / unit) * unit;

  const [amount, setAmount] = useState(0);
  const [inputText, setInputText] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) return;
    setAmount(0);
    setInputText("");
    setError("");
    setSubmitting(false);
  }, [open]);

  const isAmountValid =
    amount > 0 && amount <= maxWithdrawable && amount % unit === 0;

  const handleAmountChange = (text: string) => {
    setInputText(text);
    const val = parseInt(text, 10);
    setError("");
    if (isNaN(val)) {
      setAmount(0);
      return;
    }
    if (val > maxWithdrawable) {
      setAmount(maxWithdrawable);
      setError(`최대 출금 가능 금액은 ${maxWithdrawable.toLocaleString()}P 입니다.`);
      return;
    }
    if (val % unit !== 0) {
      setAmount(val);
      setError(`출금 금액은 ${unit.toLocaleString()}P 단위로 입력해주세요.`);
      return;
    }
    setAmount(val);
  };

  const handleSubmit = async () => {
    if (!isAmountValid || submitting) return;
    setSubmitting(true);
    setError("");
    try {
      await api.requestWithdrawal(amount);
      onSuccess();
    } catch (e) {
      setError(e instanceof Error ? e.message : "출금 신청에 실패했습니다.");
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
      <Pressable
        style={styles.overlay}
        onPress={onClose}
      >
        <Pressable style={styles.card} onPress={() => {}}>
          {/* Close button */}
          <Pressable onPress={onClose} style={styles.closeBtn} accessibilityLabel="닫기">
            <Text style={styles.closeBtnText}>✕</Text>
          </Pressable>

          <Text style={styles.title}>출금 요청</Text>

          {/* Balance */}
          <View style={styles.balanceBlock}>
            <Text style={styles.balanceLabel}>현재 보유 포인트</Text>
            <Text style={styles.balanceValue}>
              {preCheckInfo.current_balance.toLocaleString()} P
            </Text>
          </View>

          {/* Amount input */}
          <View style={styles.inputBlock}>
            <Text style={styles.inputLabel}>출금할 금액</Text>
            <View style={styles.inputRow}>
              <TextInput
                style={styles.input}
                value={inputText}
                onChangeText={handleAmountChange}
                placeholder={`${unit.toLocaleString()}P 단위`}
                keyboardType="numeric"
                textAlign="right"
              />
              <Text style={styles.inputSuffix}>P</Text>
            </View>
          </View>

          {/* Remaining */}
          {isAmountValid && (
            <Text style={styles.remainingText}>
              출금 후 잔액: {(preCheckInfo.current_balance - amount).toLocaleString()} P
            </Text>
          )}

          {/* Error */}
          {!!error && <Text style={styles.errorText}>{error}</Text>}

          {/* Submit */}
          <Pressable
            onPress={handleSubmit}
            disabled={!isAmountValid || submitting}
            style={[styles.submitBtn, (!isAmountValid || submitting) && styles.submitBtnDisabled]}
          >
            <Text style={styles.submitBtnText}>
              {submitting ? "요청 중..." : "출금 요청하기"}
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
    maxWidth: 480,
    backgroundColor: "#fff",
    borderRadius: 22,
    borderWidth: 2,
    borderColor: "#231815",
    padding: 32,
    gap: 12,
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
    marginBottom: 8,
  },
  balanceBlock: {
    gap: 4,
  },
  balanceLabel: {
    fontSize: 14,
    color: "rgba(35,24,21,0.6)",
  },
  balanceValue: {
    fontSize: 30,
    fontWeight: "700",
    color: "#231815",
  },
  inputBlock: {
    gap: 8,
  },
  inputLabel: {
    fontSize: 14,
    fontWeight: "600",
    color: "#231815",
  },
  inputRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  input: {
    flex: 1,
    height: 48,
    paddingHorizontal: 16,
    borderWidth: 2,
    borderColor: "#231815",
    borderRadius: 12,
    fontSize: 18,
    color: "#231815",
  },
  inputSuffix: {
    fontSize: 18,
    fontWeight: "600",
    color: "#231815",
  },
  remainingText: {
    fontSize: 14,
    color: "rgba(35,24,21,0.6)",
  },
  errorText: {
    fontSize: 14,
    color: "#ff5442",
  },
  submitBtn: {
    marginTop: 8,
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
