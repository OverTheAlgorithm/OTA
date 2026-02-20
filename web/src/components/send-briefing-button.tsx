import { useState } from "react";
import { sendBriefingNow } from "@/lib/api";

type SendState =
  | { type: "idle" }
  | { type: "sending" }
  | { type: "success" }
  | { type: "skipped" }
  | { type: "error"; message: string };

export function SendBriefingButton() {
  const [state, setState] = useState<SendState>({ type: "idle" });

  const handleSend = async () => {
    if (state.type === "sending") return;
    setState({ type: "sending" });

    try {
      const result = await sendBriefingNow();

      if (result.success_count > 0) {
        setState({ type: "success" });
      } else if (result.skipped_count > 0) {
        setState({ type: "skipped" });
      } else {
        setState({ type: "error", message: "전송에 실패했습니다. 채널 설정을 확인해주세요." });
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "전송에 실패했습니다.";
      setState({ type: "error", message });
    }
  };

  const isSending = state.type === "sending";

  return (
    <section className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] p-6">
      <div className="flex items-center gap-2 mb-4">
        <div className="w-8 h-8 rounded-lg bg-[#5ba4d9]/10 flex items-center justify-center">
          <svg
            className="w-4 h-4 text-[#5ba4d9]"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <line x1="22" y1="2" x2="11" y2="13" />
            <polygon points="22 2 15 22 11 13 2 9 22 2" />
          </svg>
        </div>
        <h2 className="font-semibold text-[#f5f0ff]">지금 브리핑 받기</h2>
      </div>

      <button
        onClick={handleSend}
        disabled={isSending}
        className={[
          "w-full py-3 rounded-xl font-semibold text-sm transition-all",
          isSending
            ? "bg-[#2d1f42] text-[#9b8bb4] cursor-not-allowed"
            : "bg-[#5ba4d9] hover:bg-[#4a93c8] active:bg-[#3a83b8] text-white cursor-pointer",
        ].join(" ")}
      >
        {isSending ? "전송 중..." : "브리핑 보내기"}
      </button>

      {state.type === "success" && (
        <p className="mt-3 text-xs text-center text-green-400 bg-green-500/10 border border-green-500/20 rounded-lg px-3 py-2">
          브리핑이 전송되었습니다!
        </p>
      )}
      {state.type === "skipped" && (
        <p className="mt-3 text-xs text-center text-[#9b8bb4] bg-[#2d1f42]/50 border border-[#2d1f42] rounded-lg px-3 py-2">
          이미 오늘의 브리핑이 전송되었습니다.
        </p>
      )}
      {state.type === "error" && (
        <p className="mt-3 text-xs text-center text-[#e84d3d] bg-[#e84d3d]/10 border border-[#e84d3d]/20 rounded-lg px-3 py-2">
          {state.message}
        </p>
      )}

      <p className="mt-3 text-xs text-[#9b8bb4] text-center leading-relaxed">
        활성화된 채널로 최신 브리핑을 즉시 전송합니다.
        <br />
        매일 아침 7시에 자동 전송되는 것과 동일한 내용입니다.
      </p>
    </section>
  );
}
