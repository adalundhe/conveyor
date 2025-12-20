package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/adalundhe/micron"
	"github.com/adalundhe/micron/config"
	jwtmiddleware "github.com/adalundhe/micron/middleware/jwt"
	sso "github.com/adalundhe/micron/middleware/sso"
	"github.com/adalundhe/micron/routes"
	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/emailaddress"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	jujuErr "github.com/juju/errors"
)

type OrderStatusType string

const (
	Created OrderStatusType = "created"
	Accepted OrderStatusType = "accepted"
	Rejected OrderStatusType = "rejected"
	Queued OrderStatusType = "queued"
	WorkStarted OrderStatusType = "work_started"
	WorkComplete OrderStatusType = "work_complete"
	Completed OrderStatusType = "completed"
	Delivered OrderStatusType = "delivered"
)

type Metadata struct {
	UserId string `json:"user_id"`
}

type Actor struct {
	ClientId string `json:"client_id"`
}

type Claims struct {
	Audience string `json:"aud"`
	Issuer string `json:"iss"`
	Subject string `json:"sub"`
	NotBefore int64 `json:"nbf"`
	IssuedAt int64 `json:"iat"`
	Expires int64 `json:"exp"`
	MayAct *Actor `json:"may_act"`
	Additional *Metadata `json:"addl"`
}

func (c *Claims) GetAudience() (jwt.ClaimStrings, error) {
	return jwt.ClaimStrings{c.Audience}, nil
}

func (c *Claims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.Expires, 0)), nil
}

func (c *Claims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.IssuedAt, 0)), nil
}

func (c *Claims) GetNotBefore() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.NotBefore, 0)), nil
}

func (c *Claims) GetSubject() (string, error) {
	return  c.Subject, nil
}

func (c *Claims) GetIssuer() (string, error) {
	return  c.Issuer, nil
}

func includes(value string, values []string) bool {
	for _, val := range values {
		if val == value {
			return true
		}
	}

	return  false
}


type SSOClaims struct{
	Subject string `json:"sub"`
	Audience string `json:"aud"`
	NotBefore int64 `json:"nbf"`
	Expires int64 `json:"exp"`
}

