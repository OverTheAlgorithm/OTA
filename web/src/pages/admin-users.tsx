import { useEffect, useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { LoadingState } from "@/components/spinner";
import {
  adminSearchUserByRole,
  adminUpdateUserRole,
  adminGetRoleHistory,
  type User,
  type UserRole,
  type RoleChangeLog,
} from "@/lib/api";
import { formatDateTime } from "@/lib/utils";

const ROLE_LABELS: Record<UserRole, string> = {
  user: "일반",
  editor: "에디터",
  admin: "관리자",
};

export function AdminUsersPage() {
  const { user, loading } = useAuth();
  const navigate = useNavigate();

  const [searchType, setSearchType] = useState<"id" | "email">("email");
  const [query, setQuery] = useState("");
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [target, setTarget] = useState<User | null>(null);
  const [history, setHistory] = useState<RoleChangeLog[]>([]);

  const [newRole, setNewRole] = useState<UserRole>("user");
  const [memo, setMemo] = useState("");
  const [updating, setUpdating] = useState(false);
  const [updateError, setUpdateError] = useState<string | null>(null);

  useEffect(() => {
    if (loading) return;
    if (!user || user.role !== "admin") {
      navigate("/", { replace: true });
    }
  }, [user, loading, navigate]);

  if (loading) return <LoadingState />;

  const handleSearch = async () => {
    const q = query.trim();
    if (!q) return;
    setSearching(true);
    setSearchError(null);
    setTarget(null);
    setHistory([]);
    setUpdateError(null);
    try {
      const u = await adminSearchUserByRole(searchType, q);
      setTarget(u);
      setNewRole((u.role as UserRole) ?? "user");
      const hist = await adminGetRoleHistory(u.id);
      setHistory(hist);
    } catch (err) {
      setSearchError(err instanceof Error ? err.message : "검색 실패");
    } finally {
      setSearching(false);
    }
  };

  const handleUpdate = async () => {
    if (!target) return;
    if (target.id === user?.id) {
      setUpdateError("본인의 권한은 변경할 수 없습니다");
      return;
    }
    const trimmedMemo = memo.trim();
    const confirmed = window.confirm(
      "사용자의 권한을 변경하면 즉시 적용됩니다.\n\n" +
        `대상: ${target.nickname || target.email || target.id}\n` +
        `현재 권한: ${ROLE_LABELS[target.role as UserRole] ?? target.role}\n` +
        `변경 후: ${ROLE_LABELS[newRole]}\n` +
        (trimmedMemo ? `메모: ${trimmedMemo}\n\n` : "\n") +
        "계속하시겠습니까?",
    );
    if (!confirmed) return;

    setUpdating(true);
    setUpdateError(null);
    try {
      const result = await adminUpdateUserRole(target.id, newRole, trimmedMemo);
      const updated: User = { ...target, role: result.after_role as UserRole };
      setTarget(updated);
      setMemo("");
      const hist = await adminGetRoleHistory(target.id);
      setHistory(hist);
      if (result.unchanged) {
        alert("권한이 동일하여 변경 없이 종료되었습니다.");
      } else {
        alert(
          `권한이 ${ROLE_LABELS[result.before_role as UserRole] ?? result.before_role} → ${ROLE_LABELS[result.after_role as UserRole] ?? result.after_role}로 변경되었습니다.`,
        );
      }
    } catch (err) {
      setUpdateError(err instanceof Error ? err.message : "권한 변경 실패");
    } finally {
      setUpdating(false);
    }
  };

  return (
    <div className="min-h-screen flex flex-col bg-[#fdf9ee]">
      <Header />
      <main className="flex-1 max-w-[900px] w-full mx-auto px-6 py-8">
        <header className="mb-6 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-[#231815]">사용자 권한 관리</h1>
            <p className="text-sm text-stone-600 mt-1">에디터/관리자 권한을 부여하거나 해제합니다.</p>
          </div>
          <Link to="/admin" className="text-sm underline text-[#231815]">
            ← 관리자 홈
          </Link>
        </header>

        <section className="bg-white border-2 border-[#231815] rounded-lg p-5 mb-6">
          <h2 className="text-lg font-bold text-[#231815] mb-3">유저 검색</h2>
          <div className="flex flex-wrap gap-2 items-center">
            <select
              value={searchType}
              onChange={(e) => setSearchType(e.target.value as "id" | "email")}
              className="border-2 border-[#231815] rounded px-3 h-10 bg-white"
            >
              <option value="email">이메일</option>
              <option value="id">유저 ID</option>
            </select>
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleSearch();
              }}
              placeholder={searchType === "email" ? "user@example.com" : "UUID"}
              className="flex-1 min-w-[200px] border-2 border-[#231815] rounded px-3 h-10 bg-white"
            />
            <button
              type="button"
              onClick={handleSearch}
              disabled={searching}
              className="px-5 h-10 rounded-full border-2 border-[#231815] bg-[#43b9d6] text-sm font-medium text-[#231815] hover:opacity-80 disabled:opacity-50"
            >
              {searching ? "검색 중..." : "검색"}
            </button>
          </div>
          {searchError && (
            <p className="mt-3 text-sm text-red-700">{searchError}</p>
          )}
        </section>

        {target && (
          <section className="bg-white border-2 border-[#231815] rounded-lg p-5 mb-6">
            <h2 className="text-lg font-bold text-[#231815] mb-3">대상 유저</h2>
            <dl className="grid grid-cols-[120px_1fr] gap-y-2 text-sm">
              <dt className="text-stone-500">ID</dt>
              <dd className="font-mono text-xs">{target.id}</dd>
              <dt className="text-stone-500">닉네임</dt>
              <dd>{target.nickname || "-"}</dd>
              <dt className="text-stone-500">이메일</dt>
              <dd>{target.email || "-"}</dd>
              <dt className="text-stone-500">현재 권한</dt>
              <dd className="font-bold">{ROLE_LABELS[target.role as UserRole] ?? target.role}</dd>
            </dl>

            <div className="mt-5 pt-5 border-t border-[#231815]/30">
              <h3 className="text-sm font-bold text-[#231815] mb-3">권한 변경</h3>
              <div className="flex flex-wrap gap-2 items-center">
                <select
                  value={newRole}
                  onChange={(e) => setNewRole(e.target.value as UserRole)}
                  className="border-2 border-[#231815] rounded px-3 h-10 bg-white"
                >
                  <option value="user">일반</option>
                  <option value="editor">에디터</option>
                  <option value="admin">관리자</option>
                </select>
                <input
                  type="text"
                  value={memo}
                  onChange={(e) => setMemo(e.target.value)}
                  placeholder="메모 (선택)"
                  className="flex-1 min-w-[200px] border-2 border-[#231815] rounded px-3 h-10 bg-white"
                />
                <button
                  type="button"
                  onClick={handleUpdate}
                  disabled={updating}
                  className="px-5 h-10 rounded-full border-2 border-[#231815] bg-[#43b9d6] text-sm font-medium text-[#231815] hover:opacity-80 disabled:opacity-50"
                >
                  {updating ? "변경 중..." : "권한 변경"}
                </button>
              </div>
              {updateError && (
                <p className="mt-3 text-sm text-red-700">{updateError}</p>
              )}
            </div>
          </section>
        )}

        {history.length > 0 && (
          <section className="bg-white border-2 border-[#231815] rounded-lg p-5">
            <h2 className="text-lg font-bold text-[#231815] mb-3">권한 변경 이력</h2>
            <ul className="divide-y divide-[#231815]/20">
              {history.map((entry) => (
                <li key={entry.id} className="py-2 text-sm flex flex-wrap justify-between gap-2">
                  <span>
                    {ROLE_LABELS[entry.before_role as UserRole] ?? entry.before_role} → {ROLE_LABELS[entry.after_role as UserRole] ?? entry.after_role}
                    {entry.memo && <span className="text-stone-500"> · {entry.memo}</span>}
                  </span>
                  <span className="text-xs text-stone-500">
                    {formatDateTime(entry.created_at)}
                    {entry.actor_id && <> · by {entry.actor_id.slice(0, 8)}</>}
                  </span>
                </li>
              ))}
            </ul>
          </section>
        )}
      </main>
      <Footer />
    </div>
  );
}
