import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import {
  adminSearchUser,
  adminAdjustCoins,
  type AdminUserSearchResult,
} from "@/lib/api";
import { formatDateTime } from "@/lib/utils";

export function AdminCoinsPage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();

  const [searchType, setSearchType] = useState<"id" | "email">("email");
  const [query, setQuery] = useState("");
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [result, setResult] = useState<AdminUserSearchResult | null>(null);

  const [newCoins, setNewCoins] = useState("");
  const [memo, setMemo] = useState("");
  const [adjusting, setAdjusting] = useState(false);
  const [adjustError, setAdjustError] = useState<string | null>(null);

  useEffect(() => {
    if (authLoading) return;
    if (!user) { navigate("/", { replace: true }); return; }
    if (user.role !== "admin") { navigate("/", { replace: true }); return; }
  }, [user, authLoading, navigate]);

  const handleSearch = async () => {
    const q = query.trim();
    if (!q) return;
    setSearching(true);
    setSearchError(null);
    setResult(null);
    setNewCoins("");
    setMemo("");
    setAdjustError(null);
    try {
      const data = await adminSearchUser(searchType, q);
      setResult(data);
      setNewCoins(String(data.level.total_coins));
    } catch (e) {
      setSearchError(e instanceof Error ? e.message : "검색 실패");
    } finally {
      setSearching(false);
    }
  };

  const handleAdjust = async () => {
    if (!result) return;

    const trimmedMemo = memo.trim();
    if (!trimmedMemo) {
      setAdjustError("비고(사유)는 필수입니다");
      return;
    }

    const coins = parseInt(newCoins, 10);
    if (isNaN(coins) || coins < 0) {
      setAdjustError("포인트은 0 이상의 정수여야 합니다");
      return;
    }

    const confirmed = window.confirm(
      "사용자의 포인트 보유량을 직접적으로 수정하는 것은 매우 신중한 결정이 필요합니다.\n\n" +
      "해당 관리자는 이로 인해 금전적 분쟁이 발생할 수 있음을 인지하는 것에 동의하는 것으로 간주되며, " +
      "모든 수정은 관리자의 고유 번호와 함께 기록됩니다.\n\n" +
      `대상: ${result.user.nickname || result.user.email || result.user.id}\n` +
      `현재 포인트: ${result.level.total_coins.toLocaleString()}\n` +
      `변경 후: ${coins.toLocaleString()}\n` +
      `차이: ${(coins - result.level.total_coins) >= 0 ? "+" : ""}${(coins - result.level.total_coins).toLocaleString()}\n` +
      `사유: ${trimmedMemo}\n\n` +
      "정말 진행하시겠습니까?"
    );

    if (!confirmed) return;

    setAdjusting(true);
    setAdjustError(null);
    try {
      const adjustResult = await adminAdjustCoins(result.user.id, coins, trimmedMemo);
      setResult({
        user: result.user,
        level: adjustResult.level,
      });
      setNewCoins(String(adjustResult.new_coins));
      setMemo("");
      alert(
        `포인트이 수정되었습니다.\n\n` +
        `변동: ${adjustResult.delta >= 0 ? "+" : ""}${adjustResult.delta.toLocaleString()}\n` +
        `현재 보유량: ${adjustResult.new_coins.toLocaleString()}`
      );
    } catch (e) {
      setAdjustError(e instanceof Error ? e.message : "포인트 수정 실패");
    } finally {
      setAdjusting(false);
    }
  };

  if (authLoading || !user || user.role !== "admin") {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ backgroundColor: "var(--color-bg)" }}>
        <p className="text-[#6b8db5]">로딩 중...</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex flex-col" style={{ backgroundColor: "var(--color-bg)", color: "var(--color-fg)" }}>
      <header
        className="sticky top-0 z-10 border-b bg-opacity-90 backdrop-blur-lg"
        style={{ borderColor: "var(--color-border)", backgroundColor: "var(--color-bg)" }}
      >
        <div className="max-w-2xl mx-auto px-6 h-16 flex items-center justify-between">
          <button onClick={() => navigate("/admin")} className="text-sm text-[#6b8db5] hover:text-[#1e3a5f] transition-colors">
            &larr; 관리자 페이지
          </button>
          <h1 className="text-lg font-bold text-[#1e3a5f]">포인트 수정</h1>
          <div className="w-24" />
        </div>
      </header>

      <main className="flex-1 max-w-2xl w-full mx-auto px-6 py-8 space-y-6">
        {/* Search */}
        <section className="rounded-2xl border border-[#d4e6f5] bg-[#f0f7ff] p-6 space-y-4">
          <h2 className="text-lg font-semibold text-[#1e3a5f]">유저 검색</h2>
          <div className="flex gap-2">
            <select
              value={searchType}
              onChange={(e) => setSearchType(e.target.value as "id" | "email")}
              className="bg-white border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
            >
              <option value="email">이메일</option>
              <option value="id">UUID</option>
            </select>
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSearch()}
              placeholder={searchType === "email" ? "user@example.com" : "유저 UUID"}
              className="flex-1 bg-white border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
            />
            <button
              onClick={handleSearch}
              disabled={searching || !query.trim()}
              className="px-5 py-2 rounded-lg text-sm font-semibold transition-colors disabled:opacity-50"
              style={{ background: "var(--color-button-primary)", color: "white" }}
            >
              {searching ? "검색 중..." : "검색"}
            </button>
          </div>
          {searchError && (
            <p className="text-sm text-[#ff5442]">{searchError}</p>
          )}
        </section>

        {/* User info + adjustment */}
        {result && (
          <>
            <section className="rounded-2xl border border-[#d4e6f5] bg-[#f0f7ff] p-6 space-y-4">
              <h2 className="text-lg font-semibold text-[#1e3a5f]">유저 정보</h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                <div>
                  <span className="text-[#6b8db5]">ID: </span>
                  <span className="text-[#1e3a5f] font-mono text-xs break-all">{result.user.id}</span>
                </div>
                <div>
                  <span className="text-[#6b8db5]">닉네임: </span>
                  <span className="text-[#1e3a5f]">{result.user.nickname || "-"}</span>
                </div>
                <div>
                  <span className="text-[#6b8db5]">이메일: </span>
                  <span className="text-[#1e3a5f]">{result.user.email || "-"}</span>
                  {result.user.email_verified && (
                    <span className="ml-1 text-xs text-green-600">(인증됨)</span>
                  )}
                </div>
                <div>
                  <span className="text-[#6b8db5]">역할: </span>
                  <span className="text-[#1e3a5f]">{result.user.role}</span>
                </div>
                <div>
                  <span className="text-[#6b8db5]">가입일: </span>
                  <span className="text-[#1e3a5f]">{formatDateTime(result.user.created_at)}</span>
                </div>
                <div>
                  <span className="text-[#6b8db5]">카카오 ID: </span>
                  <span className="text-[#1e3a5f]">{result.user.kakao_id}</span>
                </div>
              </div>

              <div className="border-t border-[#d4e6f5] pt-4 mt-4">
                <div className="flex items-center gap-4">
                  <div>
                    <p className="text-xs text-[#6b8db5]">현재 포인트</p>
                    <p className="text-2xl font-bold text-[#1e3a5f]">
                      {result.level.total_coins.toLocaleString()}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs text-[#6b8db5]">레벨</p>
                    <p className="text-2xl font-bold text-[#4a9fe5]">
                      Lv.{result.level.level}
                    </p>
                  </div>
                </div>
              </div>
            </section>

            <section className="rounded-2xl border border-[#ff5442]/30 bg-[#ff5442]/5 p-6 space-y-4">
              <h2 className="text-lg font-semibold text-[#ff5442]">포인트 보유량 수정</h2>
              <p className="text-xs text-[#6b8db5]">
                이 작업은 최후의 수단입니다. 모든 수정은 관리자 ID와 함께 영구적으로 기록됩니다.
              </p>

              <div className="space-y-3">
                <div>
                  <label className="text-xs text-[#6b8db5] mb-1 block">변경할 포인트 값</label>
                  <input
                    type="number"
                    value={newCoins}
                    onChange={(e) => setNewCoins(e.target.value)}
                    min={0}
                    className="w-full bg-white border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f]"
                    placeholder="새로운 포인트 보유량"
                  />
                  {newCoins !== "" && !isNaN(parseInt(newCoins)) && (
                    <p className="text-xs mt-1">
                      <span className="text-[#6b8db5]">차이: </span>
                      <span className={parseInt(newCoins) - result.level.total_coins >= 0 ? "text-green-600 font-semibold" : "text-[#ff5442] font-semibold"}>
                        {parseInt(newCoins) - result.level.total_coins >= 0 ? "+" : ""}
                        {(parseInt(newCoins) - result.level.total_coins).toLocaleString()}
                      </span>
                    </p>
                  )}
                </div>
                <div>
                  <label className="text-xs text-[#6b8db5] mb-1 block">비고 (필수)</label>
                  <textarea
                    value={memo}
                    onChange={(e) => setMemo(e.target.value)}
                    rows={2}
                    className="w-full bg-white border border-[#d4e6f5] rounded-lg px-3 py-2 text-sm text-[#1e3a5f] resize-none"
                    placeholder="수정 사유를 반드시 입력하세요 (예: 장애 보상, 오류 수정 등)"
                  />
                </div>
                {adjustError && (
                  <p className="text-sm text-[#ff5442]">{adjustError}</p>
                )}
                <button
                  onClick={handleAdjust}
                  disabled={adjusting || !memo.trim() || newCoins === ""}
                  className="w-full py-3 rounded-xl font-semibold text-sm transition-colors disabled:opacity-50 bg-[#ff5442] text-white hover:bg-[#e04030]"
                >
                  {adjusting ? "수정 중..." : "포인트 수정 실행"}
                </button>
              </div>
            </section>
          </>
        )}
      </main>
    </div>
  );
}
