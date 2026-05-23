import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Helmet } from "react-helmet-async";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { RichTextEditor } from "@/components/rich-text-editor";
import { useAuth } from "@/contexts/auth-context";
import {
  createEditorPost,
  hasRoleAtLeast,
  listMyEditorPosts,
  uploadEditorImage,
} from "@/lib/api";

export function EditorNewPage() {
  const navigate = useNavigate();
  const { user, loading } = useAuth();
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Each user keeps at most one draft. If one already exists we redirect to
  // its edit page so the in-progress content is restored automatically.
  const [checkingDraft, setCheckingDraft] = useState(true);

  const canEdit = !!user && hasRoleAtLeast(user.role, "editor");

  useEffect(() => {
    if (loading) return;
    if (!canEdit) {
      setCheckingDraft(false);
      return;
    }
    let cancelled = false;
    listMyEditorPosts()
      .then((posts) => {
        if (cancelled) return;
        const draft = posts.find((p) => p.status === "draft");
        if (draft) {
          navigate(`/editor/edit/${draft.id}`, { replace: true });
          return;
        }
        setCheckingDraft(false);
      })
      .catch(() => {
        if (cancelled) return;
        // Don't block the new-post flow if the lookup fails; just render the
        // blank editor and let the upsert on save handle uniqueness.
        setCheckingDraft(false);
      });
    return () => {
      cancelled = true;
    };
  }, [loading, canEdit, navigate]);

  if (loading || checkingDraft) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#fdf9ee]">
        <p>로딩 중...</p>
      </div>
    );
  }
  if (!canEdit) {
    navigate("/", { replace: true });
    return null;
  }

  const handleSave = async (status: "draft" | "published") => {
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
      const post = await createEditorPost({
        title: title.trim(),
        content_html: content,
        status,
      });
      if (status === "published") {
        navigate(`/editor-picks/${post.id}`);
      } else {
        navigate(`/editor/edit/${post.id}`);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "저장 실패");
    } finally {
      setSaving(false);
    }
  };

  const handleImageUpload = async (file: File): Promise<string> => {
    const { url } = await uploadEditorImage(file);
    return url;
  };

  return (
    <div className="min-h-screen flex flex-col bg-[#fdf9ee]">
      <Helmet>
        <title>새 글 발행하기 | WizLetter</title>
      </Helmet>
      <Header />
      <main className="flex-1 max-w-[1000px] w-full mx-auto px-6 py-8">
        <h1 className="text-2xl font-bold text-[#231815] mb-6">새 글 발행하기</h1>

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
          placeholder="여기에 글을 작성해주세요. 이미지, 링크, 코드 블록 등을 자유롭게 사용할 수 있습니다."
        />

        {error && (
          <div className="mt-4 px-4 py-3 rounded border-2 border-red-400 bg-red-50 text-sm text-red-700">
            {error}
          </div>
        )}

        <div className="flex flex-wrap justify-end gap-3 mt-6">
          <button
            type="button"
            onClick={() => navigate(-1)}
            disabled={saving}
            className="px-5 h-10 rounded-full border-2 border-[#231815] text-sm font-medium bg-white text-[#231815] disabled:opacity-50"
          >
            취소
          </button>
          <button
            type="button"
            onClick={() => handleSave("draft")}
            disabled={saving}
            className="px-5 h-10 rounded-full border-2 border-[#231815] text-sm font-medium bg-white text-[#231815] hover:bg-[#231815]/5 disabled:opacity-50"
          >
            {saving ? "저장 중..." : "임시 저장"}
          </button>
          <button
            type="button"
            onClick={() => handleSave("published")}
            disabled={saving}
            className="px-5 h-10 rounded-full border-2 border-[#231815] text-sm font-medium bg-[#43b9d6] text-[#231815] hover:opacity-80 disabled:opacity-50"
          >
            {saving ? "발행 중..." : "발행하기"}
          </button>
        </div>
      </main>
      <Footer />
    </div>
  );
}
