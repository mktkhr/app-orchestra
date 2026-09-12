// Command api is the platform's entry point: read the environment, wire the
// object graph, serve HTTP until the process is killed.
package main

import (
	"log/slog"
	"os"

	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/config"
	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/httpserver"
	"github.com/mktkhr/app-orchestra/services/platform/pkg/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("loading configuration", slog.Any("error", err))
		os.Exit(1)
	}

	handler, err := app.New(&app.Config{
		StaticDir:     cfg.StaticDir,
		Services:      toAppServices(cfg.Services),
		LLM:           app.LLM{BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, Model: cfg.LLMModel, Mode: cfg.LLMMode},
		PlanFixtures:  toAppPlanFixtures(cfg.PlanFixtures),
		DBPath:        cfg.DBPath,
		SecureCookie:  cfg.SecureCookie,
		AdminPassword: cfg.AdminPassword,
		SeedAccounts:  toAppSeedAccounts(cfg.SeedAccounts),
	})
	if err != nil {
		logger.Error("building the platform", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("platform listening", slog.Int("port", cfg.Port))

	if err := httpserver.Run(httpserver.NewServer(cfg.Port, handler)); err != nil {
		logger.Error("serving http", slog.Any("error", err))
		os.Exit(1)
	}
}

// toAppServices adapts config.Service to app.Service: cmd is the one place
// allowed to see both the infra config package and pkg/app's public
// surface, so the conversion lives here rather than making either package
// depend on the other's type.
func toAppServices(services []config.Service) []app.Service {
	out := make([]app.Service, 0, len(services))
	for _, s := range services {
		out = append(out, app.Service{Name: s.Name, URL: s.URL})
	}

	return out
}

// toAppPlanFixtures adapts config.PlanFixture to app.PlanFixture. See
// toAppServices; same reasoning, and see config.Config.PlanFixtures for
// why this exists at all - production never sets ORCHESTRA_PLAN_FIXTURES,
// so this path is empty on every real deployment.
func toAppPlanFixtures(fixtures []config.PlanFixture) []app.PlanFixture {
	out := make([]app.PlanFixture, 0, len(fixtures))
	for _, f := range fixtures {
		out = append(out, app.PlanFixture{
			Query:       f.Query,
			Answers:     toAppAnswers(f.Answers),
			Ask:         f.Ask,
			Question:    f.Question,
			Param:       f.Param,
			Service:     f.Service,
			OperationID: f.OperationID,
			Args:        f.Args,
		})
	}

	return out
}

// toAppAnswers adapts config.Answer to app.Answer. See toAppServices.
func toAppAnswers(answers []config.Answer) []app.Answer {
	out := make([]app.Answer, 0, len(answers))
	for _, a := range answers {
		out = append(out, app.Answer{Param: a.Param, Value: a.Value})
	}

	return out
}

// toAppSeedAccounts adapts config.SeedAccount to app.SeedAccount. See
// toAppServices; same reasoning, and see
// config.Config.SeedAccounts/app.Config.SeedAccounts for why this exists
// at all - production never sets ORCHESTRA_SEED_ACCOUNTS, so this path is
// empty on every real deployment.
func toAppSeedAccounts(accounts []config.SeedAccount) []app.SeedAccount {
	out := make([]app.SeedAccount, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, app.SeedAccount{Name: a.Name, Password: a.Password, Role: a.Role})
	}

	return out
}
