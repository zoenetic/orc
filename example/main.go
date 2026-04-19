//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/zoenetic/orc"
	"github.com/zoenetic/orc/runner"
)

// Plans holds all runbook plans for this project.
// Each exported method returning *orc.Runbook is a named plan, invoked with:
//
//	go run . run     [plan]     — execute
//	go run . preview [plan]     — show what would run
//	go run . rollback [plan]    — run undo commands in reverse order
//	go run . validate [plan]    — check for dependency cycles / errors
type Plans struct {
	*orc.Runbook
}

func main() {
	// Pass runner.Default for full TUI output (spinners, colours, stage view).
	// Useful when running locally or in a terminal that supports ANSI.
	orc.Main[Plans](runner.Default)

	// For plain text output — e.g. piped into a log file or running in CI:
	//   orc.Main[Plans]()
}

// getenv returns the value of key, or fallback if unset or empty.
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// =============================================================================
// Plan: Default
// A full staging deployment pipeline. Invoked when no plan name is given.
//
// Demonstrates: multi-stage parallelism, SkipIf, RunIf, CheckFn, Undo, Use.
// =============================================================================

func (p *Plans) Default() *orc.Runbook {
	deployEnv := getenv("DEPLOY_ENV", "staging")
	ns := deployEnv
	dbHost := fmt.Sprintf("db.%s.internal", deployEnv)
	chartVersion := getenv("CHART_VERSION", "1.4.2")

	p.Runbook = orc.New("deploy staging", orc.Options{Concurrency: 4})

	p.Use("kubectl", "1.29")
	p.Use("helm", "3.14")
	p.Use("psql", "16")
	p.Use("vault", "1.15")

	// -------------------------------------------------------------------------
	// Stage 1 — pre-flight checks (all run concurrently, no dependencies)
	// -------------------------------------------------------------------------

	clusterOK := p.Task("cluster reachable",
		orc.Do(orc.Sh("kubectl", "cluster-info", "--request-timeout=5s")),
	)

	dbOK := p.Task("database reachable",
		orc.Do(orc.Sh("psql", "--host", dbHost, "--command", "SELECT 1")),
	)

	vaultOK := p.Task("vault reachable",
		orc.Do(orc.Sh("vault", "status")),
	)

	// -------------------------------------------------------------------------
	// Stage 2 — namespace + secrets (parallel, each gated on its pre-flight)
	// -------------------------------------------------------------------------

	// SkipIf: skip task if exit code is 0 (resource already exists).
	nsTask := p.Task("create namespace",
		orc.Do(orc.Sh("kubectl", "create", "namespace", ns)),
		orc.Undo(orc.Sh("kubectl", "delete", "namespace", ns, "--ignore-not-found")),
		orc.SkipIf(orc.Sh("kubectl", "get", "namespace", ns)),
		orc.DependsOn(clusterOK),
	)

	// RunIf: skip task if exit code is non-zero (precondition not met).
	// Both SkipIf and RunIf can be combined: the first that triggers wins.
	secretsTask := p.Task("load secrets",
		orc.Do(
			orc.ShF("vault kv get -field=db_url secret/%s", ns),
			orc.ShF("kubectl create secret generic app-secrets -n %s --from-literal=db_url=$(vault kv get -field=db_url secret/%s)", ns, ns),
		),
		orc.Undo(orc.ShF("kubectl delete secret app-secrets -n %s --ignore-not-found", ns)),
		orc.SkipIf(orc.ShF("kubectl get secret app-secrets -n %s", ns)),
		orc.RunIf(orc.Sh("vault", "status")),
		orc.DependsOn(nsTask, vaultOK),
	)

	// -------------------------------------------------------------------------
	// Stage 3 — deploy chart + run migrations (parallel, both start once secrets are ready)
	// -------------------------------------------------------------------------

	// CheckFn: programmatic skip — skip if this exact chart version is already running.
	chartTask := p.Task("deploy chart",
		orc.Do(
			orc.Sh("helm", "repo", "add", "myrepo", "https://charts.example.com"),
			orc.Sh("helm", "repo", "update"),
			orc.ShF("helm upgrade --install myapp myrepo/myapp -n %s --version %s --wait", ns, chartVersion),
		),
		orc.Undo(orc.ShF("helm uninstall myapp -n %s --wait", ns)),
		orc.CheckFn(func(ctx context.Context) (bool, error) {
			out, err := exec.CommandContext(ctx,
				"helm", "list", "-n", ns, "-f", "myapp", "--output", "json",
			).Output()
			if err != nil {
				return false, nil
			}
			return strings.Contains(string(out), chartVersion), nil
		}),
		orc.DependsOn(secretsTask),
	)

	migrationsTask := p.Task("run migrations",
		orc.Do(orc.Sh("psql", "--host", dbHost, "--file", "migrations/latest.sql")),
		orc.Undo(orc.Sh("psql", "--host", dbHost, "--file", "migrations/rollback.sql")),
		orc.CheckFn(func(ctx context.Context) (bool, error) {
			out, err := exec.CommandContext(ctx,
				"psql", "--host", dbHost, "--no-psqlrc", "--tuples-only",
				"--command", "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1",
			).Output()
			if err != nil {
				return false, nil
			}
			return strings.TrimSpace(string(out)) == "20240101_latest", nil
		}),
		orc.DependsOn(secretsTask, dbOK),
	)

	// -------------------------------------------------------------------------
	// Stage 4 — verify
	// -------------------------------------------------------------------------

	rolloutTask := p.Task("verify rollout",
		orc.Do(orc.ShF("kubectl rollout status deployment/myapp -n %s --timeout=120s", ns)),
		orc.DependsOn(chartTask, migrationsTask),
	)

	// -------------------------------------------------------------------------
	// Stage 5 — smoke tests (concurrent)
	// -------------------------------------------------------------------------

	apiSmoke := p.Task("smoke test api",
		orc.Do(orc.ShF("curl -sf https://api.%s.example.com/health", deployEnv)),
		orc.DependsOn(rolloutTask),
	)

	workerSmoke := p.Task("smoke test worker",
		orc.Do(orc.ShF("curl -sf https://worker.%s.example.com/health", deployEnv)),
		orc.DependsOn(rolloutTask),
	)

	// -------------------------------------------------------------------------
	// Stage 6 — finalize
	// -------------------------------------------------------------------------

	_ = p.Task("notify",
		orc.Do(
			orc.Sh("curl", "-sf", "-X", "POST",
				"https://hooks.slack.com/services/T000/B000/xxxx",
				"-H", "Content-Type: application/json",
				"-d", fmt.Sprintf(`{"text":"✅ myapp %s deployed to %s"}`, chartVersion, deployEnv),
			),
			orc.Sh("kubectl", "annotate", "namespace", ns,
				fmt.Sprintf("deploy-version=%s", chartVersion), "--overwrite"),
		),
		orc.DependsOn(apiSmoke, workerSmoke),
	)

	return p.Runbook
}