func main() {
	
	app, err := micron.Create(&micron.App{
		Server: &micron.SeverOptions{
			Name: "Conveyor",
			Description: "A service for scheduling and orchestrating file transfers",
			Version: "v1",
			Port: 5051,
			TLSPort: 5443,
			HealthCheckPort: 5052,
		},
		Build: func(ctx context.Context, router *routes.Router, cfg *config.Config) (*routes.Router, error) {


			ssoAuth, err := sso.CreateSSOMiddlewareAndHandlers(cfg, func() interface{} {
				return &SSOClaims{}
			})

			if err != nil {
				return nil, err
			}

			router.AddRoute("/test", "GET", routes.RouteConfig{
				Endpoint: func(ctx *gin.Context) (string, error) {
					return "OK", nil
				},
				StatusCode: 200,
				Middleware: []gin.HandlerFunc{
					jwtmiddleware.New[*Claims](jwtmiddleware.JWTMiddleware[*Claims]{
						Envs: cfg.DeployEnvs,
						AccessTokenTTL: time.Duration(30 * time.Minute),
						RefreshTokenTTL: time.Duration(48 * time.Hour),
						Domain: "conveyor.audio",
						AccessCookieName: "token",
						RefreshCookieName: "refresh",
						CSRFCookieName: "csrf",
						Secure: true,
						Parse: func(data interface{}) (*Claims, error) {

							rawClaims := data.(map[string]interface{})

							audience, ok := rawClaims["aud"].(string)
							if !ok {
								return nil, jujuErr.Forbidden
							}

							issuer, ok := rawClaims["iss"].(string)
							if !ok {
								return nil, jujuErr.Forbidden
							}

							subject, ok := rawClaims["sub"].(string)
							if !ok {
								return nil, jujuErr.Forbidden
							}

							mayAct, ok := rawClaims["may_act"].(map[string]interface{})
							if !ok {
								return nil, jujuErr.Forbidden
							}

							clientId, ok := mayAct["client_id"].(string)
							if !ok {
								return nil, jujuErr.Forbidden
							}

							additional, ok := rawClaims["addl"].(map[string]interface{})
							if !ok {
								return nil, jujuErr.Forbidden
							}

							userId, ok := additional["user_id"].(string)
							if !ok {
								return nil,  jujuErr.Forbidden
							}

							notBefore, ok := rawClaims["nbf"].(int64)
							if !ok {
								return nil, jujuErr.Forbidden
							}
							
							issuedAt, ok := rawClaims["iat"].(int64)
							if !ok {
								return nil, jujuErr.Forbidden
							}

							expiresAt, ok := rawClaims["exp"].(int64)
							if !ok {
								return nil, jujuErr.Forbidden
							}
							
							return &Claims{
								NotBefore: notBefore,
								IssuedAt: issuedAt,
								Expires: expiresAt,
								Audience: audience,
								Issuer: issuer,
								Subject: subject,
								MayAct: &Actor{
									ClientId: clientId,
								},
								Additional: &Metadata{
									UserId: userId,
								},
							}, nil
						},
						Build: func(claims *Claims, expiresAt, issuedAt, notBefore time.Time) (*Claims, error) {

							conveyorApi := "conveyor-api"
		
							return &Claims{
								NotBefore: notBefore.Unix(),
								IssuedAt: issuedAt.Unix(),
								Expires: expiresAt.Unix(),
								Audience: conveyorApi,
								Issuer: conveyorApi,
								Subject: claims.Subject,
								MayAct: &Actor{
									ClientId: claims.MayAct.ClientId,
								},
								Additional: &Metadata{
									UserId: claims.Additional.UserId,
								},
							}, nil

						},
						CreateEmpty: func() *Claims {
							return &Claims{}
						},
						Verify: func(ctx *gin.Context, claims *Claims) (*Claims, error) {

							user, err := user.Get(ctx, claims.Additional.UserId)
							if err != nil {
								return nil, jujuErr.NewUnauthorized(err, "User not authorized")
							}

							if user.Banned {
								return nil, jujuErr.Unauthorized
							}

							userEmail, err := emailaddress.Get(ctx, *user.PrimaryEmailAddressID)
							if err != nil {
								return nil, jujuErr.NewUnauthorized(err, "User not authorized")
							}
							
							if _, err := micron.Stores.UserRepo.GetUserByEmail(ctx.Request.Context(), userEmail.EmailAddress); err != nil {
								return nil, jujuErr.NewUnauthorized(err, "User not authorized")
							}

							now := time.Now().Unix()
							if claims.Expires < now {
								return nil, jujuErr.Unauthorized
							}

							if claims.NotBefore > now {
								return nil, jujuErr.Unauthorized
							}

							if claims.IssuedAt > now {
								return nil, jujuErr.Unauthorized
							}

							if !includes(claims.Issuer, micron.Config.Api.Auth.AllowedIssuers) {
								return nil, jujuErr.Unauthorized
							}

							if claims.Audience != "conveyor-api" {
								return nil, jujuErr.Unauthorized
							}

							if !includes(claims.MayAct.ClientId, micron.Config.Api.Auth.AllowedIssuers) {
								return nil, jujuErr.Unauthorized
							}


							requestPath := ctx.Request.URL.Path
							requestMethod := ctx.Request.Method

							micron.Providers.Casbin.LoadPolicy()
							ok, err := micron.Providers.Casbin.Enforce(
								userEmail.EmailAddress,
								requestPath,
								requestMethod,
							)

							if err != nil {
								slog.Error("Error enforcing policy", slog.Any("error", err))
							}

							if !ok {
								return nil, jujuErr.Forbidden
							}

							return &Claims{
								Audience: claims.Audience,
								Subject: *user.PrimaryEmailAddressID,
								Issuer: claims.Issuer,
								NotBefore: claims.NotBefore,
								IssuedAt: claims.IssuedAt,
								Expires: claims.Expires,
								MayAct: claims.MayAct,
								Additional: &Metadata{
									UserId: user.ID,
								},
							}, nil
						},
						SignerName: cfg.Api.Env,
					}),
					ssoAuth.GetMiddlewareHandler(),
				},
			})

		
			return router, nil
		},
	})

	if err != nil {
		log.Fatalf("Encountered error creating Conveyor instance: %s", err.Error())
	}

	if err := app.Run(func() error {
		clerk.SetKey(os.Getenv("CLERK_API_KEY"))
		return nil
	}); err != nil {
		log.Fatalf("Encountered error running Conveyor: %s", err.Error())
	}

}