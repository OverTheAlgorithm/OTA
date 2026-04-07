// Extracted from: web/src/components/channel-preferences-section.tsx

import React, { useEffect, useState } from "react";
import {
  View,
  Text,
  Switch,
  StyleSheet,
  TouchableOpacity,
} from "react-native";
import { useNavigation } from "@react-navigation/native";
import type { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { api } from "../lib/api";
import { useAuth } from "../contexts/auth-context";
import type { ChannelPreference, ChannelDeliveryStatus } from "../../../packages/shared/src/types";

type RootStackParamList = {
  EmailVerification: { auto_subscribe?: boolean };
};

const CHANNEL_INFO = {
  email: { label: "이메일", icon: "📧", description: "이메일로 소식을 받아요" },
};

const CHANNEL_ORDER = ["email"];
const MAX_RETRIES = 3;

export function ChannelPreferencesSection() {
  const { user } = useAuth();
  const navigation = useNavigation<NativeStackNavigationProp<RootStackParamList>>();
  const [channels, setChannels] = useState<ChannelPreference[]>([]);
  const [deliveryStatuses, setDeliveryStatuses] = useState<ChannelDeliveryStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const emailVerified = user?.email_verified ?? false;

  useEffect(() => {
    api
      .getDeliveryChannels()
      .then((data) => {
        const channelMap = new Map(data.map((ch) => [ch.channel, ch.enabled]));
        const allChannels = CHANNEL_ORDER.map((channel) => ({
          channel,
          enabled: channelMap.get(channel) ?? false,
        }));
        setChannels(allChannels);
      })
      .catch(() => {
        setChannels(CHANNEL_ORDER.map((channel) => ({ channel, enabled: false })));
      })
      .finally(() => setLoading(false));

    api.getDeliveryStatus().then(setDeliveryStatuses).catch(() => {});
  }, []);

  const handleToggle = async (targetChannel: string) => {
    if (saving) return;

    if (targetChannel === "email" && !emailVerified) {
      const current = channels.find((ch) => ch.channel === "email");
      if (!current?.enabled) return;
    }

    setErrorMsg(null);
    const previous = channels;
    const updated = channels.map((ch) =>
      ch.channel === targetChannel ? { ...ch, enabled: !ch.enabled } : ch
    );
    setChannels(updated);
    setSaving(true);

    try {
      await api.updateDeliveryChannels(updated);
    } catch (err) {
      setChannels(previous);
      const message = err instanceof Error ? err.message : "저장에 실패했습니다.";
      setErrorMsg(message);
    } finally {
      setSaving(false);
    }
  };

  const getChannelFailure = (channel: string): ChannelDeliveryStatus | undefined => {
    return deliveryStatuses.find((s) => s.channel === channel && s.status === "failed");
  };

  const enabledCount = channels.filter((ch) => ch.enabled).length;

  if (loading) {
    return (
      <View style={styles.container}>
        <Text style={styles.loadingText}>채널 정보를 불러오는 중...</Text>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>알림 수신 채널</Text>
        <Text style={styles.subtitle}>
          {enabledCount > 0 ? `${enabledCount}개 활성화됨` : "채널을 선택하세요"}
        </Text>
      </View>

      {channels.map((ch) => {
        const info = CHANNEL_INFO[ch.channel as keyof typeof CHANNEL_INFO];
        if (!info) return null;

        const isEmail = ch.channel === "email";
        const needsVerification = isEmail && !emailVerified;
        const failure = getChannelFailure(ch.channel);

        return (
          <View key={ch.channel} style={styles.channelWrapper}>
            <View
              style={[
                styles.channelRow,
                needsVerification && styles.channelRowWarning,
              ]}
            >
              <Text style={styles.channelIcon}>{info.icon}</Text>
              <View style={styles.channelInfo}>
                <Text style={styles.channelLabel}>{info.label}</Text>
                <Text style={styles.channelDesc}>{info.description}</Text>
              </View>
              <Switch
                value={needsVerification ? false : ch.enabled}
                onValueChange={() => handleToggle(ch.channel)}
                disabled={saving || needsVerification}
                trackColor={{ false: "rgba(35,24,21,0.2)", true: "#43b9d6" }}
                thumbColor="#fff"
              />
            </View>

            {failure && ch.enabled && (
              <View style={styles.warningBox}>
                <Text style={styles.warningText}>
                  {failure.retry_count >= MAX_RETRIES
                    ? "이 채널로의 전달이 실패했습니다. 채널 설정을 확인해주세요."
                    : `전달이 실패하여 자동 재시도 중입니다. (${failure.retry_count}/${MAX_RETRIES}회 시도)`}
                </Text>
              </View>
            )}

            {needsVerification && (
              <View style={styles.warningBox}>
                <Text style={styles.warningText}>
                  이메일 수신을 활성화하려면 이메일 인증이 필요합니다.{" "}
                  <Text
                    style={styles.link}
                    onPress={() => navigation.navigate("EmailVerification", {})}
                  >
                    여기를 클릭하여 이메일을 설정하세요
                  </Text>
                </Text>
              </View>
            )}

            {isEmail && emailVerified && ch.enabled && (
              <View style={styles.infoBox}>
                <Text style={styles.infoText}>
                  현재 등록된 이메일: {user?.email}{" "}
                  <Text
                    style={styles.link}
                    onPress={() => navigation.navigate("EmailVerification", {})}
                  >
                    변경하기
                  </Text>
                </Text>
              </View>
            )}
          </View>
        );
      })}

      {errorMsg && (
        <View style={styles.errorBox}>
          <Text style={styles.errorText}>{errorMsg}</Text>
        </View>
      )}

      <Text style={styles.footer}>선택한 채널로 매일 아침 7시에 소식이 전달됩니다</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    borderLeftWidth: 3,
    borderLeftColor: "#43b9d6",
    paddingLeft: 16,
  },
  loadingText: {
    fontSize: 13,
    color: "rgba(35,24,21,0.5)",
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
  channelWrapper: {
    marginBottom: 8,
  },
  channelRow: {
    flexDirection: "row",
    alignItems: "center",
    padding: 14,
    borderRadius: 12,
    backgroundColor: "#fff",
    borderWidth: 2,
    borderColor: "#231815",
  },
  channelRowWarning: {
    borderColor: "rgba(255,84,66,0.4)",
  },
  channelIcon: {
    fontSize: 22,
    marginRight: 10,
  },
  channelInfo: {
    flex: 1,
  },
  channelLabel: {
    fontSize: 13,
    fontWeight: "600",
    color: "#231815",
  },
  channelDesc: {
    fontSize: 11,
    color: "rgba(35,24,21,0.5)",
    marginTop: 2,
  },
  warningBox: {
    marginTop: 6,
    marginLeft: 12,
    backgroundColor: "rgba(255,84,66,0.1)",
    borderRadius: 8,
    paddingHorizontal: 10,
    paddingVertical: 8,
    borderWidth: 1,
    borderColor: "rgba(255,84,66,0.2)",
  },
  warningText: {
    fontSize: 11,
    color: "#ff5442",
  },
  infoBox: {
    marginTop: 6,
    marginLeft: 12,
  },
  infoText: {
    fontSize: 11,
    color: "rgba(35,24,21,0.5)",
  },
  link: {
    color: "#008fb2",
    textDecorationLine: "underline",
  },
  errorBox: {
    marginTop: 12,
    backgroundColor: "rgba(255,84,66,0.1)",
    borderRadius: 8,
    paddingHorizontal: 10,
    paddingVertical: 8,
    borderWidth: 1,
    borderColor: "rgba(255,84,66,0.2)",
  },
  errorText: {
    fontSize: 11,
    color: "#ff5442",
  },
  footer: {
    marginTop: 12,
    fontSize: 11,
    color: "rgba(35,24,21,0.5)",
    textAlign: "center",
  },
});
