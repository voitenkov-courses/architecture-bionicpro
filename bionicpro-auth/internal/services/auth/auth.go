package auth

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	oauth2 "golang.org/x/oauth2"

	"github.com/voitenkov-courses/architecture-bionicpro/bionicpro-auth/internal/config"
)

type AuthContext struct {
	Cfg *config.Config
	// Конфиг OAuth2.0 для клиента, который отправляет запросы на авторизацию и получение токенов.
	Oauth2Config *oauth2.Config

	// OpenID-провайдера (в нашем случае — Keycloak)
	OidcProvider *oidc.Provider
}

func NewAuthContext(cfg *config.Config) *AuthContext {
	return &AuthContext{
		Cfg: cfg,
	}
}

func (ac *AuthContext) InitOIDC(host string) error {
	var err error

	keycloackRealmURL := ac.Cfg.KeycloakURL + "/realms/" + ac.Cfg.KeycloakRealm

	ac.OidcProvider, err = oidc.NewProvider(context.Background(), keycloackRealmURL)
	if err != nil {
		return err
	}

	ac.Oauth2Config = &oauth2.Config{
		ClientID:     ac.Cfg.ClientID,     // id клиента в Keycloak
		ClientSecret: ac.Cfg.ClientSecret, // секрет клиента в Keycloak
		Endpoint:     ac.OidcProvider.Endpoint(),
		RedirectURL:  ac.Cfg.CallbackURL, // Адрес, куда Keycloak вернёт пользователя после логина.
		Scopes: []string{
			oidc.ScopeOpenID,        // обязательно для OIDC
			oidc.ScopeOfflineAccess, // чтобы получить Refresh
			"profile", "email",      // чтобы получить имя и email в ID Token.
		},
	}

	return nil
}

func (ac *AuthContext) GetLogoutURL() string {
	return ac.Cfg.KeycloakExternalURL + "/realms/" + ac.Cfg.KeycloakRealm + "/protocol/openid-connect/logout"
}