// =============================================================================
// Plan: Provision
// Provisions cloud infrastructure with Terraform.
//
// Demonstrates: sequential single-stage pipeline, Undo at multiple steps,
// SkipIf for idempotent init.
// =============================================================================

func (p *Plans) Provision() *orc.Runbook {
	deployEnv := getenv("DEPLOY_ENV", "staging")
	tfVars := fmt.Sprintf("env/%s.tfvars", deployEnv)
	tfBackend := fmt.Sprintf("env/%s.tfbackend", deployEnv)

	p.Runbook = orc.New("provision infrastructure", orc.Options{Concurrency: 1})

	p.Use("terraform", "1.7")
	p.Use("aws", "2.x")

	init := p.Task("terraform init",
		orc.Do(orc.Sh("terraform", "init", "-reconfigure", "-backend-config="+tfBackend)),
		orc.SkipIf(orc.Sh("test", "-d", ".terraform")),
	)

	validate := p.Task("terraform validate",
		orc.Do(orc.Sh("terraform", "validate")),
		orc.DependsOn(init),
	)

	plan := p.Task("terraform plan",
		orc.Do(orc.ShF("terraform plan -out=tfplan -var-file=%s", tfVars)),
		orc.DependsOn(validate),
	)

	apply := p.Task("terraform apply",
		orc.Do(orc.Sh("terraform", "apply", "tfplan")),
		orc.Undo(orc.ShF("terraform destroy -var-file=%s -auto-approve", tfVars)),
		orc.DependsOn(plan),
	)

	_ = p.Task("write outputs",
		orc.Do(
			orc.Sh("terraform", "output", "-json"),
			orc.Sh("sh", "-c", "terraform output -json > .orc/tf-outputs.json"),
		),
		orc.DependsOn(apply),
	)

	return p.Runbook
}

// =============================================================================
// Plan: Release
// Builds, tests, and publishes a new release.
//
// Demonstrates: parallel Stage 1 (unit tests, integration tests, lint all run
// concurrently), then sequential build → push → tag.
// =============================================================================

