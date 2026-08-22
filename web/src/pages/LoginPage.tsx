import { FormEvent, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router";
import LoginSplitLayout from "../components/LoginSplitLayout";
import Alert from "../components/ui/Alert";
import ValidationHint from "../components/ui/ValidationHint";
import { LiquidMetalButton } from "../components/ui/liquid-metal-button";
import { ApiError, userFacingError } from "../lib/api";
import { useAuth } from "../lib/auth";

const LOGIN_ERROR_CODES = new Set(["access_denied", "session_expired", "invalid_request", "server_error"]);

type FieldError = "username" | "password" | null;

export default function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { login, user } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState("");
  const [csrfHint, setCsrfHint] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);
  const [fieldError, setFieldError] = useState<FieldError>(null);
  const usernameRef = useRef<HTMLInputElement>(null);
  const passwordRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const authError = searchParams.get("error");
    if (!authError) return;
    if (LOGIN_ERROR_CODES.has(authError)) {
      setError(t(`auth.errors.${authError}`));
      return;
    }
    setError("");
  }, [searchParams, t]);

  useEffect(() => {
    if (user) {
      navigate("/", { replace: true });
    }
  }, [user, navigate]);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");
    setCsrfHint(false);

    if (!username.trim()) {
      setFieldError("username");
      usernameRef.current?.focus();
      return;
    }
    if (!password) {
      setFieldError("password");
      passwordRef.current?.focus();
      return;
    }

    setFieldError(null);
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

  const requiredMessage = t("auth.fieldRequired");

  return (
    <LoginSplitLayout>
      <h1 className="login-pane__title">{t("auth.signIn")}</h1>

      <div className="login-stack">
        <form className="login-form" onSubmit={onSubmit} noValidate>
          <label className="login-form__field">
            <span>{t("auth.username")}</span>
            <input
              id="login-username"
              ref={usernameRef}
              name="username"
              value={username}
              onChange={(e) => {
                setUsername(e.target.value);
                if (fieldError === "username") setFieldError(null);
              }}
              autoComplete="username"
              autoFocus
              aria-invalid={fieldError === "username"}
              aria-describedby={fieldError === "username" ? "login-username-hint" : undefined}
            />
            {fieldError === "username" && <ValidationHint id="login-username-hint" message={requiredMessage} />}
          </label>
          <div className="login-form__field">
            <label htmlFor="login-password">{t("auth.password")}</label>
            <div className="login-form__password">
              <input
                id="login-password"
                ref={passwordRef}
                name="password"
                type={showPassword ? "text" : "password"}
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value);
                  if (fieldError === "password") setFieldError(null);
                }}
                autoComplete="current-password"
                aria-invalid={fieldError === "password"}
                aria-describedby={fieldError === "password" ? "login-password-hint" : undefined}
              />
              <button
                type="button"
                className="login-form__password-toggle"
                onClick={() => setShowPassword((v) => !v)}
                aria-pressed={showPassword}
                aria-label={showPassword ? t("auth.hidePassword") : t("auth.showPassword")}
              >
                {showPassword ? t("auth.hidePassword") : t("auth.showPassword")}
              </button>
            </div>
            {fieldError === "password" && <ValidationHint id="login-password-hint" message={requiredMessage} />}
          </div>
          <div className="login-form__submit">
            <LiquidMetalButton type="submit" label={t("auth.submitPassword")} />
          </div>
        </form>

        <Alert variant="error">{error}</Alert>
        {csrfHint && (
          <button type="button" className="login-help-link" onClick={() => window.location.reload()}>
            {t("auth.csrfReload")}
          </button>
        )}

        <div className="login-help">
          <button
            type="button"
            className="login-help-link"
            aria-expanded={helpOpen}
            aria-controls="login-help-panel"
            onClick={() => setHelpOpen((v) => !v)}
          >
            {t("auth.helpLink")}
          </button>
          {helpOpen && (
            <p id="login-help-panel" className="login-help-copy" role="region">
              {t("auth.helpCopy")}
            </p>
          )}
        </div>
      </div>
    </LoginSplitLayout>
  );
}
