package salesforce

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AuthFlowType represents the type of authentication flow used
type AuthFlowType int

const (
	AuthFlowUnknown AuthFlowType = iota
	AuthFlowUsernamePassword
	AuthFlowClientCredentials
	AuthFlowAccessToken
	AuthFlowJWT
)

func (a AuthFlowType) String() string {
	switch a {
	case AuthFlowUsernamePassword:
		return "Username/Password"
	case AuthFlowClientCredentials:
		return "Client Credentials"
	case AuthFlowAccessToken:
		return "Access Token"
	case AuthFlowJWT:
		return "JWT"
	default:
		return "Unknown"
	}
}

type authentication struct {
	AccessToken string `json:"access_token"`
	InstanceUrl string `json:"instance_url"`
	Id          string `json:"id"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	IssuedAt    string `json:"issued_at"`
	Signature   string `json:"signature"`
	grantType   string
	creds       Creds
}

type Creds struct {
	Domain         string
	Username       string
	Password       string
	SecurityToken  string
	ConsumerKey    string
	ConsumerSecret string
	ConsumerRSAPem string
	AccessToken    string
}

const JwtExpirationTime = 5 * time.Minute

const (
	grantTypeUsernamePassword  = "password"
	grantTypeClientCredentials = "client_credentials"
	grantTypeAccessToken       = "access_token"
	grantTypeJWT               = "urn:ietf:params:oauth:grant-type:jwt-bearer"
)

func validateAuth(sf Salesforce) error {
	if sf.auth == nil || sf.auth.AccessToken == "" {
		return errors.New("not authenticated: please use salesforce.Init()")
	}
	return nil
}

func (conf *configuration) validateAuthentication(auth authentication) error {
	if err := validateAuth(Salesforce{auth: &auth}); err != nil {
		return err
	}
	_, err := doRequest(&auth, conf, requestPayload{
		method:  http.MethodGet,
		uri:     "/limits",
		content: jsonType,
	})
	if err != nil {
		return err
	}

	return nil
}

func refreshSession(auth *authentication) error {
	var refreshedAuth *authentication
	var err error

	switch grantType := auth.grantType; grantType {
	case grantTypeClientCredentials:
		refreshedAuth, err = clientCredentialsFlow(
			auth.InstanceUrl,
			auth.creds.ConsumerKey,
			auth.creds.ConsumerSecret,
		)
	case grantTypeUsernamePassword:
		refreshedAuth, err = usernamePasswordFlow(
			auth.InstanceUrl,
			auth.creds.Username,
			auth.creds.Password,
			auth.creds.SecurityToken,
			auth.creds.ConsumerKey,
			auth.creds.ConsumerSecret,
		)
	case grantTypeJWT:
		refreshedAuth, err = jwtFlow(
			auth.InstanceUrl,
			auth.creds.Username,
			auth.creds.ConsumerKey,
			auth.creds.ConsumerRSAPem,
			JwtExpirationTime,
		)
	default:
		return errors.New("invalid session, unable to refresh session")
	}

	if err != nil {
		return err
	}

	if refreshedAuth == nil {
		return errors.New("missing refresh auth")
	}

	auth.AccessToken = refreshedAuth.AccessToken
	auth.IssuedAt = refreshedAuth.IssuedAt
	auth.Signature = refreshedAuth.Signature
	auth.Id = refreshedAuth.Id

	return nil
}

func doAuth(url string, body *strings.Reader) (*authentication, error) {
	resp, err := http.Post(url, "application/x-www-form-urlencoded", body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(string(resp.Status) + ":" + " failed authentication")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	auth := &authentication{}
	jsonError := json.Unmarshal(respBody, &auth)
	if jsonError != nil {
		return nil, jsonError
	}

	defer func() {
		_ = resp.Body.Close() // Ignore error since we've already read what we need
	}()
	return auth, nil
}

func usernamePasswordFlow(
	domain string,
	username string,
	password string,
	securityToken string,
	consumerKey string,
	consumerSecret string,
) (*authentication, error) {
	payload := url.Values{
		"grant_type":    {grantTypeUsernamePassword},
		"client_id":     {consumerKey},
		"client_secret": {consumerSecret},
		"username":      {username},
		"password":      {password + securityToken},
	}
	endpoint := "/services/oauth2/token"
	body := strings.NewReader(payload.Encode())
	auth, err := doAuth(domain+endpoint, body)
	if err != nil {
		return nil, err
	}
	auth.grantType = grantTypeUsernamePassword
	return auth, nil
}

func clientCredentialsFlow(
	domain string,
	consumerKey string,
	consumerSecret string,
) (*authentication, error) {
	payload := url.Values{
		"grant_type":    {grantTypeClientCredentials},
		"client_id":     {consumerKey},
		"client_secret": {consumerSecret},
	}
	endpoint := "/services/oauth2/token"
	body := strings.NewReader(payload.Encode())
	auth, err := doAuth(domain+endpoint, body)
	if err != nil {
		return nil, err
	}
	auth.grantType = grantTypeClientCredentials
	return auth, nil
}

func (conf *configuration) getAccessTokenAuthentication(
	domain string,
	accessToken string,
) (*authentication, error) {
	auth := &authentication{InstanceUrl: domain, AccessToken: accessToken}
	if conf.shouldValidateAuthentication {
		if err := conf.validateAuthentication(*auth); err != nil {
			return nil, err
		}
	}
	auth.grantType = grantTypeAccessToken
	return auth, nil
}

func jwtFlow(
	domain string,
	username string,
	consumerKey string,
	consumerRSAPem string,
	expirationTime time.Duration,
) (*authentication, error) {
	audience := domain
	if strings.Contains(audience, "test.salesforce") || strings.Contains(audience, "sandbox") {
		audience = "https://test.salesforce.com"
	} else {
		audience = "https://login.salesforce.com"
	}
	claims := &jwt.MapClaims{
		"exp": strconv.Itoa(int(time.Now().Unix() + int64(expirationTime.Seconds()))),
		"aud": audience,
		"iss": consumerKey,
		"sub": username,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(consumerRSAPem))
	if err != nil {
		return nil, fmt.Errorf("ParseRSAPrivateKeyFromPEM: %w", err)
	}
	tokenString, err := token.SignedString(signKey)
	if err != nil {
		return nil, fmt.Errorf("jwt.SignedString: %w", err)
	}

	payload := url.Values{
		"grant_type": {grantTypeJWT},
		"assertion":  {tokenString},
	}
	endpoint := "/services/oauth2/token"
	body := strings.NewReader(payload.Encode())
	auth, err := doAuth(domain+endpoint, body)
	if err != nil {
		return nil, err
	}
	auth.grantType = grantTypeJWT
	return auth, nil
}
