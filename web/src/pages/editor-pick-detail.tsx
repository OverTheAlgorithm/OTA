import { useEffect, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { Helmet } from "react-helmet-async";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { SanitizedHTML } from "@/components/sanitized-html";
import { useAuth } from "@/contexts/auth-context";
import { getEditorPick, hasRoleAtLeast, type EditorPickDetail } from "@/lib/api";
import { formatDate } from "@/lib/utils";

export function EditorPickDetailPage() {
  const { id = "" } = useParams<{ id: string }>();
  const { user } = useAuth();
  const [post, setPost] = useState<EditorPickDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    setLoading(true);
    getEditorPick(id)
      .then((p) => {
        if (cancelled) return;
        setPost(p);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "글을 불러올 수 없습니다");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  const canEdit =
    !!post &&
    !!user &&
    (user.id === post.author_id || hasRoleAtLeast(user.role, "admin"));

  return (
    <div className="min-h-screen flex flex-col bg-[#fdf9ee]">
      <Helmet>
        <title>{post?.title ?? "에디터 픽"} | WizLetter</title>
        {post && <meta name="description" content={post.title} />}
      </Helmet>
      <Header />
      <main className="flex-1 max-w-[800px] w-full mx-auto px-6 py-8">
        {loading ? (
          <p className="text-stone-600">불러오는 중...</p>
        ) : error || !post ? (
          <div>
            <p className="text-red-700 mb-4">{error ?? "글을 찾을 수 없습니다"}</p>
            <Link to="/editor-picks" className="text-sm underline text-[#231815]">
              에디터 픽 목록으로
            </Link>
          </div>
        ) : (
          <article>
            <header className="mb-6">
              <Link to="/editor-picks" className="text-xs text-stone-500 hover:underline">
                ← 에디터 픽 목록
              </Link>
              <h1 className="mt-3 text-3xl font-bold text-[#231815] leading-tight">{post.title}</h1>
              <p className="mt-2 text-sm text-stone-600">
                {post.author_name ? `${post.author_name} · ` : ""}
                {formatDate(post.published_at)}
              </p>
              {canEdit && (
                <Link
                  to={`/editor/edit/${post.id}`}
                  className="inline-block mt-3 px-4 h-8 leading-8 rounded-full border-2 border-[#231815] text-xs font-medium bg-white text-[#231815] hover:bg-[#231815]/5"
                >
                  글 수정
                </Link>
              )}
            </header>

            <SanitizedHTML
              html={post.content_html}
              className="wl-editor-content max-w-none"
            />
          </article>
        )}
      </main>
      <Footer />
    </div>
  );
}
