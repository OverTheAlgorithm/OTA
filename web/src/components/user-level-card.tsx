import { useEffect, useState } from "react";
import { getUserLevel, getWithdrawalInfo, type LevelInfo, type WithdrawalInfo } from "@/lib/api";
import { useAuth } from "@/contexts/auth-context";
import { LevelCard } from "./level-card";
import { WithdrawalErrorModal } from "./withdrawal-error-modal";
import { WithdrawalModal } from "./withdrawal-modal";

/**
 * Self-contained level card that fetches its own data.
 * Renders nothing when the user is not logged in or data is unavailable.
 * Pass a changing `refreshKey` to trigger a re-fetch (e.g. after earning coins).
 */
export function UserLevelCard({ refreshKey = 0 }: { refreshKey?: number }) {
  const { user } = useAuth();
  const [level, setLevel] = useState<LevelInfo | null>(null);
  const [errorModal, setErrorModal] = useState<{ open: boolean; message: string }>({ open: false, message: "" });
  const [withdrawalModal, setWithdrawalModal] = useState<{ open: boolean; info: WithdrawalInfo | null }>({ open: false, info: null });
  const [checking, setChecking] = useState(false);
  const [internalRefresh, setInternalRefresh] = useState(0);

  useEffect(() => {
    if (!user) {
      setLevel(null);
      return;
    }
    getUserLevel().then(setLevel).catch(() => {});
  }, [user, refreshKey, internalRefresh]);

  const handleWithdrawClick = async () => {
    if (checking) return;
    setChecking(true);
    try {
      const info = await getWithdrawalInfo();
      if (!info.has_bank_account) {
        setErrorModal({ open: true, message: "마이페이지에서 먼저 계좌를 등록해주세요." });
        return;
      }
      if (info.current_balance < info.min_withdrawal_amount) {
        setErrorModal({
          open: true,
          message: `현재 보유 포인트: ${info.current_balance.toLocaleString()}P\n최소 출금 가능 포인트: ${info.min_withdrawal_amount.toLocaleString()}P\n\n포인트가 부족합니다.`
        });
        return;
      }
      setWithdrawalModal({ open: true, info });
    } catch {
      setErrorModal({ open: true, message: "출금 정보를 확인할 수 없습니다. 잠시 후 다시 시도해주세요." });
    } finally {
      setChecking(false);
    }
  };

  const handleWithdrawalSuccess = () => {
    setWithdrawalModal({ open: false, info: null });
    setInternalRefresh(prev => prev + 1);
  };

  if (!level) return null;

  return (
    <>
      <LevelCard level={level} onWithdrawClick={handleWithdrawClick} />
      <WithdrawalErrorModal
        open={errorModal.open}
        message={errorModal.message}
        onClose={() => setErrorModal({ open: false, message: "" })}
      />
      {withdrawalModal.info && (
        <WithdrawalModal
          open={withdrawalModal.open}
          onClose={() => setWithdrawalModal({ open: false, info: null })}
          onSuccess={handleWithdrawalSuccess}
          preCheckInfo={withdrawalModal.info}
        />
      )}
    </>
  );
}
