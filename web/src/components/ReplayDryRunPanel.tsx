import { useTranslation } from "react-i18next";
import type { ReplayDryRun } from "../lib/api";
import { formatCompactCount, formatDurationMs } from "../lib/formatImpact";

export type ReplayDryRunPanelProps = {
  data?: ReplayDryRun | null;
  loading?: boolean;
  error?: string | null;
};

export default function ReplayDryRunPanel({ data, loading, error }: ReplayDryRunPanelProps) {
  const { t } = useTranslation();

  return (
    <div className="nc-blast-radius nc-replay-dry-run">
      <h3 className="nc-blast-radius__title">{t("streams.replayDryRunTitle")}</h3>
      <p className="nc-blast-radius__subtitle">{t("streams.replayDryRunSubtitle")}</p>

      {loading && !data ? (
        <p className="nc-blast-radius__status">{t("streams.replayDryRunLoading")}</p>
      ) : null}

      {error && !data ? (
        <p className="nc-blast-radius__status nc-blast-radius__status--error" role="status">
          {t("streams.replayDryRunFailed")}
          {typeof error === "string" && error !== "error" ? (
            <>
              {" "}
              <span className="nc-blast-radius__error-detail">({error})</span>
            </>
          ) : null}
        </p>
      ) : null}

      {data ? (
        <>
          <div className="nc-blast-radius__section">
            <div className="nc-blast-radius__section-label">{t("streams.replayDryRunImpact")}</div>
            <dl className="nc-replay-dry-run__stats">
              <div className="nc-replay-dry-run__row">
                <dt>{t("streams.replayDryRunMessages")}</dt>
                <dd>
                  {formatCompactCount(data.messages)}
                  {data.approximate ? (
                    <span className="nc-replay-dry-run__flag"> {t("streams.replayDryRunApproximate")}</span>
                  ) : null}
                  {data.unbounded ? (
                    <span className="nc-replay-dry-run__flag"> {t("streams.replayDryRunUnbounded")}</span>
                  ) : null}
                </dd>
              </div>
              <div className="nc-replay-dry-run__row">
                <dt>{t("streams.replayDryRunDuration")}</dt>
                <dd>{formatDurationMs(data.estimatedDurationMs)}</dd>
              </div>
              <div className="nc-replay-dry-run__row">
                <dt>{t("streams.replayDryRunConsumers")}</dt>
                <dd className="nc-blast-radius__count">{data.consumersAffected}</dd>
              </div>
            </dl>
          </div>

          {(data.potentialDuplicates ?? []).length > 0 ? (
            <div className="nc-blast-radius__section">
              <div className="nc-blast-radius__section-label nc-blast-radius__section-label--critical">
                {t("streams.replayDryRunDuplicates")}
              </div>
              <ul className="nc-blast-radius__critical">
                {(data.potentialDuplicates ?? []).map((name) => (
                  <li key={name}>
                    <code>{name}</code>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </>
      ) : null}
    </div>
  );
}
