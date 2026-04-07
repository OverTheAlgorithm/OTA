// Extracted from: web/src/components/interest-section.tsx

import React, { useState } from "react";
import {
  View,
  Text,
  TouchableOpacity,
  StyleSheet,
} from "react-native";
import { api } from "../lib/api";

const CATEGORIES: { key: string; label: string; emoji: string }[] = [
  { key: "entertainment", label: "연예/오락", emoji: "🎬" },
  { key: "business", label: "경제/비즈니스", emoji: "💰" },
  { key: "sports", label: "스포츠", emoji: "⚽" },
  { key: "technology", label: "IT/기술", emoji: "💻" },
  { key: "science", label: "과학", emoji: "🔬" },
  { key: "health", label: "건강/의학", emoji: "🏥" },
];

interface Props {
  selected: string[];
  onChange: (updated: string[]) => void;
}

export function InterestSection({ selected, onChange }: Props) {
  const [saving, setSaving] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const handleToggle = async (key: string) => {
    if (saving) return;

    setSaving(true);
    setErrorMsg(null);
    const prev = selected;
    const isSelected = selected.includes(key);

    if (isSelected) {
      onChange(selected.filter((s) => s !== key));
      try {
        await api.deleteSubscription(key);
      } catch {
        onChange(prev);
        setErrorMsg("저장에 실패했습니다. 다시 시도해주세요.");
      } finally {
        setSaving(false);
      }
    } else {
      onChange([...selected, key]);
      try {
        await api.addSubscription(key);
      } catch {
        onChange(prev);
        setErrorMsg("저장에 실패했습니다. 다시 시도해주세요.");
      } finally {
        setSaving(false);
      }
    }
  };

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>내 관심사</Text>
        <Text style={styles.subtitle}>
          {selected.length > 0
            ? `${selected.length}개 선택됨`
            : "관심사를 선택하면 맞춤 소식을 받아요"}
        </Text>
      </View>

      <View style={styles.grid}>
        {CATEGORIES.map((cat) => {
          const isActive = selected.includes(cat.key);
          return (
            <TouchableOpacity
              key={cat.key}
              onPress={() => handleToggle(cat.key)}
              disabled={saving}
              style={[
                styles.chip,
                isActive ? styles.chipActive : styles.chipInactive,
                saving && styles.chipDisabled,
              ]}
            >
              <Text style={styles.chipEmoji}>{cat.emoji}</Text>
              <Text
                style={[
                  styles.chipLabel,
                  isActive ? styles.chipLabelActive : styles.chipLabelInactive,
                ]}
              >
                {cat.label}
              </Text>
            </TouchableOpacity>
          );
        })}
      </View>

      {errorMsg && <Text style={styles.error}>{errorMsg}</Text>}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    borderLeftWidth: 3,
    borderLeftColor: "#43b9d6",
    paddingLeft: 16,
  },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    marginBottom: 16,
  },
  title: {
    fontSize: 16,
    fontWeight: "700",
    color: "#231815",
  },
  subtitle: {
    fontSize: 12,
    color: "rgba(35,24,21,0.5)",
  },
  grid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 8,
  },
  chip: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    paddingHorizontal: 12,
    paddingVertical: 10,
    borderRadius: 12,
    borderWidth: 2,
    width: "47%",
  },
  chipActive: {
    backgroundColor: "rgba(67,185,214,0.1)",
    borderColor: "#43b9d6",
  },
  chipInactive: {
    backgroundColor: "#fff",
    borderColor: "#231815",
  },
  chipDisabled: {
    opacity: 0.5,
  },
  chipEmoji: {
    fontSize: 18,
  },
  chipLabel: {
    fontSize: 13,
    fontWeight: "500",
  },
  chipLabelActive: {
    color: "#231815",
  },
  chipLabelInactive: {
    color: "#231815",
  },
  error: {
    marginTop: 8,
    fontSize: 12,
    color: "#ff5442",
  },
});
