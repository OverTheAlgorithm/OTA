import { useEffect, useState, useCallback } from "react";
import type { Comment, CommentSortOrder, CommentTargetType } from "@/lib/api";
import { listComments, createComment } from "@/lib/api";
import { useAuth } from "@/contexts/auth-context";
import { CommentComposer } from "./CommentComposer";
import { RootThread } from "./ReplyList";
import { useCommentsCache } from "./useCommentsCache";
import { NicknameWarningModal } from "./NicknameWarningModal";

interface CommentSectionProps {
  targetType: CommentTargetType;
  targetId: string;
  className?: string;
}

const PAGE_SIZE = 10;

export function CommentSection({ targetType, targetId, className }: CommentSectionProps) {
  const { user } = useAuth();
  const callerUserId = user?.id ?? null;
  const callerRole = user?.role ?? null;

  const [roots, setRoots] = useState<Comment[]>([]);
  const [nextCursor, setNextCursor] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sort, setSort] = useState<CommentSortOrder>("popular");
  const cache = useCommentsCache();

  // The modal is gated on the server's nickname_state and shown only on the
  // first composer focus while state === "default". After either button is
  // clicked the state advances to "acknowledged" (or "custom" if the user
  // saves a new nickname in mypage), so the modal will not reopen on the
  // next focus. The local `nicknameModalOpen` flag is just transient UI
  // state for the modal mount.
  const [nicknameModalOpen, setNicknameModalOpen] = useState(false);
  const showNicknameWarning = user?.nickname_state === "default";

  const fetchPage = useCallback(
    async (reset: boolean, cursor: string, sortValue: CommentSortOrder) => {
      setLoading(true);
      setError(null);
      try {
        const page = await listComments({
          target_type: targetType,
          target_id: targetId,
          sort: sortValue,
          cursor: cursor || undefined,
          limit: PAGE_SIZE,
        });
        setRoots((prev) => (reset ? page.items : [...prev, ...page.items]));
        setNextCursor(page.next_cursor);
      } catch (err) {
        setError(err instanceof Error ? err.message : "댓글 조회 실패");
      } finally {
        setLoading(false);
      }
    },
    [targetType, targetId],
  );

  useEffect(() => {
    void fetchPage(true, "", sort);
  }, [fetchPage, sort]);

  const handleSubmitRoot = useCallback(
    async (content: string) => {
      const created = await createComment({
        target_type: targetType,
        target_id: targetId,
        content,
      });
      // Prepend so the new comment is visible without an extra round-trip.
      setRoots((prev) => [created, ...prev]);
    },
    [targetType, targetId],
  );

  const updateRoot = useCallback((updated: Comment) => {
    setRoots((prev) => prev.map((r) => (r.id === updated.id ? updated : r)));
  }, []);

  const removeRoot = useCallback((id: string) => {
    setRoots((prev) =>
      prev.map((r) =>
        r.id === id
          ? { ...r, is_deleted: true, content: "", author: { id: "", display_name: "" } }
          : r,
      ),
    );
  }, []);

  return (
    <section className={`space-y-4 ${className ?? ""}`} data-testid="comment-section">
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold text-gray-900">댓글</h2>
        <div className="flex items-center gap-1 text-xs">
          <SortToggle current={sort} value="popular" label="인기순" onChange={setSort} />
          <SortToggle current={sort} value="recent" label="최신순" onChange={setSort} />
        </div>
      </div>

      {callerUserId ? (
        <CommentComposer
          onSubmit={handleSubmitRoot}
          onFirstFocus={() => {
            if (showNicknameWarning) setNicknameModalOpen(true);
          }}
        />
      ) : (
        <div
          className="rounded-md border border-dashed border-gray-200 bg-gray-50 p-4 text-center text-sm text-gray-600"
          data-testid="comment-login-prompt"
        >
          댓글을 작성하려면 로그인이 필요합니다.
        </div>
      )}

      <div className="divide-y divide-gray-100">
        {roots.map((root) => (
          <div key={root.id} className="py-2" data-testid="comment-root">
            <RootThread
              root={root}
              callerUserId={callerUserId}
              callerRole={callerRole}
              cache={cache}
              targetType={targetType}
              targetId={targetId}
              onRootUpdated={updateRoot}
              onRootDeleted={removeRoot}
            />
          </div>
        ))}
      </div>

      {error ? (
        <div className="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700" data-testid="comment-error">
          {error}
        </div>
      ) : null}

      {nextCursor ? (
        <div className="text-center">
          <button
            type="button"
            onClick={() => fetchPage(false, nextCursor, sort)}
            disabled={loading}
            className="rounded-full border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
            data-testid="comment-load-more"
          >
            {loading ? "..." : "더 보기"}
          </button>
        </div>
      ) : null}

      {!loading && roots.length === 0 && !error ? (
        <p className="py-8 text-center text-sm text-gray-400">아직 댓글이 없습니다.</p>
      ) : null}

      <NicknameWarningModal open={nicknameModalOpen} onClose={() => setNicknameModalOpen(false)} />
    </section>
  );
}

function SortToggle({
  current,
  value,
  label,
  onChange,
}: {
  current: CommentSortOrder;
  value: CommentSortOrder;
  label: string;
  onChange: (v: CommentSortOrder) => void;
}) {
  const active = current === value;
  return (
    <button
      type="button"
      onClick={() => onChange(value)}
      className={`rounded-full px-2.5 py-1 transition-colors ${
        active ? "bg-gray-900 text-white" : "text-gray-600 hover:bg-gray-100"
      }`}
      data-testid={`comment-sort-${value}`}
      aria-pressed={active}
    >
      {label}
    </button>
  );
}
