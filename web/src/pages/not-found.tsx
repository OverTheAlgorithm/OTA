import { Link } from "react-router-dom";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";

export function NotFoundPage() {
  return (
    <div className="min-h-screen flex flex-col bg-[#fdf9ee]">
      <Header />

      <main className="flex-1 flex flex-col items-center justify-center px-4 py-20">
        <p className="text-8xl font-bold text-[#43b9d6]">404</p>
        <h1 className="mt-4 text-3xl font-semibold text-[#231815]">
          페이지를 찾을 수 없습니다
        </h1>
        <p className="mt-3 text-lg text-[#231815]/60">
          요청하신 페이지가 존재하지 않거나 이동되었어요.
        </p>
        <Link
          to="/"
          className="mt-8 px-10 py-4 rounded-full text-lg font-semibold bg-[#43b9d6] text-[#231815] border-[2px] border-[#231815] hover:brightness-110 transition-all"
        >
          홈으로 돌아가기
        </Link>
      </main>

      <Footer />
    </div>
  );
}
