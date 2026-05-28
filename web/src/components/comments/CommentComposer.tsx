import { useState, useCallback } from "react";

const MAX_LEN = 2000;

interface CommentComposerProps {
  /** Placeholder when the textarea is empty. */
  placeholder?: string;
  /** Initial value for the textarea. Used for editing. */
  initialValue?: string;
  /** Submit handler. Resolve to indicate success; rejected promise re-enables the form. */
  onSubmit: (content: string) => Promise<void>;
  /** Optional cancel button — shown when provided (e.g. inline reply or edit). */
  onCancel?: () => void;
  /** Label for the submit button. */
  submitLabel?: string;
  /** When true the textarea auto-focuses on mount (useful for reply composer). */
  autoFocus?: boolean;
  /** Fired the first time the textarea receives focus. Used by the parent
   *  to surface the one-time nickname warning modal. */
  onFirstFocus?: () => void;
}

export function CommentComposer({
  placeholder = "댓글 추가...",
  initialValue = "",
  onSubmit,
  onCancel,
  submitLabel = "댓글",
  autoFocus = false,
  onFirstFocus,
}: CommentComposerProps) {
  const [value, setValue] = useState(initialValue);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [firstFocusFired, setFirstFocusFired] = useState(false);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const trimmed = value.trim();
      if (!trimmed) {
        setError("내용을 입력해주세요");
        return;
      }
      if (trimmed.length > MAX_LEN) {
        setError(`최대 ${MAX_LEN}자까지 입력 가능합니다`);
        return;
      }
      setSubmitting(true);
      setError(null);
      try {
        await onSubmit(trimmed);
        setValue("");
      } catch (err) {
        setError(err instanceof Error ? err.message : "댓글 작성 실패");
      } finally {
        setSubmitting(false);
      }
    },
    [value, onSubmit],
  );

  const disabled = submitting || value.trim().length === 0;

  return (
    <form onSubmit={handleSubmit} className="space-y-2" data-testid="comment-composer">
      <textarea
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onFocus={() => {
          if (!firstFocusFired) {
            setFirstFocusFired(true);
            onFirstFocus?.();
          }
        }}
        placeholder={placeholder}
        rows={3}
        maxLength={MAX_LEN + 100}
        autoFocus={autoFocus}
        className="w-full resize-none rounded-md border border-gray-200 bg-white p-3 text-sm leading-relaxed text-gray-900 focus:border-gray-400 focus:outline-none"
        data-testid="comment-composer-input"
      />
      <div className="flex items-center justify-between">
        <div className="text-xs text-gray-400">
          {value.length}/{MAX_LEN}
        </div>
        <div className="flex items-center gap-2">
          {onCancel ? (
            <button
              type="button"
              onClick={onCancel}
              className="rounded-full px-3 py-1.5 text-sm font-medium text-gray-600 hover:bg-gray-100"
            >
              취소
            </button>
          ) : null}
          <button
            type="submit"
            disabled={disabled}
            className="rounded-full bg-gray-900 px-4 py-1.5 text-sm font-medium text-white transition-opacity disabled:cursor-not-allowed disabled:opacity-40"
            data-testid="comment-composer-submit"
          >
            {submitting ? "..." : submitLabel}
          </button>
        </div>
      </div>
      {error ? <div className="text-xs text-red-600">{error}</div> : null}
    </form>
  );
}
