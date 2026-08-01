import { Component, type ErrorInfo, type ReactNode } from "react";
import { withTranslation, type WithTranslation } from "react-i18next";
import { Link } from "react-router";

type Props = WithTranslation & {
  children: ReactNode;
};

type State = {
  error: Error | null;
};

class ErrorBoundaryBase extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("UI render error", error, info.componentStack);
  }

  private reset = () => {
    this.setState({ error: null });
  };

  render() {
    const { t, children } = this.props;
    const { error } = this.state;
    if (!error) return children;

    return (
      <div className="nc-error-boundary" role="alert">
        <h1 className="nc-page-title">{t("errors.unexpectedTitle")}</h1>
        <p className="nc-page-sub">{t("errors.unexpectedDescription")}</p>
        {error.message && <p className="text-muted">{error.message}</p>}
        <div className="nc-error-boundary__actions">
          <button type="button" className="btn btn--primary" onClick={() => window.location.reload()}>
            {t("errors.reload")}
          </button>
          <Link className="btn" to="/systems" onClick={this.reset}>
            {t("errors.goHome")}
          </Link>
        </div>
      </div>
    );
  }
}

const ErrorBoundary = withTranslation()(ErrorBoundaryBase);
export default ErrorBoundary;
