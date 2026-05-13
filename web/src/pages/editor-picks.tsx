import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Helmet } from "react-helmet-async";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import {
  listEditorPicks,
  defaultImage,
  type EditorPickCard,
} from "@/lib/api";
import { formatDate } from "@/lib/utils";

const PAGE_SIZE = 10;

export function EditorPicksPage() {
  const [items, setItems] = useState<EditorPickCard[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (currentOffset: number, append: boolean) => {
    try {
      const page = await listEditorPicks(PAGE_SIZE, currentOffset);
      setTotal(page.total);
      setItems((prev) => (append ? [...prev, ...page.items] : page.items));
    } catch (err) {
      setError(err instanceof Error ? err.message : "글 목록을 불러올 수 없습니다");
    }
  }, []);

  useEffect(() => {
    setLoading(true);
    load(0, false).finally(() => setLoading(false));
  }, [load]);

  const handleLoadMore = async () => {
    setLoadingMore(true);
    const next = offset + PAGE_SIZE;
    await load(next, true);
    setOffset(next);
    setLoadingMore(false);
  };

  const canLoadMore = items.length < total;

  return (
    <div className="min-h-screen flex flex-col bg-[#fdf9ee]">
      <Helmet>
        <title>에디터 픽 | WizLetter</title>
        <meta name="description" content="WizLetter 에디터가 직접 작성한 글들을 만나보세요" />
      </Helmet>
      <Header />
      <main className="flex-1 max-w-[900px] w-full mx-auto px-6 py-8">
        <header className="mb-8">
          <h1 className="text-3xl font-bold text-[#231815]">에디터 픽</h1>
          <p className="text-sm text-stone-600 mt-2">위즈레터 에디터가 직접 쓴 글</p>
        </header>

        {loading ? (
          <p className="text-stone-600">불러오는 중...</p>
        ) : error ? (
          <p className="text-red-700">{error}</p>
        ) : items.length === 0 ? (
          <p className="text-stone-600">아직 발행된 글이 없습니다.</p>
        ) : (
          <>
            <ul className="space-y-4">
              {items.map((card) => (
                <EditorPickListItem key={card.id} card={card} />
              ))}
            </ul>
            {canLoadMore && (
              <div className="flex justify-center mt-8">
                <button
                  type="button"
                  onClick={handleLoadMore}
                  disabled={loadingMore}
                  className="px-6 h-11 rounded-full border-2 border-[#231815] text-sm font-medium bg-white text-[#231815] hover:bg-[#231815]/5 disabled:opacity-60"
                >
                  {loadingMore ? "불러오는 중..." : `더 보기 (${items.length}/${total})`}
                </button>
              </div>
            )}
          </>
        )}
      </main>
      <Footer />
    </div>
  );
}

function EditorPickListItem({ card }: { card: EditorPickCard }) {
  const thumbnail = card.first_image_url || defaultImage;

  return (
    <li>
      <Link
        to={`/editor-picks/${card.id}`}
        className="flex gap-4 p-4 border-2 border-[#231815] rounded-lg bg-white hover:opacity-90 transition-opacity"
      >
        <div className="w-24 h-24 sm:w-28 sm:h-28 shrink-0 rounded-md overflow-hidden bg-[#f0ece0] border border-[#231815]/30">
          <img
            src={thumbnail}
            alt={card.title}
            className="w-full h-full object-cover"
            onError={(e) => {
              if (e.currentTarget.src !== defaultImage) e.currentTarget.src = defaultImage;
            }}
          />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-xs text-stone-500 mb-1">
            {card.author_name ? `${card.author_name} · ` : ""}
            {formatDate(card.published_at)}
          </p>
          <h2 className="text-lg font-bold text-[#231815] line-clamp-2">{card.title}</h2>
          {card.excerpt && (
            <p className="mt-1 text-sm text-stone-600 line-clamp-2">{card.excerpt}</p>
          )}
        </div>
      </Link>
    </li>
  );
}
