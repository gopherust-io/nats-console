import { FormEvent, useEffect, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { Link, useNavigate, useParams } from "react-router";
import LoginSplitLayout from "../components/LoginSplitLayout";
import Alert from "../components/ui/Alert";
import { api } from "../lib/api";
import { useAuth } from "../lib/auth";

type InviteInfo = {
  username: string;
  email: string;
  expiresAt: string;
};

export default function InviteAcceptPage() {
  const { t } = useTranslation();
  const { token } = useParams();
  const navigate = useNavigate();
  const { reload, user } = useAuth();
  const [info, setInfo] = useState<InviteInfo | null>(null);
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (user) {
      navigate("/", { replace: true });
    }
  }, [user, navigate]);

  useEffect(() => {
    if (!token) return;
    setLoading(true);
    api<InviteInfo>(`/api/v1/auth/invite/${encodeURIComponent(token)}`)
      .then(setInfo)
      .catch((err) => setError(err instanceof Error ? err.message : t("auth.invalidInvite")))
      .finally(() => setLoading(false));
  }, [token, t]);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");
    if (password.length < 8) {
      setError(t("auth.passwordMin"));
      return;
    }
    if (password !== confirm) {
      setError(t("auth.passwordMismatch"));
      return;
    }
    try {
      await api("/api/v1/auth/invite/accept", {
        method: "POST",
        body: JSON.stringify({ token, password }),
      });
      await reload();
      navigate("/", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : t("auth.acceptFailed"));
    }
  }

  return (
    <LoginSplitLayout>
      <h1 className="login-pane__title">{t("auth.acceptInvite")}</h1>
      <div className="login-stack">
        {loading && <p className="login-help-copy">{t("auth.loadingInvite")}</p>}
        {!loading && info && (
          <>
            <p className="login-help-copy">
              <Trans
                i18nKey="auth.setPasswordFor"
                values={{ username: info.username }}
                components={{ strong: <strong /> }}
              />
            </p>
            <form className="login-form" onSubmit={onSubmit}>
              <label className="login-form__field">
                <span>{t("auth.password")}</span>
                <input
                  id="invite-password"
                  name="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="new-password"
                  required
                  autoFocus
                />
              </label>
              <label className="login-form__field">
                <span>{t("auth.confirmPassword")}</span>
                <input
                  id="invite-password-confirm"
                  name="password_confirm"
                  type="password"
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                  autoComplete="new-password"
                  required
                />
              </label>
              <button className="login-primary-btn" type="submit">
                {t("auth.setPasswordSubmit")}
              </button>
            </form>
          </>
        )}
        <Alert variant="error">{error}</Alert>
        <Link className="login-help-link" to="/login">
          {t("auth.backToSignIn")}
        </Link>
      </div>
    </LoginSplitLayout>
  );
}
