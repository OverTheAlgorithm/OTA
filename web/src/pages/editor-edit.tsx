import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Helmet } from "react-helmet-async";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { RichTextEditor } from "@/components/rich-text-editor";
import { useAuth } from "@/contexts/auth-context";
import {
  deleteEditorPost,
  getEditorPost,
  hasRoleAtLeast,
  updateEditorPost,
  uploadEditorImage,
  type EditorPost,
} from "@/lib/api";

export function EditorEditPage() {
  const { id = "" } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user, loading } = useAuth();
  const [post, setPost] = useState<EditorPost | null>(null);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [fetching, setFetching] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    setFetching(true);
    getEditorPost(id)
      .then((p) => {
        if (cancelled) return;
        setPost(p);
        setTitle(p.title);
        setContent(p.content_html);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "글을 불러올 수 없습니다");
      })
      .finally(() => {
        if (!cancelled) setFetching(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  if (loading) return <div className="min-h-screen flex items-center justify-center bg-[#fdf9ee]">로딩 중...</div>;
  if (!user || !hasRoleAtLeast(user.role, "editor")) {
    navigate("/", { replace: true });
    return null;
  }

  const handleSave = async (status: "draft" | "published") => {
    if (!post) return;
    setError(null);
    if (!title.trim()) {
      setError("제목을 입력해주세요");
      return;
    }
    if (!content.trim() || content === "<p></p>") {
      setError("내용을 입력해주세요");
      return;
    }
    setSaving(true);
    try {
      const updated = await updateEditorPost(post.id, {
        title: title.trim(),
        content_html: content,
        status,
      });
      if (status === "published") {
        navigate(`/editor-picks/${updated.id}`);
      } else {
        setPost(updated);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "저장 실패");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!post) return;
    if (!confirm("정말 이 글을 삭제하시겠습니까? 되돌릴 수 없습니다.")) return;
    setSaving(true);
    try {
      await deleteEditorPost(post.id);
      navigate("/editor-picks", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "삭제 실패");
    } finally {
      setSaving(false);
    }
  };

  const handleImageUpload = async (file: File): Promise<string> => {
    const { url } = await uploadEditorImage(file);
    return url;
  };

  if (fetching) {
    return (
      <div className="min-h-screen bg-[#fdf9ee]">
        <Header />
        <main className="max-w-[1000px] w-full mx-auto px-6 py-8">
          <p>글을 불러오는 중...</p>
        </main>
      </div>
    );
  }

  if (!post) {
    return (
      <div className="min-h-screen bg-[#fdf9ee]">
        <Header />
        <main className="max-w-[1000px] w-full mx-auto px-6 py-8 text-red-700">
          {error ?? "글을 찾을 수 없습니다"}
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex flex-col bg-[#fdf9ee]">
      <Helmet>
        <title>{title || "글 수정"} | WizLetter</title>
      </Helmet>
      <Header />
      <main className="flex-1 max-w-[1000px] w-full mx-auto px-6 py-8">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold text-[#231815]">글 수정하기</h1>
          <span className={`text-xs font-medium px-3 py-1 rounded-full border ${post.status === "published" ? "bg-[#43b9d6] border-[#231815]" : "bg-stone-100 border-stone-300"}`}>
            {post.status === "published" ? "발행됨" : "임시저장"}
          </span>
        </div>

        <input
          type="text"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="제목을 입력하세요"
          maxLength={200}
          className="w-full border-2 border-[#231815] rounded-md px-4 py-3 text-lg font-bold bg-white mb-4 focus:outline-none focus:ring-2 focus:ring-[#43b9d6]"
        />

        <RichTextEditor
          value={content}
          onChange={setContent}
          onImageUpload={handleImageUpload}
        />

        {error && (
          <div className="mt-4 px-4 py-3 rounded border-2 border-red-400 bg-red-50 text-sm text-red-700">
            {error}
          </div>
        )}

        <div className="flex flex-wrap justify-between gap-3 mt-6">
          <button
            type="button"
            onClick={handleDelete}
            disabled={saving}
            className="px-5 h-10 rounded-full border-2 border-red-500 text-sm font-medium bg-white text-red-600 disabled:opacity-50"
          >
            삭제
          </button>
          <div className="flex flex-wrap gap-3">
            <button
              type="button"
              onClick={() => navigate(-1)}
              disabled={saving}
              className="px-5 h-10 rounded-full border-2 border-[#231815] text-sm font-medium bg-white text-[#231815] disabled:opacity-50"
            >
              취소
            </button>
            {post.status === "draft" && (
              <button
                type="button"
                onClick={() => handleSave("draft")}
                disabled={saving}
                className="px-5 h-10 rounded-full border-2 border-[#231815] text-sm font-medium bg-white text-[#231815] hover:bg-[#231815]/5 disabled:opacity-50"
              >
                {saving ? "저장 중..." : "임시 저장"}
              </button>
            )}
            <button
              type="button"
              onClick={() => handleSave("published")}
              disabled={saving}
              className="px-5 h-10 rounded-full border-2 border-[#231815] text-sm font-medium bg-[#43b9d6] text-[#231815] hover:opacity-80 disabled:opacity-50"
            >
              {saving ? "발행 중..." : "발행하기"}
            </button>
          </div>
        </div>
      </main>
      <Footer />
    </div>
  );
}