func (p *Plans) Release() *orc.Runbook {
	version := getenv("VERSION", "v1.4.2")
	registry := getenv("REGISTRY", "registry.example.com")
	image := fmt.Sprintf("%s/myapp:%s", registry, version)

	p.Runbook = orc.New("release "+version, orc.Options{Concurrency: 4})

	p.Use("docker", "24.0")
	p.Use("go", "1.22")
	p.Use("golangci-lint", "1.56")

	// -------------------------------------------------------------------------
	// Stage 1 — all checks run concurrently
	// -------------------------------------------------------------------------

	unitTests := p.Task("unit tests",
		orc.Do(orc.Sh("go", "test", "-race", "-count=1", "./...")),
	)

	integrationTests := p.Task("integration tests",
		orc.Do(orc.Sh("go", "test", "-tags=integration", "-count=1", "./...")),
	)

	lint := p.Task("lint",
		orc.Do(orc.Sh("golangci-lint", "run", "--timeout=5m")),
	)

	// -------------------------------------------------------------------------
	// Stage 2 — build image (blocked on all checks)
	// -------------------------------------------------------------------------

	buildTask := p.Task("build image",
		orc.Do(
			orc.Sh("docker", "build",
				"--build-arg", "VERSION="+version,
				"--label", "version="+version,
				"-t", image, "."),
		),
		orc.Undo(orc.Sh("docker", "rmi", "--force", image)),
		orc.DependsOn(unitTests, integrationTests, lint),
	)

	// -------------------------------------------------------------------------
	// Stage 3 — push + tag (both depend on build, run in parallel)
	// -------------------------------------------------------------------------

	// SkipIf: skip if this exact image digest is already in the registry.
	pushTask := p.Task("push image",
		orc.Do(
			orc.Sh("docker", "push", image),
		),
		orc.SkipIf(orc.Sh("docker", "manifest", "inspect", image)),
		orc.DependsOn(buildTask),
	)

	tagTask := p.Task("tag release",
		orc.Do(
			orc.Sh("git", "tag", "--annotate", version, "--message", "Release "+version),
			orc.Sh("git", "push", "origin", version),
		),
		orc.Undo(
			orc.Sh("git", "tag", "--delete", version),
			orc.Sh("git", "push", "origin", "--delete", version),
		),
		orc.SkipIf(orc.Sh("git", "rev-parse", version)),
		orc.DependsOn(buildTask),
	)

	// -------------------------------------------------------------------------
	// Stage 4 — notify
	// -------------------------------------------------------------------------

	_ = p.Task("notify",
		orc.Do(
			orc.Sh("curl", "-sf", "-X", "POST",
				"https://hooks.slack.com/services/T000/B000/xxxx",
				"-H", "Content-Type: application/json",
				"-d", fmt.Sprintf(`{"text":"🚀 %s released: %s"}`, "myapp", version),
			),
		),
		orc.DependsOn(pushTask, tagTask),
	)

	return p.Runbook
}

// =============================================================================
// Plan: Migrate
// Runs database migrations in isolation.
//
// Demonstrates: a focused single-concern plan, CheckFn for querying migration
// state, Undo for rollback safety.
// =============================================================================

func (p *Plans) Migrate() *orc.Runbook {
	dbHost := getenv("DB_HOST", "db.staging.internal")
	dbName := getenv("DB_NAME", "myapp")
	dsn := fmt.Sprintf("postgres://%s/%s", dbHost, dbName)

	p.Runbook = orc.New("database migrations", orc.Options{Concurrency: 1})

	p.Use("psql", "16")
	p.Use("migrate", "4.17")

	ping := p.Task("ping database",
		orc.Do(orc.Sh("psql", "--host", dbHost, "--dbname", dbName, "--command", "SELECT 1")),
	)

	// Back up schema before touching anything.
	backup := p.Task("backup schema",
		orc.Do(
			orc.Sh("pg_dump", "--schema-only",
				"--host", dbHost, "--dbname", dbName,
				"--file", "/tmp/schema-pre-migration.sql"),
		),
		orc.DependsOn(ping),
	)

	migrate := p.Task("apply migrations",
		orc.Do(orc.ShF("migrate -path ./migrations -database %s up", dsn)),
		orc.Undo(orc.ShF("migrate -path ./migrations -database %s down 1", dsn)),
		// CheckFn lets you query real state to decide whether to skip.
		orc.CheckFn(func(ctx context.Context) (bool, error) {
			out, err := exec.CommandContext(ctx,
				"migrate", "-path", "./migrations",
				"-database", dsn,
				"version",
			).Output()
			if err != nil {
				return false, nil // can't tell — run migrations
			}
			// "migrate version" prints "N\n" or "N (dirty)\n"
			return strings.HasPrefix(strings.TrimSpace(string(out)), "20") &&
				!strings.Contains(string(out), "dirty"), nil
		}),
		orc.DependsOn(backup),
	)

	_ = p.Task("verify schema",
		orc.Do(
			orc.Sh("psql", "--host", dbHost, "--dbname", dbName,
				"--command", "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 5"),
		),
		orc.DependsOn(migrate),
	)

	return p.Runbook
}
