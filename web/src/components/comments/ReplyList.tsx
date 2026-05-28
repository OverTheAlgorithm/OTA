import { useState, useCallback } from "react";
import type { Comment, CommentTargetType } from "@/lib/api";
import { listReplies, createComment } from "@/lib/api";
import { CommentItem } from "./CommentItem";
import { CommentComposer } from "./CommentComposer";
import type { CommentsCache } from "./useCommentsCache";

interface RootThreadProps {
  root: Comment;
  callerUserId: string | null;
  callerRole: string | null;
  cache: CommentsCache;
  targetType: CommentTargetType;
  targetId: string;
  onRootUpdated: (updated: Comment) => void;
  onRootDeleted: (id: string) => void;
  pageSize?: number;
}

// RootThread renders one root comment plus its lazy-loaded reply thread.
// The composer state lives here so each root manages its own reply
// composer independently — opening a composer under one root does not
// disturb others on the page.
export function RootThread({
  root,
  callerUserId,
  callerRole,
  cache,
  targetType,
  targetId,
  onRootUpdated,
  onRootDeleted,
  pageSize = 10,
}: RootThreadProps) {
  const state = cache.getReplies(root.id);
  const [composerTarget, setComposerTarget] = useState<{ parentId: string; mention: string } | null>(null);

  const loadMore = useCallback(async () => {
    cache.startLoad(root.id);
    try {
      const page = await listReplies({
        group_id: root.id,
        cursor: state.nextCursor || undefined,
        limit: pageSize,
      });
      cache.finishLoad(root.id, page.items, page.next_cursor);
    } catch (err) {
      cache.failLoad(root.id, err instanceof Error ? err.message : "대댓글 조회 실패");
    }
  }, [cache, root.id, state.nextCursor, pageSize]);

  const handleToggle = useCallback(async () => {
    const next = !state.isOpen;
    cache.setOpen(root.id, next);
    if (next && state.loaded.length === 0 && !state.isLoading) {
      await loadMore();
    }
  }, [cache, root.id, state.isOpen, state.loaded.length, state.isLoading, loadMore]);

  const handleReplySubmit = useCallback(
    async (content: string) => {
      if (!composerTarget) return;
      const created = await createComment({
        target_type: targetType,
        target_id: targetId,
        parent_id: composerTarget.parentId,
        content,
      });
      cache.appendReply(root.id, created);
      // Auto-expand the reply list so the user sees their new reply.
      cache.setOpen(root.id, true);
      // Bump root's reply_count in the parent state so the toggle text
      // updates without a full refetch.
      onRootUpdated({ ...root, reply_count: root.reply_count + 1 });
      setComposerTarget(null);
    },
    [composerTarget, targetType, targetId, cache, root, onRootUpdated],
  );

  const openComposerFor = useCallback((parentId: string, mention: string) => {
    setComposerTarget({ parentId, mention });
  }, []);

  const replyCount = root.reply_count;

  return (
    <div data-testid="root-thread">
      <CommentItem
        comment={root}
        callerUserId={callerUserId}
        callerRole={callerRole}
        onCommentChange={onRootUpdated}
        onCommentDeleted={onRootDeleted}
        onReplyClick={() => openComposerFor(root.id, root.author.display_name || "사용자")}
      />

      {composerTarget ? (
        <div className="mt-2 pl-8" data-testid="reply-composer-wrap">
          <CommentComposer
            placeholder={`${composerTarget.mention} 님에게 답글...`}
            onSubmit={handleReplySubmit}
            onCancel={() => setComposerTarget(null)}
            submitLabel="답글"
            autoFocus
          />
        </div>
      ) : null}

      {replyCount > 0 ? (
        <div className="pl-8">
          <button
            type="button"
            onClick={handleToggle}
            className="mt-1 flex items-center gap-1 rounded-full px-2 py-1 text-xs font-semibold text-blue-600 transition-colors hover:bg-blue-50"
            data-testid="reply-toggle"
            aria-expanded={state.isOpen}
          >
            <Chevron open={state.isOpen} />
            <span>답글 {replyCount}개</span>
          </button>
        </div>
      ) : null}

      {state.isOpen ? (
        <div className="space-y-0" data-testid="reply-list">
          {state.loaded.map((r) => (
            <CommentItem
              key={r.id}
              comment={r}
              callerUserId={callerUserId}
              callerRole={callerRole}
              isReply
              onCommentChange={(updated) => cache.updateReply(root.id, updated)}
              onCommentDeleted={(id) => cache.removeReply(root.id, id)}
              onReplyClick={() =>
                openComposerFor(r.id, r.author.display_name || "사용자")
              }
            />
          ))}
          {state.isLoading ? (
            <p className="pl-8 py-2 text-xs text-gray-500">로딩 중...</p>
          ) : null}
          {state.error ? (
            <p className="pl-8 py-2 text-xs text-red-600">{state.error}</p>
          ) : null}
          {state.nextCursor ? (
            <button
              type="button"
              onClick={loadMore}
              className="ml-8 mt-2 rounded-full px-3 py-1 text-xs font-medium text-blue-600 hover:bg-blue-50"
              data-testid="reply-load-more"
            >
              더 보기
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function Chevron({ open }: { open: boolean }) {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{ transform: open ? "rotate(180deg)" : undefined, transition: "transform 150ms" }}
      aria-hidden="true"
    >
      <polyline points="6 9 12 15 18 9" />
    </svg>
  );
}
