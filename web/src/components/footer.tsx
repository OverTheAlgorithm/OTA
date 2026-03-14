import { useEffect, useState } from "react";
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
      } text-[#6b8db5]`}
    >
      {terms.map((t) => (
        <a
          key={t.id}
          href={t.url}
          target="_blank"
          rel="noopener noreferrer"
          className="hover:text-[#1e3a5f] transition-colors"
        >
          {t.title}
        </a>
      ))}
      <a
        href="mailto:mindhacker.admin@gmail.com"
        className="hover:text-[#1e3a5f] transition-colors"
      >
        mindhacker.admin@gmail.com
      </a>
    </div>
  );

  if (compact) {
    return (
      <footer className="border-t border-[#d4e6f5] py-6 px-6 mt-4">
        <div className="max-w-2xl mx-auto flex flex-col items-center gap-3">
          <img src="/OTA_logo.png" alt="OTA" className="h-5 opacity-50" />
          {links}
          <p className="text-xs text-[#6b8db5]">
            &copy; 2026 Over the Algorithm. All rights reserved.
          </p>
        </div>
      </footer>
    );
  }

  return (
    <footer className="border-t border-[#d4e6f5] py-8 px-6">
      <div className="max-w-[1200px] mx-auto flex flex-col items-center gap-4">
        <img src="/OTA_logo.png" alt="OTA" className="h-6 opacity-50" />
        {links}
        <p className="text-sm text-[#6b8db5]">
          &copy; 2026 Over the Algorithm. All rights reserved.
        </p>
      </div>
    </footer>
  );
}
