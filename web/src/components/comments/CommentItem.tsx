import { useState, useCallback } from "react";
import type { Comment, CommentReaction } from "@/lib/api";
import { reactComment, deleteComment, updateComment } from "@/lib/api";
import { CommentComposer } from "./CommentComposer";
import { formatRelativeTime } from "./formatRelativeTime";

export interface CommentItemProps {
  comment: Comment;
  callerUserId: string | null;
  callerRole: string | null;
  isReply?: boolean;
  onCommentChange?: (updated: Comment) => void;
  onCommentDeleted?: (id: string) => void;
  onReplyClick?: () => void;
  /** Right-aligned reply toggle ("N 댓글" button), rendered only for roots. */
  replyToggle?: React.ReactNode;
}

export function CommentItem({
  comment,
  callerUserId,
  callerRole,
  isReply = false,
  onCommentChange,
  onCommentDeleted,
  onReplyClick,
  replyToggle,
}: CommentItemProps) {
  const isAuthed = Boolean(callerUserId);
  const isOwner = callerUserId !== null && callerUserId === comment.author.id;
  const isAdmin = callerRole === "admin";
  const [isEditing, setIsEditing] = useState(false);
  const [pendingReaction, setPendingReaction] = useState(false);

  const handleReact = useCallback(
    async (target: CommentReaction) => {
      if (!isAuthed || pendingReaction) return;
      const desired: CommentReaction = comment.my_reaction === target ? 0 : target;
      setPendingReaction(true);
      try {
        const res = await reactComment(comment.id, desired);
        onCommentChange?.({
          ...comment,
          likes_count: res.likes_count,
          dislikes_count: res.dislikes_count,
          my_reaction: res.my_reaction,
        });
      } catch (err) {
        console.error("react failed", err);
      } finally {
        setPendingReaction(false);
      }
    },
    [comment, isAuthed, pendingReaction, onCommentChange],
  );

  const handleDelete = useCallback(async () => {
    if (!confirm("이 댓글을 삭제하시겠습니까?")) return;
    try {
      await deleteComment(comment.id);
      onCommentDeleted?.(comment.id);
    } catch (err) {
      console.error("delete failed", err);
      alert("삭제에 실패했습니다");
    }
  }, [comment.id, onCommentDeleted]);

  const handleEditSubmit = useCallback(
    async (content: string) => {
      const updated = await updateComment(comment.id, content);
      onCommentChange?.(updated);
      setIsEditing(false);
    },
    [comment.id, onCommentChange],
  );

  if (comment.is_deleted) {
    return (
      <div className={`${isReply ? "pl-8" : ""} py-3`} data-testid="comment-item-deleted">
        <p className="text-sm italic text-gray-400">[삭제된 댓글입니다]</p>
      </div>
    );
  }

  const display = comment.author.display_name || "사용자";

  return (
    <div className={`${isReply ? "pl-8" : ""} py-3`} data-testid="comment-item">
      <div className="flex">
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline gap-2 text-sm">
            <span className="font-medium text-gray-900" data-testid="comment-author">
              {display}
            </span>
            <span className="text-xs text-gray-500">{formatRelativeTime(comment.created_at)}</span>
            {comment.edited_at ? <span className="text-xs text-gray-400">(수정됨)</span> : null}
          </div>

          {isEditing ? (
            <div className="mt-2">
              <CommentComposer
                initialValue={comment.content}
                onSubmit={handleEditSubmit}
                onCancel={() => setIsEditing(false)}
                submitLabel="저장"
                autoFocus
              />
            </div>
          ) : (
            <p className="mt-1 whitespace-pre-wrap break-words text-sm leading-relaxed text-gray-900">
              {comment.content}
            </p>
          )}

          <div className="mt-2 flex items-center gap-2 text-xs text-gray-600">
            <button
              type="button"
              onClick={() => handleReact(1)}
              disabled={!isAuthed || pendingReaction}
              className={`flex items-center gap-1 rounded-full px-2 py-1 transition-colors ${
                comment.my_reaction === 1 ? "bg-gray-200 text-gray-900" : "hover:bg-gray-100"
              } disabled:cursor-not-allowed`}
              data-testid="comment-like-button"
              aria-pressed={comment.my_reaction === 1}
              aria-label="좋아요"
            >
              <ThumbUpIcon filled={comment.my_reaction === 1} />
              {comment.likes_count > 0 ? <span>{comment.likes_count}</span> : null}
            </button>
            <button
              type="button"
              onClick={() => handleReact(-1)}
              disabled={!isAuthed || pendingReaction}
              className={`flex items-center gap-1 rounded-full px-2 py-1 transition-colors ${
                comment.my_reaction === -1 ? "bg-gray-200 text-gray-900" : "hover:bg-gray-100"
              } disabled:cursor-not-allowed`}
              data-testid="comment-dislike-button"
              aria-pressed={comment.my_reaction === -1}
              aria-label="싫어요"
            >
              <ThumbDownIcon filled={comment.my_reaction === -1} />
              {comment.dislikes_count > 0 ? <span>{comment.dislikes_count}</span> : null}
            </button>
            {onReplyClick ? (
              <button
                type="button"
                onClick={onReplyClick}
                disabled={!isAuthed}
                className="rounded-full px-2 py-1 font-medium transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50"
                data-testid="comment-reply-button"
              >
                답글
              </button>
            ) : null}
            {isOwner ? (
              <>
                <span className="text-gray-300">·</span>
                <button
                  type="button"
                  onClick={() => setIsEditing(true)}
                  className="rounded-full px-2 py-1 transition-colors hover:bg-gray-100"
                  data-testid="comment-edit-button"
                >
                  수정
                </button>
                <button
                  type="button"
                  onClick={handleDelete}
                  className="rounded-full px-2 py-1 transition-colors hover:bg-gray-100"
                  data-testid="comment-delete-button"
                >
                  삭제
                </button>
              </>
            ) : isAdmin ? (
              <>
                <span className="text-gray-300">·</span>
                <button
                  type="button"
                  onClick={handleDelete}
                  className="rounded-full px-2 py-1 text-red-600 transition-colors hover:bg-red-50"
                  data-testid="comment-admin-delete-button"
                >
                  관리자 삭제
                </button>
              </>
            ) : null}
          </div>

          {replyToggle}
        </div>
      </div>
    </div>
  );
}

function ThumbUpIcon({ filled }: { filled: boolean }) {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill={filled ? "currentColor" : "none"}
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M7 11v8H4v-8h3zM7 11l5-9c1.5 0 3 1 3 3v4h4c1.5 0 2.5 1 2.5 2.5L20 18c-.2 1-1 2-2.5 2H7" />
    </svg>
  );
}

function ThumbDownIcon({ filled }: { filled: boolean }) {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill={filled ? "currentColor" : "none"}
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M17 13V5h3v8h-3zM17 13l-5 9c-1.5 0-3-1-3-3v-4H5c-1.5 0-2.5-1-2.5-2.5L4 6c.2-1 1-2 2.5-2H17" />
    </svg>
  );
}
