import { useState } from "react";
import { adminUpdatePoll, adminDeletePoll, type PollForUser } from "@/lib/api";

interface PollEditModalProps {
  poll: PollForUser;
  onClose: () => void;
  onSaved: (updated: { question: string; options: string[] }) => void;
  onDeleted: () => void;
}

export function PollEditModal({ poll, onClose, onSaved, onDeleted }: PollEditModalProps) {
  const [question, setQuestion] = useState(poll.question);
  const [options, setOptions] = useState<string[]>(poll.options);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const optionsChanged =
    options.length !== poll.options.length ||
    options.some((o, i) => o !== poll.options[i]);

  function setOpt(i: number, v: string) {
    setOptions(options.map((o, idx) => (idx === i ? v : o)));
  }
  function addOpt() {
    if (options.length < 4) setOptions([...options, ""]);
  }
  function removeOpt(i: number) {
    if (options.length > 2) setOptions(options.filter((_, idx) => idx !== i));
  }

  async function save() {
    setSaving(true);
    setError(null);
    try {
      const trimmedQ = question.trim();
      const trimmedOpts = options.map((o) => o.trim());
      await adminUpdatePoll(poll.context_item_id, trimmedQ, trimmedOpts);
      onSaved({ question: trimmedQ, options: trimmedOpts });
    } catch (e) {
      setError(e instanceof Error ? e.message : "update_failed");
    } finally {
      setSaving(false);
    }
  }

  async function del() {
    if (!window.confirm("이 투표를 삭제할까요? 투표 기록도 함께 사라져요.")) return;
    setSaving(true);
    setError(null);
    try {
      await adminDeletePoll(poll.context_item_id);
      onDeleted();
    } catch (e) {
      setError(e instanceof Error ? e.message : "delete_failed");
      setSaving(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
      role="dialog"
      aria-modal="true"
    >
      <div className="w-full max-w-lg rounded-2xl bg-[#fdf9ee] p-6 shadow-xl">
        <p className="mb-4 text-lg font-bold text-[#231815]">투표 편집 (관리자)</p>

        <label className="mb-1 block text-xs font-bold text-[#231815]/70">질문</label>
        <textarea
          value={question}
          onChange={(e) => setQuestion(e.target.value)}
          rows={2}
          className="mb-4 w-full rounded-xl border border-[#231815]/20 bg-white px-3 py-2 text-sm focus:border-[#43b9d6] focus:outline-none"
        />

        <label className="mb-1 block text-xs font-bold text-[#231815]/70">선택지 (2~4개)</label>
        <ul className="mb-2 flex flex-col gap-2">
          {options.map((opt, i) => (
            <li key={i} className="flex gap-2">
              <input
                value={opt}
                onChange={(e) => setOpt(i, e.target.value)}
                className="flex-1 rounded-xl border border-[#231815]/20 bg-white px-3 py-2 text-sm focus:border-[#43b9d6] focus:outline-none"
              />
              <button
                type="button"
                onClick={() => removeOpt(i)}
                disabled={options.length <= 2}
                className="rounded-xl border border-[#231815]/20 bg-white px-3 text-xs font-medium text-[#231815] disabled:opacity-40"
              >
                삭제
              </button>
            </li>
          ))}
        </ul>
        <button
          type="button"
          onClick={addOpt}
          disabled={options.length >= 4}
          className="mb-4 rounded-xl border border-dashed border-[#231815]/30 bg-white px-3 py-2 text-xs font-medium text-[#231815]/70 disabled:opacity-40"
        >
          + 선택지 추가
        </button>

        {optionsChanged && (
          <p className="mb-3 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800">
            선택지를 변경하면 기존 투표 기록이 모두 초기화됩니다.
          </p>
        )}
        {error && <p className="mb-2 text-xs text-red-500">{error}</p>}

        <div className="mt-2 flex items-center justify-between">
          <button
            type="button"
            onClick={del}
            disabled={saving}
            className="text-xs font-medium text-red-600 underline disabled:opacity-40"
          >
            투표 삭제
          </button>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={onClose}
              disabled={saving}
              className="rounded-xl border border-[#231815]/20 bg-white px-4 py-2 text-sm font-medium text-[#231815]"
            >
              취소
            </button>
            <button
              type="button"
              onClick={save}
              disabled={saving}
              className="rounded-xl bg-[#43b9d6] px-4 py-2 text-sm font-bold text-white disabled:opacity-50"
            >
              저장
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
