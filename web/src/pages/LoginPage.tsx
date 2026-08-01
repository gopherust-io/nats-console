import { FormEvent, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router";
import LoginSplitLayout from "../components/LoginSplitLayout";
import Alert from "../components/ui/Alert";
import { ApiError, userFacingError } from "../lib/api";
import { useAuth } from "../lib/auth";

export default function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { login, user } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [csrfHint, setCsrfHint] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);

  useEffect(() => {
    const authError = searchParams.get("error");
    if (authError) {
      setError(authError);
    }
  }, [searchParams]);

  useEffect(() => {
    if (user) {
      navigate("/", { replace: true });
    }
  }, [user, navigate]);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");
    setCsrfHint(false);
    try {
      await login(username, password);
      navigate("/");
    } catch (err) {
      if (err instanceof ApiError && err.code === "csrf_invalid") {
        setCsrfHint(true);
      }
      setError(err instanceof ApiError ? userFacingError(err, t) : err instanceof Error ? err.message : t("auth.loginFailed"));
    }
  }

  return (
    <LoginSplitLayout>
      <h1 className="login-pane__title">{t("auth.signIn")}</h1>
      <p className="login-pane__sub">{t("auth.needAccess")}</p>

      <div className="login-stack">
        <form className="login-form" onSubmit={onSubmit}>
          <label className="login-form__field">
            <span>{t("auth.username")}</span>
            <input
              id="login-username"
              name="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              autoFocus
              required
            />
          </label>
          <label className="login-form__field">
            <span>{t("auth.password")}</span>
            <input
              id="login-password"
              name="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
            />
          </label>
          <button className="login-primary-btn" type="submit">
            {t("auth.submitPassword")}
          </button>
        </form>

        <Alert variant="error">{error}</Alert>
        {csrfHint && (
          <button type="button" className="login-help-link" onClick={() => window.location.reload()}>
            {t("auth.csrfReload")}
          </button>
        )}

        <button type="button" className="login-help-link" onClick={() => setHelpOpen((v) => !v)}>
          {t("auth.helpLink")}
        </button>
        {helpOpen && <p className="login-help-copy">{t("auth.helpCopy")}</p>}
      </div>
    </LoginSplitLayout>
  );
}
