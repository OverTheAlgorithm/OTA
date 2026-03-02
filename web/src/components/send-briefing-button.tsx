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
    <section className="rounded-2xl bg-[#f0f7ff] border border-[#d4e6f5] p-6">
      <div className="flex items-center gap-2 mb-4">
        <div className="w-8 h-8 rounded-lg bg-[#4a9fe5]/10 flex items-center justify-center">
          <svg
            className="w-4 h-4 text-[#4a9fe5]"
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
        <h2 className="font-semibold text-[#1e3a5f]">지금 브리핑 받기</h2>
      </div>

      <button
        onClick={handleSend}
        disabled={isSending}
        className={[
          "w-full py-3 rounded-xl font-semibold text-sm transition-all",
          isSending
            ? "bg-[#d4e6f5] text-[#6b8db5] cursor-not-allowed"
            : "bg-[#26b0ff] hover:bg-[#1a9fed] active:bg-[#0e8ee0] text-white cursor-pointer",
        ].join(" ")}
      >
        {isSending ? "전송 중..." : "브리핑 보내기"}
      </button>

      {state.type === "success" && (
        <p className="mt-3 text-xs text-center text-green-600 bg-green-100 border border-green-300 rounded-lg px-3 py-2">
          브리핑이 전송되었습니다!
        </p>
      )}
      {state.type === "skipped" && (
        <p className="mt-3 text-xs text-center text-[#6b8db5] bg-[#d4e6f5]/50 border border-[#d4e6f5] rounded-lg px-3 py-2">
          이미 오늘의 브리핑이 전송되었습니다.
        </p>
      )}
      {state.type === "error" && (
        <p className="mt-3 text-xs text-center text-[#ff5442] bg-[#ff5442]/10 border border-[#ff5442]/20 rounded-lg px-3 py-2">
          {state.message}
        </p>
      )}

      <p className="mt-3 text-xs text-[#6b8db5] text-center leading-relaxed">
        활성화된 채널로 최신 브리핑을 즉시 전송합니다.
        <br />
        매일 아침 7시에 자동 전송되는 것과 동일한 내용입니다.
      </p>
    </section>
  );
}
