# Salesforce REST API client written in Go

## Installation
```
go get github.com/k-capehart/go-salesforce
```

## Authentication

https://help.salesforce.com/s/articleView?id=sf.remoteaccess_oauth_flows.htm&type=5

```go
type Salesforce struct {
    auth *Auth
}

type Auth struct {
    AccessToken string `json:"access_token"`
    InstanceUrl string `json:"instance_url"`
    Id          string `json:"id"`
    IssuedAt    string `json:"issued_at"`
    Signature   string `json:"signature"`
}

type Creds struct {
    Domain         string
    Username       string
    Password       string
    SecurityToken  string
    ConsumerKey    string
    ConsumerSecret string
}
```

<br>

`func Init(creds Creds) *Salesforce {}`

#### Username-Password Flow
- Create a Connected App in your Salesforce org: https://help.salesforce.com/s/articleView?id=sf.connected_app_create.htm&type=5

```go
sf := salesforce.Init(salesforce.Creds{
    Domain:         {DOMAIN},
    Username:       {USERNAME},
    Password:       {PASSWORD},
    SecurityToken:  {SECURITY_TOKEN},
    ConsumerKey:    {CONSUMER_KEY},
    ConsumerSecret: {CONSUMER_SECRET},
})
```

## SOQL Query

https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_query.htm

```go
type QueryResponse struct {
	TotalSize int              `json:"totalSize"`
	Done      bool             `json:"done"`
	Records   []map[string]any `json:"records"`
}
```

<br>

`func (sf *Salesforce) Query(query string) *QueryResponse {}`

```go
type Opportunity struct {
	Id        string
	Name      string
	IsPrivate bool
}
```

```go
queryResult := sf.Query("SELECT Id, Name, IsPrivate FROM Opportunity LIMIT 1")
var opp []Opportunity
err := mapstructure.Decode(queryResult.Records, &opp)
if err != nil {
    panic(err)
}
```