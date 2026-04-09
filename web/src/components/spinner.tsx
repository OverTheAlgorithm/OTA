import type { CSSProperties } from "react";

type SpinnerSize = "sm" | "md" | "lg";

const sizeClasses: Record<SpinnerSize, string> = {
  sm: "w-4 h-4",
  md: "w-6 h-6",
  lg: "w-10 h-10",
};

type SpinnerProps = {
  size?: SpinnerSize;
  className?: string;
  style?: CSSProperties;
};

/**
 * Spinner — 깔끔한 원형 로딩 인디케이터.
 * currentColor를 사용하므로 부모에서 text-* 클래스로 색상 제어.
 */
export function Spinner({ size = "md", className = "", style }: SpinnerProps) {
  return (
    <svg
      className={`${sizeClasses[size]} animate-spin ${className}`}
      viewBox="0 0 50 50"
      fill="none"
      role="status"
      aria-label="로딩 중"
      style={style}
    >
      <circle
        cx="25"
        cy="25"
        r="20"
        stroke="currentColor"
        strokeOpacity="0.15"
        strokeWidth="5"
      />
      <circle
        cx="25"
        cy="25"
        r="20"
        stroke="currentColor"
        strokeWidth="5"
        strokeLinecap="round"
        strokeDasharray="32 120"
      />
    </svg>
  );
}

type LoadingStateProps = {
  label?: string;
  size?: SpinnerSize;
  inline?: boolean;
  className?: string;
};

/**
 * LoadingState — 스피너 + 선택적 라벨.
 * - inline=false (기본): 세로 스택, 전체 영역 센터 정렬용
 * - inline=true: 가로 정렬, 리스트 안 등 작은 영역용
 * 색상은 부모의 text-* 클래스를 따른다.
 */
export function LoadingState({
  label,
  size,
  inline = false,
  className = "",
}: LoadingStateProps) {
  if (inline) {
    return (
      <div className={`flex items-center justify-center gap-2 ${className}`}>
        <Spinner size={size ?? "sm"} />
        {label && <span className="text-sm">{label}</span>}
      </div>
    );
  }

  return (
    <div className={`flex flex-col items-center justify-center gap-3 ${className}`}>
      <Spinner size={size ?? "lg"} />
      {label && <p className="text-sm">{label}</p>}
    </div>
  );
}
