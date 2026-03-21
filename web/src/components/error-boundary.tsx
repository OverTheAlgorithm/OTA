import { Component, type ErrorInfo, type ReactNode } from "react";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[ErrorBoundary] Caught rendering error:", error, info.componentStack);
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }
      return (
        <ErrorFallback
          error={this.state.error}
          onRetry={this.handleRetry}
        />
      );
    }
    return this.props.children;
  }
}

interface ErrorFallbackProps {
  error: Error | null;
  onRetry?: () => void;
}

export function ErrorFallback({ onRetry }: ErrorFallbackProps) {
  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-[#fdf9ee] px-4">
      <p className="text-6xl font-bold text-[#43b9d6]">!</p>
      <h1 className="mt-4 text-2xl font-semibold text-[#231815]">
        문제가 발생했어요
      </h1>
      <p className="mt-3 text-base text-[#231815]/60 text-center">
        페이지를 불러오는 중 오류가 발생했습니다.
        <br />
        잠시 후 다시 시도해 주세요.
      </p>
      <div className="mt-8 flex gap-3">
        {onRetry && (
          <button
            onClick={onRetry}
            className="px-8 py-3 rounded-full text-base font-semibold bg-[#43b9d6] text-[#231815] border-[2px] border-[#231815] hover:brightness-110 transition-all"
          >
            다시 시도
          </button>
        )}
        <button
          onClick={() => window.location.assign("/")}
          className="px-8 py-3 rounded-full text-base font-semibold bg-white text-[#231815] border-[2px] border-[#231815] hover:brightness-95 transition-all"
        >
          홈으로 돌아가기
        </button>
      </div>
    </div>
  );
}
