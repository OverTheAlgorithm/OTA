import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getActiveTerms, type Term } from "@/lib/api";

interface FooterProps {
  compact?: boolean;
}

export function Footer({ compact = false }: FooterProps) {
  const [terms, setTerms] = useState<Term[]>([]);

  useEffect(() => {
    getActiveTerms().then(setTerms).catch(() => {});
  }, []);

  const links = (
    <div
      className={`flex flex-wrap justify-center gap-y-1 ${
        compact ? "gap-x-4 text-xs" : "gap-x-6 text-sm"
      } text-[#231815]/60`}
    >
      {terms.filter((t) => t.title === "개인정보 처리방침 동의" || t.title === "서비스 이용약관 동의").map((t) => (
        <a
          key={t.id}
          href={t.url.match(/^https?:\/\//) ? t.url : `https://${t.url}`}
          target="_blank"
          rel="noopener noreferrer"
          className="hover:text-[#231815] transition-colors"
        >
          {t.title.replace(/ 동의$/, "")}
        </a>
      ))}
      <Link to="/cookie-policy" className="hover:text-[#231815] transition-colors">
        쿠키 정책
      </Link>
    </div>
  );

  if (compact) {
    return (
      <footer className="border-t-[3px] border-[#231815] py-6 px-6 mt-4 bg-[#fdf9ee]">
        <div className="max-w-2xl mx-auto flex flex-col items-center gap-3">
          <img src="/wl-logo.png" alt="WizLetter" className="w-[160px] opacity-60" />
          {links}
          <p className="text-xs text-[#231815]/60">문의: mindhacker.admin@gmail.com</p>
          <p className="text-xs text-[#231815]/50">
            &copy; 2026 WizLetter. All rights reserved.
          </p>
        </div>
      </footer>
    );
  }

  return (
    <footer className="border-t-[3px] border-[#231815] py-10 px-6 bg-[#fdf9ee]">
      <div className="max-w-[1200px] mx-auto flex flex-col items-center gap-4">
        <img src="/wl-logo.png" alt="WizLetter" className="w-[220px] opacity-60" />
        <div className="text-xs text-[#231815]/60 text-center space-y-1">
          <p>사업자 등록번호: 798-08-03338 | 주소: 서울특별시 영등포구 여의대방로43다길 19, 1층 101호(신길동)</p>
          <p>문의: mindhacker.admin@gmail.com</p>
        </div>
        {links}
        <p className="text-sm text-[#231815]/50">
          &copy; 2026 WizLetter. All rights reserved.
        </p>
      </div>
    </footer>
  );
}
