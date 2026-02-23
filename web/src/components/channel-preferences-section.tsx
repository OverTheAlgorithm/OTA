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
  email: { label: "이메일", icon: "📧", description: "이메일로 맥락을 받아요" },
  kakao: { label: "카카오톡", icon: "💬", description: "카카오톡으로 맥락을 받아요 (준비중)" },
};

const CHANNEL_ORDER = ["email", "kakao"];
const MAX_RETRIES = 3;

export function ChannelPreferencesSection() {
  const { user } = useAuth();
  const [channels, setChannels] = useState<ChannelPreference[]>([]);
  const [deliveryStatuses, setDeliveryStatuses] = useState<ChannelDeliveryStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

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

    setErrorMsg(null);
    const previous = channels;

    const updated = channels.map((ch) =>
      ch.channel === targetChannel ? { ...ch, enabled: !ch.enabled } : ch
    );
    setChannels(updated);
    setSaving(true);

    try {
      await updateDeliveryChannels(updated);
    } catch (err) {
      console.error("채널 설정 저장 실패:", err);
      setChannels(previous);
      setErrorMsg("저장에 실패했습니다. 다시 시도해주세요.");
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
      <section className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] p-6">
        <p className="text-sm text-[#9b8bb4]">채널 정보를 불러오는 중...</p>
      </section>
    );
  }

  return (
    <section className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] p-6">
      <div className="flex items-center gap-2 mb-5">
        <div className="w-8 h-8 rounded-lg bg-[#e84d3d]/10 flex items-center justify-center">
          <svg className="w-4 h-4 text-[#e84d3d]" viewBox="0 0 24 24" fill="none"
            stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/>
          </svg>
        </div>
        <h2 className="font-semibold text-[#f5f0ff]">알림 수신 채널</h2>
        <span className="ml-auto text-xs text-[#9b8bb4]">
          {enabledCount > 0 ? `${enabledCount}개 활성화됨` : "채널을 선택하세요"}
        </span>
      </div>

      <div className="space-y-3">
        {channels.map((ch) => {
          const info = CHANNEL_INFO[ch.channel as keyof typeof CHANNEL_INFO];
          if (!info) return null;

          const isEmailChannel = ch.channel === "email";
          const hasEmail = user?.email;
          const showEmailWarning = isEmailChannel && (!hasEmail || ch.enabled);
          const failure = getChannelFailure(ch.channel);

          return (
            <div key={ch.channel}>
              <div className="flex items-center justify-between p-4 rounded-xl bg-[#0f0a19] border border-[#2d1f42]">
                <div className="flex items-center gap-3">
                  <span className="text-2xl">{info.icon}</span>
                  <div>
                    <p className="text-sm font-medium text-[#f5f0ff]">{info.label}</p>
                    <p className="text-xs text-[#9b8bb4] mt-0.5">{info.description}</p>
                  </div>
                </div>

                <button
                  onClick={() => handleToggle(ch.channel)}
                  disabled={saving}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors
                    ${ch.enabled ? "bg-[#5ba4d9]" : "bg-[#2d1f42]"}
                    ${saving ? "opacity-50 cursor-not-allowed" : "cursor-pointer"}
                  `}
                  aria-label={`${info.label} ${ch.enabled ? "비활성화" : "활성화"}`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform
                      ${ch.enabled ? "translate-x-6" : "translate-x-1"}
                    `}
                  />
                </button>
              </div>

              {/* Per-channel delivery failure warning */}
              {failure && ch.enabled && (
                <div className="mt-2 ml-4 text-xs text-[#e84d3d] bg-[#e84d3d]/10 rounded-lg px-3 py-2 border border-[#e84d3d]/20">
                  {failure.retry_count >= MAX_RETRIES ? (
                    <p>이 채널로의 전달이 실패했습니다. 채널 설정을 확인해주세요.</p>
                  ) : (
                    <p>전달이 실패하여 자동 재시도 중입니다. ({failure.retry_count}/{MAX_RETRIES}회 시도)</p>
                  )}
                </div>
              )}

              {/* Email warning/link */}
              {showEmailWarning && (
                <div className="mt-2 ml-4 text-xs">
                  {!hasEmail ? (
                    <p className="text-[#e84d3d]">
                      이메일 주소가 등록되지 않았습니다.{" "}
                      <Link to="/email-verification" className="underline hover:text-[#f56b5d] transition-colors">
                        이메일 추가하기
                      </Link>
                    </p>
                  ) : (
                    <p className="text-[#9b8bb4]">
                      현재 등록된 이메일: {user.email}{" "}
                      <Link to="/email-verification" className="underline hover:text-white transition-colors">
                        변경하기
                      </Link>
                    </p>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {errorMsg && (
        <p className="mt-4 text-xs text-[#e84d3d] bg-[#e84d3d]/10 rounded-lg px-3 py-2 border border-[#e84d3d]/20">
          {errorMsg}
        </p>
      )}

      <p className="mt-4 text-xs text-[#9b8bb4] text-center">
        선택한 채널로 매일 아침 7시에 맥락이 전달됩니다
      </p>
    </section>
  );
}
