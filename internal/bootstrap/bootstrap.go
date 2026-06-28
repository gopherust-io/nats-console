package bootstrap

import (
	"context"

	natsadapter "github.com/gopherust-io/nats-consol/internal/adapter/nats"
	"github.com/gopherust-io/nats-consol/internal/adapter/postgres"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/assistant"
	"github.com/gopherust-io/nats-consol/internal/audit"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/crypto"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type Application struct {
	Gateway     port.ClusterGateway
	UoW         *postgres.UnitOfWork
	NATSManager *natsclient.Manager
	Auth        *auth.Service
	Assistant   *assistant.Service
	AuditWriter *audit.Writer
	Services    *app.Services
	Config      config.Config
}

func New(ctx context.Context, cfg config.Config, encryptor *crypto.Encryptor) (*Application, error) {
	uow, err := postgres.OpenWithConfig(ctx, cfg, encryptor)
	if err != nil {
		return nil, err
	}

	authSvc, err := auth.NewService(cfg, uow.Raw())
	if err != nil {
		uow.Close()
		return nil, err
	}
	if err := authSvc.SeedAdmin(ctx); err != nil {
		uow.Close()
		return nil, err
	}

	manager := natsclient.NewManager(uow.Raw(), cfg)
	gateway := natsadapter.NewGateway(manager)

	services := app.NewServices(uow, gateway, authSvc, nil)

	if err := services.Cluster.BootstrapDefault(ctx); err != nil {
		uow.Close()
		manager.Close()
		return nil, err
	}

	assistantSvc, err := assistant.NewService(cfg, uow.Raw(), manager)
	if err != nil {
		uow.Close()
		manager.Close()
		return nil, err
	}
	services.Assistant = assistantSvc

	return &Application{
		Config:      cfg,
		UoW:         uow,
		Gateway:     gateway,
		NATSManager: manager,
		Auth:        authSvc,
		Assistant:   assistantSvc,
		AuditWriter: audit.NewWriter(uow.Raw()),
		Services:    services,
	}, nil
}

func (a *Application) Close() {
	if a == nil {
		return
	}
	if a.Gateway != nil {
		a.Gateway.Close()
	}
	if a.UoW != nil {
		a.UoW.Close()
	}
}
