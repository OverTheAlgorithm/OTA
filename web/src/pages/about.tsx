import { Link } from "react-router-dom";
import { Helmet } from "react-helmet-async";
import { Footer } from "@/components/footer";

export function AboutPage() {
  return (
    <div className="flex min-h-screen flex-col bg-[#fdf9ee]">
      <Helmet>
        <title>서비스 소개 - 위즈레터</title>
        <meta name="description" content="위즈레터는 AI 기술을 활용하여 다양한 뉴스를 종합 분석하고 핵심 소식을 전달하는 서비스입니다." />
        <link rel="canonical" href="https://wizletter.mindhacker.club/about" />
      </Helmet>
      <header className="border-b-[3px] border-[#231815] px-6 py-4">
        <div className="mx-auto max-w-3xl">
          <Link to="/">
            <img src="/wl-logo.png" alt="WizLetter" className="w-[160px]" />
          </Link>
        </div>
      </header>

      <main className="mx-auto w-full max-w-3xl flex-1 px-6 py-10">
        <h1 className="mb-8 text-2xl font-bold text-[#231815]">위즈레터 소개</h1>

        <section className="mb-8">
          <h2 className="mb-3 text-lg font-semibold text-[#231815]">서비스 소개</h2>
          <p className="text-sm leading-relaxed text-[#231815]/80">
            위즈레터는 매일 아침 AI 기술을 활용하여 다양한 뉴스 소스를 종합 분석하고, 핵심 소식을 간결한
            브리핑으로 전달하는 서비스입니다. 개인화 알고리즘에 갇히지 않고 더 넓은 시야로 세상을 바라볼 수
            있도록 돕습니다.
          </p>
        </section>

        <section className="mb-8">
          <h2 className="mb-3 text-lg font-semibold text-[#231815]">편집 방침</h2>
          <p className="text-sm leading-relaxed text-[#231815]/80">
            위즈레터의 콘텐츠는 AI가 여러 뉴스 소스를 종합하여 구조화된 브리핑으로 재구성합니다. 모든 콘텐츠는
            원본 기사의 사실에 기반하며, 출처를 명시합니다. 팀이 콘텐츠 품질을 지속적으로 모니터링하고
            개선합니다.
          </p>
        </section>

        <section className="mb-8">
          <h2 className="mb-3 text-lg font-semibold text-[#231815]">문의</h2>
          <p className="text-sm leading-relaxed text-[#231815]/80">
            이메일:{" "}
            <a
              href="mailto:mindhacker.admin@gmail.com"
              className="text-[#43b9d6] underline underline-offset-2"
            >
              mindhacker.admin@gmail.com
            </a>
          </p>
          <p className="mt-2 text-sm leading-relaxed text-[#231815]/80">
            사업자 등록번호: 798-08-03338
          </p>
          <p className="mt-1 text-sm leading-relaxed text-[#231815]/80">
            주소: 서울특별시 영등포구 여의대방로43다길 19, 1층 101호(신길동)
          </p>
        </section>
      </main>

      <Footer compact />
    </div>
  );
}
