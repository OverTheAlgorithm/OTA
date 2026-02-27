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

      <div className="grid grid-cols-2 gap-2">
        {CATEGORIES.map((cat) => {
          const isActive = selected.includes(cat.key);
          return (
            <button
              key={cat.key}
              onClick={() => handleToggle(cat.key)}
              disabled={saving}
              className={`flex items-center gap-2.5 px-4 py-3 rounded-xl text-sm font-medium transition-colors
                ${isActive
                  ? "bg-[#5ba4d9]/20 text-[#5ba4d9] border border-[#5ba4d9]/30"
                  : "bg-[#0f0a19] text-[#9b8bb4] border border-[#2d1f42] hover:border-[#5ba4d9]/30 hover:text-[#f5f0ff]"
                }
                ${saving ? "opacity-50 cursor-not-allowed" : "cursor-pointer"}
              `}
            >
              <span className="text-lg">{cat.emoji}</span>
              <span>{cat.label}</span>
              {isActive && (
                <svg className="w-3.5 h-3.5 ml-auto" viewBox="0 0 14 14" fill="none"
                  stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M11.5 3.5L5.5 10.5L2.5 7.5"/>
                </svg>
              )}
            </button>
          );
        })}
      </div>

      {errorMsg && (
        <p className="mt-3 text-xs text-[#e84d3d]">{errorMsg}</p>
      )}
    </section>
  );
}
