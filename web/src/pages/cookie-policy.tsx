import { Link } from "react-router-dom";
import { Footer } from "@/components/footer";

export function CookiePolicyPage() {
  return (
    <div className="flex min-h-screen flex-col bg-[#fdf9ee]">
      <header className="border-b-[3px] border-[#231815] px-6 py-4">
        <div className="mx-auto max-w-3xl">
          <Link to="/">
            <img src="/wl-logo.png" alt="WizLetter" className="w-[160px]" />
          </Link>
        </div>
      </header>

      <main className="mx-auto w-full max-w-3xl flex-1 px-6 py-10">
        <h1 className="mb-8 text-2xl font-bold text-[#231815]">쿠키 정책</h1>
        <p className="mb-2 text-xs text-[#231815]/50">최종 업데이트: 2026년 4월 1일</p>

        <section className="mb-8">
          <h2 className="mb-3 text-lg font-semibold text-[#231815]">쿠키란?</h2>
          <p className="text-sm leading-relaxed text-[#231815]/80">
            쿠키(Cookie)는 웹사이트가 사용자의 브라우저에 저장하는 작은 텍스트
            파일입니다. 쿠키를 통해 사용자의 로그인 상태 유지, 사이트 이용 통계
            수집, 맞춤형 광고 제공 등이 가능합니다. WizLetter는 더 나은 서비스
            제공을 위해 아래와 같은 쿠키 및 유사 기술을 사용합니다.
          </p>
        </section>

        <section className="mb-8">
          <h2 className="mb-3 text-lg font-semibold text-[#231815]">
            사용하는 쿠키 목록
          </h2>

          <h3 className="mb-2 mt-4 text-sm font-semibold text-[#231815]">
            1. 필수 쿠키
          </h3>
          <p className="mb-3 text-sm text-[#231815]/70">
            서비스 운영에 반드시 필요한 쿠키로, 비활성화할 수 없습니다.
          </p>
          <div className="overflow-x-auto">
            <table className="mb-4 w-full text-sm">
              <thead>
                <tr className="border-b-2 border-[#231815]">
                  <th className="py-2 pr-4 text-left font-semibold text-[#231815]">이름</th>
                  <th className="py-2 pr-4 text-left font-semibold text-[#231815]">목적</th>
                  <th className="py-2 pr-4 text-left font-semibold text-[#231815]">유형</th>
                  <th className="py-2 text-left font-semibold text-[#231815]">만료</th>
                </tr>
              </thead>
              <tbody className="text-[#231815]/80">
                <tr className="border-b border-[#231815]/10">
                  <td className="py-2 pr-4 font-mono text-xs">ota_token</td>
                  <td className="py-2 pr-4">로그인 인증 토큰 (JWT)</td>
                  <td className="py-2 pr-4">쿠키</td>
                  <td className="py-2">7일</td>
                </tr>
                <tr className="border-b border-[#231815]/10">
                  <td className="py-2 pr-4 font-mono text-xs">wl_cookie_consent</td>
                  <td className="py-2 pr-4">쿠키 동의 여부 기록</td>
                  <td className="py-2 pr-4">localStorage</td>
                  <td className="py-2">영구</td>
                </tr>
                <tr className="border-b border-[#231815]/10">
                  <td className="py-2 pr-4 font-mono text-xs">cf_clearance 등</td>
                  <td className="py-2 pr-4">Cloudflare Turnstile 봇 방지 검증</td>
                  <td className="py-2 pr-4">쿠키</td>
                  <td className="py-2">세션</td>
                </tr>
              </tbody>
            </table>
          </div>

          <h3 className="mb-2 mt-4 text-sm font-semibold text-[#231815]">
            2. 분석 쿠키
          </h3>
          <p className="mb-3 text-sm text-[#231815]/70">
            사이트 이용 현황을 파악하고 서비스를 개선하기 위해 사용됩니다.
          </p>
          <div className="overflow-x-auto">
            <table className="mb-4 w-full text-sm">
              <thead>
                <tr className="border-b-2 border-[#231815]">
                  <th className="py-2 pr-4 text-left font-semibold text-[#231815]">이름</th>
                  <th className="py-2 pr-4 text-left font-semibold text-[#231815]">목적</th>
                  <th className="py-2 pr-4 text-left font-semibold text-[#231815]">제공자</th>
                  <th className="py-2 text-left font-semibold text-[#231815]">만료</th>
                </tr>
              </thead>
              <tbody className="text-[#231815]/80">
                <tr className="border-b border-[#231815]/10">
                  <td className="py-2 pr-4 font-mono text-xs">_ga, _gid 등</td>
                  <td className="py-2 pr-4">방문자 수, 페이지뷰, 이용 패턴 분석</td>
                  <td className="py-2 pr-4">Google Analytics (GTM)</td>
                  <td className="py-2">최대 2년</td>
                </tr>
              </tbody>
            </table>
          </div>

          <h3 className="mb-2 mt-4 text-sm font-semibold text-[#231815]">
            3. 광고 쿠키
          </h3>
          <p className="mb-3 text-sm text-[#231815]/70">
            사용자에게 관련성 높은 광고를 제공하고 광고 효과를 측정하기 위해
            사용됩니다.
          </p>
          <div className="overflow-x-auto">
            <table className="mb-4 w-full text-sm">
              <thead>
                <tr className="border-b-2 border-[#231815]">
                  <th className="py-2 pr-4 text-left font-semibold text-[#231815]">이름</th>
                  <th className="py-2 pr-4 text-left font-semibold text-[#231815]">목적</th>
                  <th className="py-2 pr-4 text-left font-semibold text-[#231815]">제공자</th>
                  <th className="py-2 text-left font-semibold text-[#231815]">만료</th>
                </tr>
              </thead>
              <tbody className="text-[#231815]/80">
                <tr className="border-b border-[#231815]/10">
                  <td className="py-2 pr-4 font-mono text-xs">__gads, __gpi 등</td>
                  <td className="py-2 pr-4">맞춤형 광고 제공 및 광고 성과 측정</td>
                  <td className="py-2 pr-4">Google AdSense</td>
                  <td className="py-2">최대 13개월</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section className="mb-8">
          <h2 className="mb-3 text-lg font-semibold text-[#231815]">
            쿠키 관리 방법
          </h2>
          <p className="mb-3 text-sm leading-relaxed text-[#231815]/80">
            대부분의 웹 브라우저에서 쿠키를 관리할 수 있습니다. 브라우저
            설정에서 쿠키를 삭제하거나 차단할 수 있으며, 특정 사이트의 쿠키만
            선택적으로 허용할 수도 있습니다.
          </p>
          <ul className="list-inside list-disc space-y-1 text-sm text-[#231815]/80">
            <li>
              <strong>Chrome</strong>: 설정 &gt; 개인정보 및 보안 &gt; 쿠키 및 기타 사이트 데이터
            </li>
            <li>
              <strong>Safari</strong>: 환경설정 &gt; 개인 정보 보호 &gt; 쿠키 및 웹사이트 데이터
            </li>
            <li>
              <strong>Firefox</strong>: 설정 &gt; 개인 정보 및 보안 &gt; 쿠키와 사이트 데이터
            </li>
            <li>
              <strong>Edge</strong>: 설정 &gt; 쿠키 및 사이트 권한 &gt; 쿠키 및 사이트 데이터
            </li>
          </ul>
          <p className="mt-3 text-sm leading-relaxed text-[#231815]/70">
            필수 쿠키를 차단하면 로그인 등 일부 기능이 정상적으로 작동하지 않을
            수 있습니다.
          </p>
        </section>

        <section className="mb-8">
          <h2 className="mb-3 text-lg font-semibold text-[#231815]">문의</h2>
          <p className="text-sm leading-relaxed text-[#231815]/80">
            쿠키 정책에 대한 문의 사항은{" "}
            <a
              href="mailto:mindhacker.admin@gmail.com"
              className="text-[#43b9d6] underline underline-offset-2"
            >
              mindhacker.admin@gmail.com
            </a>
            으로 연락해 주세요.
          </p>
        </section>
      </main>

      <Footer compact />
    </div>
  );
}
