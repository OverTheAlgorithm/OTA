import { useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { KakaoLoginButton } from "@/components/kakao-login-button";
import { useAuth } from "@/contexts/auth-context";

export function LoginPage() {
  const { user, loading } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const error = searchParams.get("error");

  useEffect(() => {
    if (!loading && user) {
      navigate("/", { replace: true });
    }
  }, [user, loading, navigate]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <p className="text-muted-foreground">로딩 중...</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm flex flex-col items-center gap-8">
        <div className="text-center">
          <h1 className="text-3xl font-bold text-foreground tracking-tight">
            Over the Algorithm
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            알고리즘을 넘어, 지금 가장 뜨거운 맥락을 만나보세요
          </p>
        </div>

        {error && (
          <p className="text-sm text-destructive">
            로그인에 실패했습니다. 다시 시도해주세요.
          </p>
        )}

        <KakaoLoginButton />
      </div>
    </div>
  );
}
