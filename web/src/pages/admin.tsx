import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { triggerCollection } from "@/lib/api";

type CollectState =
  | { status: "idle" }
  | { status: "running" }
  | { status: "requested" }
  | { status: "error"; message: string };

export function AdminPage() {
  const { user, loading } = useAuth();
  const navigate = useNavigate();
  const [collectState, setCollectState] = useState<CollectState>({ status: "idle" });

  useEffect(() => {
    if (loading) return;
    if (!user) { navigate("/", { replace: true }); return; }
    if (user.role !== "admin") { navigate("/home", { replace: true }); return; }
  }, [user, loading, navigate]);

  if (loading || !user || user.role !== "admin") {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#0f0a19]">
        <p className="text-[#9b8bb4]">로딩 중...</p>
      </div>
    );
  }

  const handleCollect = async () => {
    setCollectState({ status: "running" });
    try {
      await triggerCollection();
      setCollectState({ status: "requested" });
    } catch (e) {
      setCollectState({ status: "error", message: e instanceof Error ? e.message : "알 수 없는 오류" });
    }
  };

  const isDisabled = collectState.status === "running" || collectState.status === "requested";

  return (
    <div className="min-h-screen bg-[#0f0a19] text-[#f5f0ff] p-8">
      <div className="max-w-xl mx-auto space-y-8">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">관리자 페이지</h1>
          <button
            onClick={() => navigate("/home")}
            className="text-sm text-[#9b8bb4] hover:text-[#f5f0ff] transition-colors"
          >
            ← 홈으로
          </button>
        </div>

        <section className="rounded-2xl border border-[#2d1f42] bg-[#1a1229] p-6 space-y-4">
          <h2 className="text-lg font-semibold">데이터 수집</h2>
          <p className="text-sm text-[#9b8bb4]">
            AI를 통해 오늘의 한국 트렌드를 즉시 수집합니다. 이 작업은 1시간까지 소요될 수 있습니다.
          </p>

          <button
            onClick={handleCollect}
            disabled={isDisabled}
            className="w-full py-3 rounded-xl font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            style={{ background: "#2d1f42", color: "#f5f0ff" }}
          >
            {collectState.status === "running" && "수집 중..."}
            {collectState.status === "requested" && "수집 요청 완료"}
            {(collectState.status === "idle" || collectState.status === "error") && "수집 실행"}
          </button>

          {collectState.status === "requested" && (
            <p className="text-xs text-[#9b8bb4]">작업 완료시 슬랙으로 통지합니다.</p>
          )}

          {collectState.status === "error" && (
            <div className="rounded-xl border border-red-900 bg-[#0f0a19] p-4 text-sm">
              <p className="text-red-400 font-semibold">수집 실패</p>
              <p className="text-[#9b8bb4] mt-1">{collectState.message}</p>
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
