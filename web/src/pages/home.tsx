import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";

export function HomePage() {
  const { user, loading, logout } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (!loading && !user) {
      navigate("/login", { replace: true });
    }
  }, [user, loading, navigate]);

  if (loading || !user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <p className="text-muted-foreground">로딩 중...</p>
      </div>
    );
  }

  const handleLogout = async () => {
    await logout();
    navigate("/login", { replace: true });
  };

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b border-border">
        <div className="max-w-4xl mx-auto px-4 py-4 flex items-center justify-between">
          <h1 className="text-lg font-bold text-foreground">
            Over the Algorithm
          </h1>
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              {user.profile_image && (
                <img
                  src={user.profile_image}
                  alt=""
                  className="w-8 h-8 rounded-full"
                />
              )}
              <span className="text-sm text-foreground">
                {user.nickname || user.email || "사용자"}
              </span>
            </div>
            <button
              onClick={handleLogout}
              className="text-sm text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            >
              로그아웃
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-4xl mx-auto px-4 py-12">
        <div className="text-center">
          <h2 className="text-2xl font-bold text-foreground">
            환영합니다!
          </h2>
          <p className="mt-2 text-muted-foreground">
            매일 아침 7시, 가장 뜨거운 맥락을 전해드릴게요.
          </p>
        </div>
      </main>
    </div>
  );
}
