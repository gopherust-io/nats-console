import { useTranslation } from "react-i18next";
import type { BlastRadius } from "../lib/api";

export type BlastRadiusPanelProps = {
  data?: BlastRadius | null;
  loading?: boolean;
  error?: string | null;
};

function formatNames(names: string[]): string {
  return names.length > 0 ? names.join(", ") : "—";
}

export default function BlastRadiusPanel({ data, loading, error }: BlastRadiusPanelProps) {
  const { t } = useTranslation();

  return (
    <div className="nc-blast-radius">
      <h3 className="nc-blast-radius__title">{t("streams.blastRadiusTitle")}</h3>
      <p className="nc-blast-radius__subtitle">{t("streams.blastRadiusSubtitle")}</p>

      {loading && !data ? (
        <p className="nc-blast-radius__status">{t("streams.blastRadiusLoading")}</p>
      ) : null}

      {error && !data ? (
        <p className="nc-blast-radius__status nc-blast-radius__status--error" role="status">
          {t("streams.blastRadiusFailed")}
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
            <div className="nc-blast-radius__section-label">{t("streams.blastRadiusImpact")}</div>
            <div className="nc-blast-radius__table-wrap">
              <table className="nc-blast-radius__table">
                <thead>
                  <tr>
                    <th scope="col">{t("streams.blastRadiusColType")}</th>
                    <th scope="col">{t("streams.blastRadiusColCount")}</th>
                    <th scope="col">{t("streams.blastRadiusColNames")}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td>{t("streams.blastRadiusServices")}</td>
                    <td className="nc-blast-radius__count">{data.services}</td>
                    <td>
                      <code>{formatNames(data.serviceNames ?? [])}</code>
                    </td>
                  </tr>
                  <tr>
                    <td>{t("streams.blastRadiusStreams")}</td>
                    <td className="nc-blast-radius__count">{data.streams}</td>
                    <td>
                      <code>{formatNames(data.relatedStreams ?? [])}</code>
                    </td>
                  </tr>
                  <tr>
                    <td>{t("streams.blastRadiusConsumers")}</td>
                    <td className="nc-blast-radius__count">{data.consumers}</td>
                    <td>
                      <code>{formatNames(data.consumerNames ?? [])}</code>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          {data.critical.length > 0 ? (
            <div className="nc-blast-radius__section">
              <div className="nc-blast-radius__section-label nc-blast-radius__section-label--critical">
                {t("streams.blastRadiusCritical")}
              </div>
              <ul className="nc-blast-radius__critical">
                {data.critical.map((name) => (
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
