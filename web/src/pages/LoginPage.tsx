import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import ThemeSwitcher from "../components/ThemeSwitcher";
import { api, setAuth } from "../lib/api";

export default function LoginPage() {
  const navigate = useNavigate();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("admin");
  const [error, setError] = useState("");

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");
    setAuth(username, password);
    try {
      await api("/api/health");
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
        <p className="login-card__hint">Default credentials: admin / admin</p>
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
        <div className="login-card__themes">
          <div className="login-card__themes-label">Appearance</div>
          <ThemeSwitcher compact />
        </div>
      </div>
    </div>
  );
}
