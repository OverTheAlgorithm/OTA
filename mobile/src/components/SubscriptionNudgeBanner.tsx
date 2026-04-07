// Extracted from: web/src/components/subscription-nudge-banner.tsx
// Differences: AsyncStorage instead of localStorage, no pathname check (caller controls visibility),
//              Switch instead of checkbox, no useLocation/useNavigate

import { useEffect, useState } from "react";
import { View, Text, Pressable, Switch, StyleSheet } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { useAuth } from "../contexts/auth-context";
import { api } from "../lib/api";
import type { ChannelPreference } from "../lib/api";

const DISMISS_KEY = "nudge_dismiss_until";

async function isDismissed(): Promise<boolean> {
  const raw = await AsyncStorage.getItem(DISMISS_KEY).catch(() => null);
  if (!raw) return false;
  return Date.now() < Number(raw);
}

async function dismissForWeek() {
  const oneWeek = 7 * 24 * 60 * 60 * 1000;
  await AsyncStorage.setItem(DISMISS_KEY, String(Date.now() + oneWeek)).catch(() => {});
}

interface Props {
  onNavigateEmailVerification?: () => void;
}

export function SubscriptionNudgeBanner({ onNavigateEmailVerification }: Props) {
  const { user, loading: authLoading } = useAuth();

  const [channels, setChannels] = useState<ChannelPreference[] | null>(null);
  const [dismissed, setDismissed] = useState(false);
  const [subscribed, setSubscribed] = useState(false);
  const [subscribing, setSubscribing] = useState(false);
  const [subscribeError, setSubscribeError] = useState(false);
  const [hideWeekChecked, setHideWeekChecked] = useState(false);

  useEffect(() => {
    if (authLoading || !user) return;
    isDismissed().then((d) => {
      if (d) {
        setDismissed(true);
        return;
      }
      api.getDeliveryChannels()
        .then(setChannels)
        .catch(() => setChannels([]));
    });
  }, [user, authLoading]);

  if (authLoading || !user || dismissed) return null;
  if (channels === null) return null;
  const hasEnabledChannel = channels.some((ch) => ch.enabled);
  if (hasEnabledChannel) return null;

  const handleClose = async () => {
    if (hideWeekChecked) {
      await dismissForWeek();
    }
    setDismissed(true);
  };

  const handleSubscribe = async () => {
    if (subscribing || subscribed) return;

    if (!user.email_verified) {
      onNavigateEmailVerification?.();
      return;
    }

    setSubscribing(true);
    setSubscribeError(false);
    try {
      const updated = channels.map((ch) =>
        ch.channel === "email" ? { ...ch, enabled: true } : ch,
      );
      const hasEmail = updated.some((ch) => ch.channel === "email");
      const payload = hasEmail
        ? updated
        : [...updated, { channel: "email", enabled: true }];
      await api.updateDeliveryChannels(payload);
      setSubscribed(true);
    } catch {
      setSubscribeError(true);
    } finally {
      setSubscribing(false);
    }
  };

  return (
    <View style={styles.wrapper}>
      <View style={styles.card}>
        {/* Close button */}
        <Pressable onPress={handleClose} style={styles.closeBtn} accessibilityLabel="닫기">
          <Text style={styles.closeBtnText}>✕</Text>
        </Pressable>

        <View style={styles.row}>
          {/* Icon */}
          <View style={styles.iconBox}>
            <Text style={styles.iconEmoji}>📧</Text>
          </View>

          {/* Content */}
          <View style={styles.content}>
            <Text style={styles.title}>위즈레터를 구독해서 매일 아침 소식을 받아보세요!</Text>
            <Text style={styles.body}>
              복잡한 소식을 간결하게 요약해서, 매일 아침 7시에 보내드립니다.
              출근길에 세상의 흐름을 읽어보세요!
            </Text>

            <Pressable
              onPress={handleSubscribe}
              disabled={subscribing || subscribed}
              style={[
                styles.subscribeBtn,
                subscribed ? styles.subscribedBtn : styles.activeSubscribeBtn,
                (subscribing || subscribed) && styles.disabledBtn,
              ]}
            >
              <Text style={styles.subscribeBtnText}>
                {subscribed
                  ? "이제 매일 소식이 도착합니다"
                  : subscribing
                    ? "구독 중..."
                    : "위즈레터 구독하기"}
              </Text>
            </Pressable>

            {subscribeError && (
              <Text style={styles.errorText}>
                구독 중 오류가 발생했습니다. 다시 시도해주세요.
              </Text>
            )}
          </View>
        </View>

        {/* Bottom: hide for a week */}
        <View style={styles.dismissRow}>
          <Switch
            value={hideWeekChecked}
            onValueChange={setHideWeekChecked}
            trackColor={{ true: "#231815" }}
            style={styles.switch}
          />
          <Text style={styles.dismissText}>1주일 동안 보지 않기</Text>
        </View>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  wrapper: {
    paddingHorizontal: 16,
    paddingTop: 16,
  },
  card: {
    backgroundColor: "#fff",
    borderRadius: 16,
    borderWidth: 1,
    borderColor: "rgba(0,0,0,0.1)",
    paddingHorizontal: 16,
    paddingVertical: 16,
    gap: 12,
  },
  closeBtn: {
    position: "absolute",
    top: 12,
    right: 16,
    width: 32,
    height: 32,
    justifyContent: "center",
    alignItems: "center",
  },
  closeBtnText: {
    fontSize: 16,
    color: "#231815",
  },
  row: {
    flexDirection: "row",
    alignItems: "flex-start",
    gap: 16,
    paddingRight: 32,
  },
  iconBox: {
    width: 48,
    height: 48,
    borderRadius: 12,
    backgroundColor: "rgba(67,185,214,0.1)",
    justifyContent: "center",
    alignItems: "center",
    flexShrink: 0,
  },
  iconEmoji: {
    fontSize: 24,
  },
  content: {
    flex: 1,
    gap: 8,
  },
  title: {
    fontSize: 16,
    fontWeight: "600",
    color: "#000",
    lineHeight: 22,
  },
  body: {
    fontSize: 14,
    color: "#000",
    lineHeight: 20,
  },
  subscribeBtn: {
    alignSelf: "flex-start",
    paddingHorizontal: 20,
    paddingVertical: 8,
    borderRadius: 20,
    borderWidth: 2,
    borderColor: "#231815",
  },
  activeSubscribeBtn: {
    backgroundColor: "#43b9d6",
  },
  subscribedBtn: {
    backgroundColor: "#e8f8ec",
  },
  disabledBtn: {
    opacity: 0.5,
  },
  subscribeBtnText: {
    fontSize: 14,
    fontWeight: "600",
    color: "#231815",
  },
  errorText: {
    fontSize: 12,
    color: "#ff5442",
  },
  dismissRow: {
    flexDirection: "row",
    justifyContent: "flex-end",
    alignItems: "center",
    gap: 8,
  },
  switch: {
    transform: [{ scaleX: 0.8 }, { scaleY: 0.8 }],
  },
  dismissText: {
    fontSize: 12,
    color: "#231815",
  },
});
