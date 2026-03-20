import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import {
  getDeliveryChannels,
  updateDeliveryChannels,
  getDeliveryStatus,
  type ChannelPreference,
  type ChannelDeliveryStatus,
} from "@/lib/api";

const CHANNEL_INFO = {
  email: { label: "이메일", icon: "📧", description: "이메일로 소식을 받아요" },
};

const CHANNEL_ORDER = ["email"];
const MAX_RETRIES = 3;

export function ChannelPreferencesSection() {
  const { user } = useAuth();
  const [channels, setChannels] = useState<ChannelPreference[]>([]);
  const [deliveryStatuses, setDeliveryStatuses] = useState<ChannelDeliveryStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const emailVerified = user?.email_verified ?? false;

  useEffect(() => {
    getDeliveryChannels()
      .then((data) => {
        const channelMap = new Map(data.map((ch) => [ch.channel, ch.enabled]));
        const allChannels = CHANNEL_ORDER.map((channel) => ({
          channel,
          enabled: channelMap.get(channel) ?? false,
        }));
        setChannels(allChannels);
      })
      .catch((err) => {
        console.error("채널 정보 조회 실패:", err);
        setChannels(CHANNEL_ORDER.map((channel) => ({ channel, enabled: false })));
      })
      .finally(() => setLoading(false));

    getDeliveryStatus()
      .then(setDeliveryStatuses)
      .catch(() => {});
  }, []);

  const handleToggle = async (targetChannel: string) => {
    if (saving) return;

    // Block email toggle activation when not verified
    if (targetChannel === "email" && !emailVerified) {
      const current = channels.find((ch) => ch.channel === "email");
      if (!current?.enabled) return; // trying to enable — blocked
    }

    setErrorMsg(null);
    const previous = channels;

    const updated = channels.map((ch) =>
      ch.channel === targetChannel ? { ...ch, enabled: !ch.enabled } : ch
    );
    setChannels(updated);
    setSaving(true);

    try {
      await updateDeliveryChannels(updated);
    } catch (err: unknown) {
      console.error("채널 설정 저장 실패:", err);
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
      <section className="rounded-2xl bg-[#f0f7ff] border border-[#d4e6f5] p-6">
        <p className="text-sm text-[#6b8db5]">채널 정보를 불러오는 중...</p>
      </section>
    );
  }

  return (
    <section className="rounded-2xl bg-[#f0f7ff] border border-[#d4e6f5] p-6">
      <div className="flex items-center gap-2 mb-5">
        <div className="w-8 h-8 rounded-lg bg-[#ff5442]/10 flex items-center justify-center">
          <svg className="w-4 h-4 text-[#ff5442]" viewBox="0 0 24 24" fill="none"
            stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/>
          </svg>
        </div>
        <h2 className="font-semibold text-[#1e3a5f]">알림 수신 채널</h2>
        <span className="ml-auto text-xs text-[#6b8db5]">
          {enabledCount > 0 ? `${enabledCount}개 활성화됨` : "채널을 선택하세요"}
        </span>
      </div>

      <div className="space-y-3">
        {channels.map((ch) => {
          const info = CHANNEL_INFO[ch.channel as keyof typeof CHANNEL_INFO];
          if (!info) return null;

          const isEmail = ch.channel === "email";
          const needsVerification = isEmail && !emailVerified;
          const failure = getChannelFailure(ch.channel);

          return (
            <div key={ch.channel}>
              <div
                className={`flex items-center justify-between p-4 rounded-xl bg-white border transition-colors ${
                  needsVerification ? "border-[#ff5442]/40" : "border-[#d4e6f5]"
                }`}
              >
                <div className="flex items-center gap-3">
                  <span className="text-2xl">{info.icon}</span>
                  <div>
                    <p className="text-sm font-medium text-[#1e3a5f]">{info.label}</p>
                    <p className="text-xs text-[#6b8db5] mt-0.5">{info.description}</p>
                  </div>
                </div>

                <button
                  onClick={() => handleToggle(ch.channel)}
                  disabled={saving || needsVerification}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    needsVerification
                      ? "bg-[#ff5442]/30 cursor-not-allowed"
                      : ch.enabled
                        ? "bg-[#26b0ff]"
                        : "bg-[#d4e6f5]"
                  } ${saving ? "opacity-50 cursor-not-allowed" : !needsVerification ? "cursor-pointer" : ""}`}
                  aria-label={`${info.label} ${ch.enabled ? "비활성화" : "활성화"}`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                      ch.enabled && !needsVerification ? "translate-x-6" : "translate-x-1"
                    }`}
                  />
                </button>
              </div>

              {/* Delivery failure warning */}
              {failure && ch.enabled && (
                <div className="mt-2 ml-4 text-xs text-[#ff5442] bg-[#ff5442]/10 rounded-lg px-3 py-2 border border-[#ff5442]/20">
                  {failure.retry_count >= MAX_RETRIES ? (
                    <p>이 채널로의 전달이 실패했습니다. 채널 설정을 확인해주세요.</p>
                  ) : (
                    <p>전달이 실패하여 자동 재시도 중입니다. ({failure.retry_count}/{MAX_RETRIES}회 시도)</p>
                  )}
                </div>
              )}

              {/* Email verification required */}
              {needsVerification && (
                <div className="mt-2 ml-4 text-xs text-[#ff5442] bg-[#ff5442]/10 rounded-lg px-3 py-2 border border-[#ff5442]/20">
                  <p>
                    이메일 수신을 활성화하려면 이메일 인증이 필요합니다.{" "}
                    <Link
                      to="/email-verification"
                      className="underline font-medium hover:text-[#e63a2e] transition-colors"
                    >
                      여기를 클릭하여 이메일을 설정하세요
                    </Link>
                  </p>
                </div>
              )}

              {/* Verified email info */}
              {isEmail && emailVerified && ch.enabled && (
                <div className="mt-2 ml-4 text-xs text-[#6b8db5]">
                  <p>
                    현재 등록된 이메일: {user?.email}{" "}
                    <Link to="/email-verification" className="underline hover:text-[#1e3a5f] transition-colors">
                      변경하기
                    </Link>
                  </p>
                </div>
              )}
            </div>
          );  
        })}
      </div>

      {errorMsg && (
        <p className="mt-4 text-xs text-[#ff5442] bg-[#ff5442]/10 rounded-lg px-3 py-2 border border-[#ff5442]/20">
          {errorMsg}
        </p>
      )}

      <p className="mt-4 text-xs text-[#6b8db5] text-center">
        선택한 채널로 매일 아침 7시에 소식이 전달됩니다
      </p>
    </section>
  );
}
