import { BrowserRouter, Routes, Route, Navigate, useSearchParams } from "react-router-dom";
import { AuthProvider } from "@/contexts/auth-context";
import { LandingPage } from "@/pages/landing";
import { HomePage } from "@/pages/home";
import { EmailVerificationPage } from "@/pages/email-verification";
import { TopicPage } from "@/pages/topic";

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
          <Route path="/home" element={<HomePage />} />
          <Route path="/email-verification" element={<EmailVerificationPage />} />
          <Route path="/topic/:id" element={<TopicPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}

export default App;
