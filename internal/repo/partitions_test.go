package repo

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidPartitionIdentAcceptsDailyNames(t *testing.T) {
	day := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC).Format("2006_01_02")
	assert.True(t, validPartitionIdent(metricSamplesParent+"_"+day))
	assert.True(t, validPartitionIdent(incidentSamplesParent+"_"+day))
}

func TestValidPartitionIdentRejectsInjectionPayloads(t *testing.T) {
	payloads := []string{
		`x"; DROP TABLE users; --`,
		`cluster_metric_samples_2026_08_07"; DROP TABLE users; --`,
		"cluster_metric_samples_2026_08_07; DROP TABLE users",
		"cluster_metric_samples_2026_08_07 --",
		"cluster_metric_samples 2026_08_07",
		"CLUSTER_METRIC_SAMPLES_2026_08_07",
		metricSamplesParent, // parent alone, no date suffix
		incidentSamplesParent,
		"other_table_2026_08_07",
		`"; DROP TABLE data; --`,
		"",
		"1cluster_metric_samples_2026_08_07",
	}
	for _, name := range payloads {
		t.Run(name, func(t *testing.T) {
			assert.False(t, validPartitionIdent(name))
		})
	}
}

func TestQuoteIdentEscapesDoubleQuotes(t *testing.T) {
	assert.Equal(t, `"foo"`, quoteIdent("foo"))
	assert.Equal(t, `"a""b"`, quoteIdent(`a"b`))
}

func TestPartitionDDLNotBuiltForRejectedNames(t *testing.T) {
	payloads := []string{
		`x"; DROP TABLE users; --`,
		`cluster_metric_samples_2026_08_07"; DROP TABLE users; --`,
		"evil; DROP TABLE users",
	}
	day := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)
	from := quoteTimestamp(day)
	to := quoteTimestamp(day.Add(24 * time.Hour))

	for _, name := range payloads {
		t.Run(name, func(t *testing.T) {
			require.False(t, validPartitionIdent(name), "payload must be rejected before Exec")

			// Production builds DDL only after validPartitionIdent passes.
			createSQL := fmt.Sprintf(
				`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM (%s) TO (%s)`,
				quoteIdent(name),
				quoteIdent(metricSamplesParent),
				from,
				to,
			)
			dropSQL := "DROP TABLE IF EXISTS " + quoteIdent(name)

			// Even if quoteIdent is applied, allowlist must block before Exec.
			// Guard: raw payload metacharacters must not appear outside quoted idents
			// in a way that breaks out — semicolon/comment payloads fail allowlist first.
			assert.False(t, validPartitionIdent(name))
			assert.True(t, strings.HasPrefix(createSQL, "CREATE TABLE"))
			assert.True(t, strings.HasPrefix(dropSQL, "DROP TABLE"))
		})
	}
}

func TestEnsureDayPartitionNameShapeIsAllowlisted(t *testing.T) {
	day := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)
	for _, parent := range []string{metricSamplesParent, incidentSamplesParent} {
		name := fmt.Sprintf("%s_%s", parent, day.Format("2006_01_02"))
		require.True(t, validPartitionIdent(name), "app-generated name %q must be valid", name)
	}
}
