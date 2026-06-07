import { useEffect } from "react";
import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  useSearchParams,
  useLocation,
  useNavigationType,
} from "react-router-dom";
import { AuthProvider } from "@/contexts/auth-context";
import { ErrorBoundary } from "@/components/error-boundary";
import { CookieConsent } from "@/components/cookie-consent";
import { MainPage } from "@/pages/main";
import { SearchPage } from "@/pages/search";

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
import { EditorPicksPage } from "@/pages/editor-picks";
import { EditorPickDetailPage } from "@/pages/editor-pick-detail";
import { EditorNewPage } from "@/pages/editor-new";
import { EditorEditPage } from "@/pages/editor-edit";
import { AdminUsersPage } from "@/pages/admin-users";

function LoginRedirect() {
  const [searchParams] = useSearchParams();
  const error = searchParams.get("error");
  return <Navigate to={error ? `/?error=${error}` : "/"} replace />;
}

// Scroll to top on every new navigation (PUSH/REPLACE). POP (back/forward,
// reload) is left alone so per-page sessionStorage restoration (allnews,
// latest) can place the user where they were. Disables the browser's auto
// scrollRestoration so it doesn't fight the manual restore.
function ScrollToTop() {
  const { pathname } = useLocation();
  const navType = useNavigationType();

  useEffect(() => {
    if ("scrollRestoration" in window.history) {
      window.history.scrollRestoration = "manual";
    }
  }, []);

  useEffect(() => {
    if (navType === "POP") return;
    window.scrollTo(0, 0);
  }, [pathname, navType]);

  return null;
}

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <ScrollToTop />
        <Routes>
          <Route path="/" element={<ErrorBoundary><MainPage /></ErrorBoundary>} />
          <Route path="/login" element={<LoginRedirect />} />
          <Route path="/search" element={<ErrorBoundary><SearchPage /></ErrorBoundary>} />

          <Route path="/latest" element={<ErrorBoundary><LatestPage /></ErrorBoundary>} />
          <Route path="/allnews" element={<ErrorBoundary><AllNewsPage /></ErrorBoundary>} />
          <Route path="/email-verification" element={<ErrorBoundary><EmailVerificationPage /></ErrorBoundary>} />
          <Route path="/topic/:id" element={<ErrorBoundary><TopicPage /></ErrorBoundary>} />
          <Route path="/admin" element={<ErrorBoundary><AdminPage /></ErrorBoundary>} />
          <Route path="/admin/withdrawals" element={<ErrorBoundary><AdminWithdrawalsPage /></ErrorBoundary>} />
          <Route path="/admin/terms" element={<ErrorBoundary><AdminTermsPage /></ErrorBoundary>} />
          <Route path="/admin/coins" element={<ErrorBoundary><AdminCoinsPage /></ErrorBoundary>} />
          <Route path="/admin/push" element={<ErrorBoundary><AdminPushPage /></ErrorBoundary>} />
          <Route path="/admin/users" element={<ErrorBoundary><AdminUsersPage /></ErrorBoundary>} />
          <Route path="/editor-picks" element={<ErrorBoundary><EditorPicksPage /></ErrorBoundary>} />
          <Route path="/editor-picks/:id" element={<ErrorBoundary><EditorPickDetailPage /></ErrorBoundary>} />
          <Route path="/editor/new" element={<ErrorBoundary><EditorNewPage /></ErrorBoundary>} />
          <Route path="/editor/edit/:id" element={<ErrorBoundary><EditorEditPage /></ErrorBoundary>} />
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
