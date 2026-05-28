import { useCallback, useState } from "react";
import { Link } from "react-router-dom";
import { dismissNicknameWarning } from "@/lib/api";
import { useAuth } from "@/contexts/auth-context";

interface NicknameWarningModalProps {
  open: boolean;
  onClose: () => void;
}

// Shown on first focus of the comment composer when the user's
// nickname_state is "default". Two outcomes both advance the server state
// so the modal never reappears for this user:
//
//   - "닉네임 변경하기" navigates to /mypage. The user may or may not
//     actually save a new nickname; we still mark the warning dismissed so
//     they can comment without being prompted again. If they do save, the
//     state advances further to "custom".
//   - "닫기" calls POST /user/nickname-warning/dismiss explicitly to flip
//     the state to "acknowledged".
//
// We refresh the auth user after either action so the in-memory User
// object reflects the new state immediately.
export function NicknameWarningModal({ open, onClose }: NicknameWarningModalProps) {
  const { refreshUser } = useAuth();
  const [pending, setPending] = useState(false);

  const dismissAndClose = useCallback(
    async (close: boolean) => {
      if (pending) return;
      setPending(true);
      try {
        await dismissNicknameWarning();
        await refreshUser();
      } catch (err) {
        // Surface a non-blocking warning but still close the modal so the
        // user is not stuck. The state may simply have already advanced.
        console.warn("dismiss nickname warning failed", err);
      } finally {
        setPending(false);
        if (close) onClose();
      }
    },
    [pending, refreshUser, onClose],
  );

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="nickname-warning-title"
      data-testid="nickname-warning-modal"
    >
      <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
        <h3 id="nickname-warning-title" className="text-base font-semibold text-gray-900">
          닉네임을 변경한 적이 없습니다
        </h3>
        <p className="mt-2 text-sm leading-relaxed text-gray-600">
          댓글 작성시 카카오톡 이름이 표시됩니다. 마이페이지에서 닉네임을 변경할 수 있습니다.
        </p>
        <div className="mt-5 flex items-center justify-end gap-2">
          <button
            type="button"
            onClick={() => dismissAndClose(true)}
            disabled={pending}
            className="rounded-full px-4 py-2 text-sm font-medium text-gray-600 hover:bg-gray-100 disabled:opacity-50"
            data-testid="nickname-warning-dismiss"
          >
            닫기
          </button>
          <Link
            to="/mypage?tab=settings"
            onClick={() => void dismissAndClose(false)}
            className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-black"
            data-testid="nickname-warning-change"
          >
            닉네임 변경하기
          </Link>
        </div>
      </div>
    </div>
  );
}
