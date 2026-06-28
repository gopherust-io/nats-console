import { FormEvent, useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import ThemeSwitcher from "../components/ThemeSwitcher";
import { useAuth } from "../lib/auth";

export default function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { login, oidcEnabled, basicEnabled, user } = useAuth();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("admin");
  const [error, setError] = useState("");

  useEffect(() => {
    const authError = searchParams.get("error");
    if (authError) {
      setError(authError === "oidc_failed" ? "SSO sign-in failed. Try again or contact your administrator." : authError);
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
    try {
      await login(username, password);
      navigate("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    }
  }

  return (
    <div className="login-page">
      <div className="login-card">
        <div className="brand" style={{ marginBottom: 24 }}>
          <span className="brand__icon">NC</span>
          NATS Consol
        </div>
        <h1>Sign in</h1>
        <p className="login-card__hint">
          {oidcEnabled && basicEnabled
            ? "Use your console credentials or SSO"
            : oidcEnabled
              ? "Sign in with your organization account"
              : "Use your console credentials"}
        </p>
        {oidcEnabled && (
          <a className="btn btn--secondary mb-16" href="/api/v1/auth/oidc/login" style={{ display: "block", textAlign: "center" }}>
            Sign in with SSO
          </a>
        )}
        {basicEnabled && (
          <form className="form-grid" onSubmit={onSubmit}>
            <label>
              Username
              <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
            </label>
            <label>
              Password
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
              />
            </label>
            <button className="btn" type="submit">
              Continue
            </button>
            {error && <div className="error">{error}</div>}
          </form>
        )}
        {!basicEnabled && error && <div className="error">{error}</div>}
        <div className="login-card__themes">
          <div className="login-card__themes-label">Appearance</div>
          <ThemeSwitcher compact />
        </div>
      </div>
    </div>
  );
}
