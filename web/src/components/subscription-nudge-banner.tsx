import { useEffect, useState } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import {
  getDeliveryChannels,
  updateDeliveryChannels,
  type ChannelPreference,
} from "@/lib/api";

const DISMISS_KEY = "nudge_dismiss_until";

function isDismissed(): boolean {
  const raw = localStorage.getItem(DISMISS_KEY);
  if (!raw) return false;
  return Date.now() < Number(raw);
}

function dismissForWeek() {
  const oneWeek = 7 * 24 * 60 * 60 * 1000;
  localStorage.setItem(DISMISS_KEY, String(Date.now() + oneWeek));
}

const HIDDEN_PATHS = ["/email-verification", "/mypage"];

export function SubscriptionNudgeBanner() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();
  const { pathname } = useLocation();

  const [channels, setChannels] = useState<ChannelPreference[] | null>(null);
  const [dismissed, setDismissed] = useState(false);
  const [subscribed, setSubscribed] = useState(false);
  const [subscribing, setSubscribing] = useState(false);
  const [subscribeError, setSubscribeError] = useState(false);
  const [hideWeekChecked, setHideWeekChecked] = useState(false);

  useEffect(() => {
    if (authLoading || !user) return;
    if (isDismissed()) {
      setDismissed(true);
      return;
    }
    getDeliveryChannels()
      .then(setChannels)
      .catch(() => setChannels([]));
  }, [user, authLoading]);

  // Don't render conditions
  if (authLoading || !user || dismissed) return null;
  if (HIDDEN_PATHS.some((p) => pathname.startsWith(p))) return null;
  if (channels === null) return null; // still loading
  const hasEnabledChannel = channels.some((ch) => ch.enabled);
  if (hasEnabledChannel) return null;

  const handleClose = () => {
    if (hideWeekChecked) {
      dismissForWeek();
    }
    setDismissed(true);
  };

  const handleSubscribe = async () => {
    if (subscribing || subscribed) return;

    if (!user.email_verified) {
      navigate("/email-verification?auto_subscribe=true");
      return;
    }

    setSubscribing(true);
    setSubscribeError(false);
    try {
      const updated = channels.map((ch) =>
        ch.channel === "email" ? { ...ch, enabled: true } : ch
      );
      const hasEmail = updated.some((ch) => ch.channel === "email");
      const payload = hasEmail
        ? updated
        : [...updated, { channel: "email", enabled: true }];
      await updateDeliveryChannels(payload);
      setSubscribed(true);
    } catch {
      setSubscribeError(true);
    } finally {
      setSubscribing(false);
    }
  };

  return (
    <div className="max-w-[900px] mx-auto px-6 pt-5">
      <div className="bg-white rounded-2xl border border-black/10 px-6 py-5 relative">
        {/* Close button */}
        <button
          onClick={handleClose}
          className="absolute top-3 right-4 w-8 h-8 flex items-center justify-center rounded-full hover:bg-[#f5f5f5] transition-colors"
          aria-label="닫기"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#231815" strokeWidth="2.5" strokeLinecap="round">
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>

        <div className="flex items-start gap-5 pr-10">
          {/* Icon */}
          <div className="flex-shrink-0 w-14 h-14 flex items-center justify-center rounded-2xl text-3xl">
            📧
          </div>

          {/* Content */}
          <div className="flex-1 min-w-0">
            <h3 className="text-lg md:text-xl font-semibold text-[#000] leading-snug">
              위즈레터를 구독해서 매일 아침 소식을 받아보세요!
            </h3>
            <p className="text-sm md:text-base text-[#000] mt-1.5 leading-relaxed">
              복잡한 소식을 간결하게 요약해서, 매일 아침 7시에 보내드립니다.
              <br className="hidden md:block" />
              출근길에 세상에 흐름을 읽어보세요!
            </p>

            <div className="mt-4">
              <button
                onClick={handleSubscribe}
                disabled={subscribing || subscribed}
                className={`inline-flex items-center justify-center px-6 py-2.5 rounded-full text-sm font-semibold border-[2px] border-[#231815] transition-all ${
                  subscribed
                    ? "bg-[#e8f8ec] text-[#231815] cursor-default"
                    : "bg-[#43b9d6] text-[#231815] hover:brightness-110 disabled:opacity-50"
                }`}
              >
                {subscribed
                  ? "🎉 이제 매일 소식이 도착합니다"
                  : subscribing
                    ? "구독 중..."
                    : "위즈레터 구독하기"}
              </button>
              {subscribeError && (
                <p className="text-xs text-[#ff5442] mt-2">
                  구독 중 오류가 발생했습니다. 다시 시도해주세요.
                </p>
              )}
            </div>
          </div>
        </div>

        {/* Bottom: hide for a week checkbox */}
        <div className="flex justify-end mt-3">
          <label className="flex items-center gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={hideWeekChecked}
              onChange={(e) => {
                setHideWeekChecked(e.target.checked);
              }}
              className="w-4 h-4 rounded border-[#a8a8a8] accent-[#231815]"
            />
            <span className="text-xs text-[#231815] tracking-tight">
              1주일 동안 보지 않기
            </span>
          </label>
        </div>
      </div>
    </div>
  );
}
