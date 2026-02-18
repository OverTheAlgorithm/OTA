import { useState } from "react";
import { addSubscription, deleteSubscription } from "@/lib/api";

const PRESET_TAGS = [
  "연예/오락", "경제", "스포츠", "정치", "IT/기술",
  "패션/뷰티", "음식/맛집", "여행", "건강/의학",
  "게임", "사회/이슈", "문화/예술",
];

interface Props {
  selected: string[];
  onChange: (updated: string[]) => void;
}

export function InterestSection({ selected, onChange }: Props) {
  const [customInput, setCustomInput] = useState("");
  const [adding, setAdding] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const handleAdd = async (category: string) => {
    const trimmed = category.trim();
    if (!trimmed || selected.includes(trimmed)) return;
    setAdding(true);
    setErrorMsg(null);
    const prev = selected;
    onChange([...selected, trimmed]);
    try {
      await addSubscription(trimmed);
      setCustomInput("");
    } catch (err) {
      console.error("관심사 추가 실패:", err);
      onChange(prev);
      setErrorMsg("저장에 실패했습니다. 다시 시도해주세요.");
    } finally {
      setAdding(false);
    }
  };

  const handleRemove = async (category: string) => {
    setErrorMsg(null);
    const prev = selected;
    onChange(selected.filter((s) => s !== category));
    try {
      await deleteSubscription(category);
    } catch (err) {
      console.error("관심사 삭제 실패:", err);
      onChange(prev);
      setErrorMsg("저장에 실패했습니다. 다시 시도해주세요.");
    }
  };

  const unselected = PRESET_TAGS.filter((t) => !selected.includes(t));

  return (
    <section className="rounded-2xl bg-[#1a1229] border border-[#2d1f42] p-6">
      <div className="flex items-center gap-2 mb-5">
        <div className="w-8 h-8 rounded-lg bg-[#5ba4d9]/10 flex items-center justify-center">
          <svg className="w-4 h-4 text-[#5ba4d9]" viewBox="0 0 24 24" fill="none"
            stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M20.59 13.41l-7.17 7.17a2 2 0 01-2.83 0L2 12V2h10l8.59 8.59a2 2 0 010 2.82z"/>
            <line x1="7" y1="7" x2="7.01" y2="7"/>
          </svg>
        </div>
        <h2 className="font-semibold text-[#f5f0ff]">내 관심사</h2>
        <span className="ml-auto text-xs text-[#9b8bb4]">
          {selected.length > 0 ? `${selected.length}개 선택됨` : "관심사를 선택하면 맞춤 맥락을 받아요"}
        </span>
      </div>

      {selected.length > 0 && (
        <div className="flex flex-wrap gap-2 mb-4">
          {selected.map((tag) => (
            <button
              key={tag}
              onClick={() => handleRemove(tag)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium
                         bg-[#5ba4d9]/20 text-[#5ba4d9] border border-[#5ba4d9]/30
                         hover:bg-[#5ba4d9]/30 transition-colors"
            >
              {tag}
              <svg className="w-3 h-3" viewBox="0 0 12 12" fill="none"
                stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
                <path d="M9 3L3 9M3 3l6 6"/>
              </svg>
            </button>
          ))}
        </div>
      )}

      {unselected.length > 0 && (
        <>
          <p className="text-xs text-[#9b8bb4] mb-3">추가할 수 있는 관심사</p>
          <div className="flex flex-wrap gap-2 mb-4">
            {unselected.map((tag) => (
              <button
                key={tag}
                onClick={() => handleAdd(tag)}
                disabled={adding}
                className="px-3 py-1.5 rounded-full text-sm text-[#9b8bb4]
                           border border-[#2d1f42] hover:border-[#5ba4d9]/50
                           hover:text-[#f5f0ff] transition-colors disabled:opacity-50"
              >
                + {tag}
              </button>
            ))}
          </div>
        </>
      )}

      <div className="flex gap-2 mt-2">
        <input
          type="text"
          value={customInput}
          onChange={(e) => setCustomInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && handleAdd(customInput)}
          placeholder="직접 입력..."
          className="flex-1 bg-[#0f0a19] border border-[#2d1f42] rounded-xl px-4 py-2
                     text-sm text-[#f5f0ff] placeholder:text-[#9b8bb4]/50
                     focus:outline-none focus:border-[#5ba4d9]/50 transition-colors"
        />
        <button
          onClick={() => handleAdd(customInput)}
          disabled={!customInput.trim() || adding}
          className="px-4 py-2 rounded-xl text-sm font-medium bg-[#5ba4d9]/20 text-[#5ba4d9]
                     border border-[#5ba4d9]/30 hover:bg-[#5ba4d9]/30 transition-colors
                     disabled:opacity-40 disabled:cursor-not-allowed"
        >
          추가
        </button>
      </div>

      {errorMsg && (
        <p className="mt-3 text-xs text-[#e84d3d]">{errorMsg}</p>
      )}
    </section>
  );
}
