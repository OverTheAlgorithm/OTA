import { useCallback, useEffect, useRef } from "react";
import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Image from "@tiptap/extension-image";
import Link from "@tiptap/extension-link";
import Placeholder from "@tiptap/extension-placeholder";
import CharacterCount from "@tiptap/extension-character-count";
import TextAlign from "@tiptap/extension-text-align";

const MAX_CONTENT_CHARS = 50_000;

interface RichTextEditorProps {
  value: string;
  onChange: (html: string) => void;
  onImageUpload: (file: File) => Promise<string>;
  placeholder?: string;
}

export function RichTextEditor({
  value,
  onChange,
  onImageUpload,
  placeholder = "여기에 내용을 입력하세요...",
}: RichTextEditorProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        // Hardcoded TipTap defaults are fine; we sanitise on the server anyway.
      }),
      Image.configure({ inline: false, allowBase64: false }),
      Link.configure({
        openOnClick: false,
        autolink: true,
        HTMLAttributes: { rel: "nofollow noopener", target: "_blank" },
      }),
      Placeholder.configure({ placeholder }),
      CharacterCount.configure({ limit: MAX_CONTENT_CHARS }),
      TextAlign.configure({ types: ["heading", "paragraph"] }),
    ],
    content: value,
    onUpdate: ({ editor }) => {
      onChange(editor.getHTML());
    },
    editorProps: {
      attributes: {
        class:
          "prose prose-stone max-w-none min-h-[400px] focus:outline-none px-4 py-3",
      },
    },
  });

  // Keep the editor in sync if the parent resets the value (e.g. after loading
  // an existing post). Only fire when the external value actually differs to
  // avoid clobbering ongoing edits.
  useEffect(() => {
    if (editor && value !== editor.getHTML()) {
      editor.commands.setContent(value, { emitUpdate: false });
    }
  }, [value, editor]);

  const handleImageSelect = useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0];
      if (!file || !editor) return;
      try {
        const url = await onImageUpload(file);
        editor.chain().focus().setImage({ src: url, alt: file.name }).run();
      } catch (err) {
        alert(err instanceof Error ? err.message : "이미지 업로드 실패");
      } finally {
        // Reset so the same file can be picked again.
        event.target.value = "";
      }
    },
    [editor, onImageUpload],
  );

  const promptLink = useCallback(() => {
    if (!editor) return;
    const prev = editor.getAttributes("link").href as string | undefined;
    const url = window.prompt("링크 URL", prev ?? "https://");
    if (url === null) return;
    if (url === "") {
      editor.chain().focus().unsetLink().run();
      return;
    }
    editor.chain().focus().extendMarkRange("link").setLink({ href: url }).run();
  }, [editor]);

  if (!editor) {
    return (
      <div className="border-2 border-[#231815] rounded p-4 text-sm text-stone-500">
        에디터를 불러오는 중...
      </div>
    );
  }

  return (
    <div className="border-2 border-[#231815] rounded-md bg-white">
      <div className="flex flex-wrap items-center gap-1 border-b-2 border-[#231815] p-2 bg-[#fdf9ee]">
        <ToolbarButton active={editor.isActive("bold")} onClick={() => editor.chain().focus().toggleBold().run()} label="굵게">
          <strong>B</strong>
        </ToolbarButton>
        <ToolbarButton active={editor.isActive("italic")} onClick={() => editor.chain().focus().toggleItalic().run()} label="기울임">
          <em>I</em>
        </ToolbarButton>
        <ToolbarButton active={editor.isActive("strike")} onClick={() => editor.chain().focus().toggleStrike().run()} label="취소선">
          <s>S</s>
        </ToolbarButton>
        <Divider />
        <ToolbarButton active={editor.isActive("heading", { level: 1 })} onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()} label="제목 1">H1</ToolbarButton>
        <ToolbarButton active={editor.isActive("heading", { level: 2 })} onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()} label="제목 2">H2</ToolbarButton>
        <ToolbarButton active={editor.isActive("heading", { level: 3 })} onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()} label="제목 3">H3</ToolbarButton>
        <Divider />
        <ToolbarButton active={editor.isActive("bulletList")} onClick={() => editor.chain().focus().toggleBulletList().run()} label="글머리 기호">• 목록</ToolbarButton>
        <ToolbarButton active={editor.isActive("orderedList")} onClick={() => editor.chain().focus().toggleOrderedList().run()} label="번호 매기기">1. 목록</ToolbarButton>
        <ToolbarButton active={editor.isActive("blockquote")} onClick={() => editor.chain().focus().toggleBlockquote().run()} label="인용">❝</ToolbarButton>
        <ToolbarButton active={editor.isActive("codeBlock")} onClick={() => editor.chain().focus().toggleCodeBlock().run()} label="코드 블록">{"<>"}</ToolbarButton>
        <Divider />
        <ToolbarButton active={editor.isActive({ textAlign: "left" })} onClick={() => editor.chain().focus().setTextAlign("left").run()} label="좌측 정렬">⬅</ToolbarButton>
        <ToolbarButton active={editor.isActive({ textAlign: "center" })} onClick={() => editor.chain().focus().setTextAlign("center").run()} label="중앙 정렬">⬌</ToolbarButton>
        <ToolbarButton active={editor.isActive({ textAlign: "right" })} onClick={() => editor.chain().focus().setTextAlign("right").run()} label="우측 정렬">➡</ToolbarButton>
        <Divider />
        <ToolbarButton active={editor.isActive("link")} onClick={promptLink} label="링크">🔗</ToolbarButton>
        <ToolbarButton onClick={() => fileInputRef.current?.click()} label="이미지">🖼️</ToolbarButton>
        <input
          ref={fileInputRef}
          type="file"
          accept="image/jpeg,image/png,image/webp,image/gif"
          className="hidden"
          onChange={handleImageSelect}
        />
        <Divider />
        <ToolbarButton onClick={() => editor.chain().focus().undo().run()} label="실행 취소">↶</ToolbarButton>
        <ToolbarButton onClick={() => editor.chain().focus().redo().run()} label="다시 실행">↷</ToolbarButton>
      </div>
      <EditorContent editor={editor} />
      <div className="flex justify-between items-center border-t border-[#231815]/30 px-4 py-2 text-xs text-stone-500">
        <span>{editor.storage.characterCount.characters()} / {MAX_CONTENT_CHARS.toLocaleString()}자</span>
        <span>{editor.storage.characterCount.words()} 단어</span>
      </div>
    </div>
  );
}

interface ToolbarButtonProps {
  active?: boolean;
  onClick: () => void;
  label: string;
  children: React.ReactNode;
}

function ToolbarButton({ active, onClick, label, children }: ToolbarButtonProps) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      className={`inline-flex items-center justify-center min-w-[34px] h-8 px-2 rounded text-sm font-medium border ${
        active
          ? "bg-[#231815] text-white border-[#231815]"
          : "bg-white text-[#231815] border-[#231815]/40 hover:bg-[#231815]/5"
      }`}
    >
      {children}
    </button>
  );
}

function Divider() {
  return <div className="w-px h-6 bg-[#231815]/30 mx-1" aria-hidden />;
}
