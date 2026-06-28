import { FormEvent, useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import SSOProviders from "../components/SSOProviders";
import Alert from "../components/ui/Alert";
import { useAuth } from "../lib/auth";

export default function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { login, oidcEnabled, oidcProviders, basicEnabled, user } = useAuth();
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
      <div className="login-page__backdrop" aria-hidden>
        <div className="login-orb login-orb--1" />
        <div className="login-orb login-orb--2" />
        <div className="login-orb login-orb--3" />
      </div>

      <div className="login-layout">
        <section className="login-hero">
          <div className="brand login-hero__brand">
            <span className="brand__icon">
              <span className="brand__mark">NC</span>
            </span>
            <div className="brand__text">
              <span className="brand__name">NATS Consol</span>
              <span className="brand__tagline">JetStream operations console</span>
            </div>
          </div>
          <h1 className="login-hero__title">Operate JetStream with clarity.</h1>
          <p className="login-hero__desc">
            Streams, consumers, KV, object stores, and live tail — unified in one fast console built for operators.
          </p>
          <ul className="login-hero__features">
            <li>Multi-cluster management</li>
            <li>Live message inspection</li>
            <li>Role-based access control</li>
          </ul>
        </section>

        <div className="login-card">
          <h2 className="login-card__title">Welcome back</h2>
          <p className="login-card__hint">
            {oidcEnabled && basicEnabled
              ? "Sign in with SSO or your console credentials"
              : oidcEnabled
                ? "Sign in with your organization account"
                : "Enter your console credentials"}
          </p>

          {oidcEnabled && <SSOProviders providers={oidcProviders} />}
          {oidcEnabled && basicEnabled && <div className="login-divider">or continue with email</div>}

          {basicEnabled && (
            <form className="form-grid form-grid--login" onSubmit={onSubmit}>
              <label>
                Username
                <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" placeholder="admin" />
              </label>
              <label>
                Password
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="current-password"
                  placeholder="••••••••"
                />
              </label>
              <button className="btn btn--block" type="submit">
                Sign in
              </button>
            </form>
          )}

          <Alert variant="error">{error}</Alert>
        </div>
      </div>
    </div>
  );
}
