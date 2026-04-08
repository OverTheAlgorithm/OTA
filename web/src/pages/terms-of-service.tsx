import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Helmet } from "react-helmet-async";
import { Footer } from "@/components/footer";
import { renderMarkdown } from "@/lib/markdown";

export function TermsOfServicePage() {
  const [html, setHtml] = useState<string>("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch("/terms-of-service.md")
      .then((res) => res.text())
      .then((md) => {
        setHtml(renderMarkdown(md));
        setLoading(false);
      })
      .catch(() => {
        setHtml("<p>내용을 불러오는 데 실패했습니다.</p>");
        setLoading(false);
      });
  }, []);

  return (
    <div className="flex min-h-screen flex-col bg-[#fdf9ee]">
      <Helmet>
        <title>서비스 이용약관 - 위즈레터</title>
        <meta name="description" content="위즈레터의 서비스 이용약관을 안내합니다." />
        <link rel="canonical" href="https://wizletter.mindhacker.club/terms-of-service" />
      </Helmet>
      <header className="border-b-[3px] border-[#231815] px-6 py-4">
        <div className="mx-auto max-w-3xl">
          <Link to="/">
            <img src="/wl-logo.png" alt="WizLetter" className="w-[160px]" />
          </Link>
        </div>
      </header>

      <main className="mx-auto w-full max-w-3xl flex-1 px-6 py-10">
        {loading ? (
          <p className="text-sm text-[#231815]/50">불러오는 중...</p>
        ) : (
          <div
            className="prose-wl"
            dangerouslySetInnerHTML={{ __html: html }}
          />
        )}
      </main>

      <Footer compact />
    </div>
  );
}
