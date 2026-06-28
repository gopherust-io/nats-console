export type SSOProvider = {
  id: string;
  name: string;
};

type Props = {
  providers: SSOProvider[];
};

function providerIcon(id: string) {
  switch (id) {
    case "google":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
          <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
          <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
          <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
        </svg>
      );
    case "github":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            fill="currentColor"
            d="M12 0C5.37 0 0 5.37 0 12c0 5.3 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61-.546-1.385-1.335-1.755-1.335-1.755-1.087-.744.084-.729.084-.729 1.205.084 1.84 1.236 1.84 1.236 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.303-5.466-1.332-5.466-5.93 0-1.31.468-2.382 1.235-3.222-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23a11.5 11.5 0 0 1 3.003-.404c1.02.005 2.047.138 3.003.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.652.242 2.873.118 3.176.77.84 1.235 1.911 1.235 3.222 0 4.61-2.807 5.624-5.479 5.92.43.372.823 1.102.823 2.222 0 1.606-.015 2.896-.015 3.286 0 .315.21.694.825.576C20.565 21.796 24 17.297 24 12 24 5.37 18.63 0 12 0z"
          />
        </svg>
      );
    case "gitlab":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path fill="#FC6D26" d="m23.6 9.6-.9-2.7L12 1.2 1.3 6.9l-.9 2.7 2.7 7.8L12 22.8l9.9-5.4 2.7-7.8z" />
          <path fill="#E24329" d="M12 22.8 7.5 17.4h9L12 22.8z" />
          <path fill="#FC6D26" d="M23.6 9.6H.4l2.7 7.8L12 22.8l8.9-5.4 2.7-7.8z" />
          <path fill="#FCA326" d="M12 1.2 15.3 9.6H8.7L12 1.2z" />
          <path fill="#E24329" d="M1.3 6.9 8.7 9.6 12 1.2 15.3 9.6l7.4-2.7L12 1.2 1.3 6.9z" />
        </svg>
      );
    case "microsoft":
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path fill="#F25022" d="M1 1h10v10H1z" />
          <path fill="#7FBA00" d="M13 1h10v10H13z" />
          <path fill="#00A4EF" d="M1 13h10v10H1z" />
          <path fill="#FFB900" d="M13 13h10v10H13z" />
        </svg>
      );
    default:
      return (
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            fill="currentColor"
            d="M12 2a5 5 0 0 1 5 5v1h2a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V10a2 2 0 0 1 2-2h2V7a5 5 0 0 1 5-5zm0 2a3 3 0 0 0-3 3v1h6V7a3 3 0 0 0-3-3z"
          />
        </svg>
      );
  }
}

function loginPath(provider: SSOProvider) {
  return provider.id === "oidc"
    ? "/api/v1/auth/oidc/login"
    : `/api/v1/auth/oidc/${encodeURIComponent(provider.id)}/login`;
}

export default function SSOProviders({ providers }: Props) {
  if (providers.length === 0) {
    return null;
  }

  return (
    <div className="sso-providers">
      {providers.map((provider) => (
        <a
          key={provider.id}
          className={`sso-btn sso-btn--${provider.id}`}
          href={loginPath(provider)}
        >
          <span className="sso-btn__icon">{providerIcon(provider.id)}</span>
          <span>Continue with {provider.name}</span>
        </a>
      ))}
    </div>
  );
}
