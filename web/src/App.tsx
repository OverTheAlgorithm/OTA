import { BrowserRouter, Routes, Route, Navigate, useSearchParams } from "react-router-dom";
import { AuthProvider } from "@/contexts/auth-context";
import { LandingPage } from "@/pages/landing";

import { EmailVerificationPage } from "@/pages/email-verification";
import { TopicPage } from "@/pages/topic";
import { AdminPage } from "@/pages/admin";
import { MypagePage } from "@/pages/mypage";
import { WithdrawalPage } from "@/pages/withdrawal";
import { AdminWithdrawalsPage } from "@/pages/admin-withdrawals";
import { AdminTermsPage } from "@/pages/admin-terms";
import { AdminCoinsPage } from "@/pages/admin-coins";
import { TermsConsentPage } from "@/pages/terms-consent";
import { AllNewsPage } from "@/pages/allnews";
import { NotFoundPage } from "@/pages/not-found";

function LoginRedirect() {
  const [searchParams] = useSearchParams();
  const error = searchParams.get("error");
  return <Navigate to={error ? `/?error=${error}` : "/"} replace />;
}

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/" element={<LandingPage />} />
          <Route path="/login" element={<LoginRedirect />} />

          <Route path="/allnews" element={<AllNewsPage />} />
          <Route path="/email-verification" element={<EmailVerificationPage />} />
          <Route path="/topic/:id" element={<TopicPage />} />
          <Route path="/admin" element={<AdminPage />} />
          <Route path="/admin/withdrawals" element={<AdminWithdrawalsPage />} />
          <Route path="/admin/terms" element={<AdminTermsPage />} />
          <Route path="/admin/coins" element={<AdminCoinsPage />} />
          <Route path="/terms-consent" element={<TermsConsentPage />} />
          <Route path="/mypage" element={<MypagePage />} />
          <Route path="/withdrawal" element={<WithdrawalPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}

export default App;
