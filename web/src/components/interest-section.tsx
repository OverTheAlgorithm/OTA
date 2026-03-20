import { useState } from "react";
import { addSubscription, deleteSubscription } from "@/lib/api";

const CATEGORIES: { key: string; label: string; emoji: string }[] = [
  { key: "entertainment", label: "연예/오락", emoji: "🎬" },
  { key: "business", label: "경제/비즈니스", emoji: "💰" },
  { key: "sports", label: "스포츠", emoji: "⚽" },
  { key: "technology", label: "IT/기술", emoji: "💻" },
  { key: "science", label: "과학", emoji: "🔬" },
  { key: "health", label: "건강/의학", emoji: "🏥" },
];

interface Props {
  selected: string[];
  onChange: (updated: string[]) => void;
}

export function InterestSection({ selected, onChange }: Props) {
  const [saving, setSaving] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const handleToggle = async (key: string) => {
    if (saving) return;

    setSaving(true);
    setErrorMsg(null);
    const prev = selected;
    const isSelected = selected.includes(key);

    if (isSelected) {
      onChange(selected.filter((s) => s !== key));
      try {
        await deleteSubscription(key);
      } catch (err) {
        console.error("관심사 삭제 실패:", err);
        onChange(prev);
        setErrorMsg("저장에 실패했습니다. 다시 시도해주세요.");
      } finally {
        setSaving(false);
      }
    } else {
      onChange([...selected, key]);
      try {
        await addSubscription(key);
      } catch (err) {
        console.error("관심사 추가 실패:", err);
        onChange(prev);
        setErrorMsg("저장에 실패했습니다. 다시 시도해주세요.");
      } finally {
        setSaving(false);
      }
    }
  };

  return (
    <section className="border-l-[3px] border-[#43b9d6] pl-5">
      <div className="flex items-center justify-between mb-5">
        <h2 className="text-lg font-bold text-[#231815]">내 관심사</h2>
        <span className="text-sm text-[#231815]/50">
          {selected.length > 0 ? `${selected.length}개 선택됨` : "관심사를 선택하면 맞춤 소식을 받아요"}
        </span>
      </div>

      <div className="grid grid-cols-2 gap-3">
        {CATEGORIES.map((cat) => {
          const isActive = selected.includes(cat.key);
          return (
            <button
              key={cat.key}
              onClick={() => handleToggle(cat.key)}
              disabled={saving}
              className={`flex items-center gap-3 px-4 py-3.5 rounded-xl text-sm font-medium border-[2px] transition-colors
                ${isActive
                  ? "bg-[#43b9d6]/10 text-[#231815] border-[#43b9d6]"
                  : "bg-white text-[#231815] border-[#231815] hover:bg-[#43b9d6]/5"
                }
                ${saving ? "opacity-50 cursor-not-allowed" : "cursor-pointer"}
              `}
            >
              <span className="text-xl">{cat.emoji}</span>
              <span>{cat.label}</span>
              {isActive && (
                <svg className="w-4 h-4 ml-auto text-[#43b9d6]" viewBox="0 0 14 14" fill="none"
                  stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M11.5 3.5L5.5 10.5L2.5 7.5"/>
                </svg>
              )}
            </button>
          );
        })}
      </div>

      {errorMsg && (
        <p className="mt-3 text-xs text-[#ff5442]">{errorMsg}</p>
      )}
    </section>
  );
}
