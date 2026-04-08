import { BrowserRouter, Routes, Route, Navigate, useSearchParams } from "react-router-dom";
import { AuthProvider } from "@/contexts/auth-context";
import { ErrorBoundary } from "@/components/error-boundary";
import { CookieConsent } from "@/components/cookie-consent";
import { LandingPage } from "@/pages/landing";

import { EmailVerificationPage } from "@/pages/email-verification";
import { TopicPage } from "@/pages/topic";
import { AdminPage } from "@/pages/admin";
import { MypagePage } from "@/pages/mypage";
import { WithdrawalPage } from "@/pages/withdrawal";
import { AdminWithdrawalsPage } from "@/pages/admin-withdrawals";
import { AdminTermsPage } from "@/pages/admin-terms";
import { AdminCoinsPage } from "@/pages/admin-coins";
import { AdminPushPage } from "@/pages/admin-push";
import { TermsConsentPage } from "@/pages/terms-consent";
import { AllNewsPage } from "@/pages/allnews";
import { CookiePolicyPage } from "@/pages/cookie-policy";
import { PrivacyPolicyPage } from "@/pages/privacy-policy";
import { TermsOfServicePage } from "@/pages/terms-of-service";
import { AboutPage } from "@/pages/about";
import { LatestPage } from "@/pages/latest";
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
          <Route path="/" element={<ErrorBoundary><LandingPage /></ErrorBoundary>} />
          <Route path="/login" element={<LoginRedirect />} />

          <Route path="/latest" element={<ErrorBoundary><LatestPage /></ErrorBoundary>} />
          <Route path="/allnews" element={<ErrorBoundary><AllNewsPage /></ErrorBoundary>} />
          <Route path="/email-verification" element={<ErrorBoundary><EmailVerificationPage /></ErrorBoundary>} />
          <Route path="/topic/:id" element={<ErrorBoundary><TopicPage /></ErrorBoundary>} />
          <Route path="/admin" element={<ErrorBoundary><AdminPage /></ErrorBoundary>} />
          <Route path="/admin/withdrawals" element={<ErrorBoundary><AdminWithdrawalsPage /></ErrorBoundary>} />
          <Route path="/admin/terms" element={<ErrorBoundary><AdminTermsPage /></ErrorBoundary>} />
          <Route path="/admin/coins" element={<ErrorBoundary><AdminCoinsPage /></ErrorBoundary>} />
          <Route path="/admin/push" element={<ErrorBoundary><AdminPushPage /></ErrorBoundary>} />
          <Route path="/terms-consent" element={<ErrorBoundary><TermsConsentPage /></ErrorBoundary>} />
          <Route path="/mypage" element={<ErrorBoundary><MypagePage /></ErrorBoundary>} />
          <Route path="/withdrawal" element={<ErrorBoundary><WithdrawalPage /></ErrorBoundary>} />
          <Route path="/cookie-policy" element={<ErrorBoundary><CookiePolicyPage /></ErrorBoundary>} />
          <Route path="/privacy-policy" element={<ErrorBoundary><PrivacyPolicyPage /></ErrorBoundary>} />
          <Route path="/terms-of-service" element={<ErrorBoundary><TermsOfServicePage /></ErrorBoundary>} />
          <Route path="/about" element={<ErrorBoundary><AboutPage /></ErrorBoundary>} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
        <CookieConsent />
      </AuthProvider>
    </BrowserRouter>
  );
}

export default App;
