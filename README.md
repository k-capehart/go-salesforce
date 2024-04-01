# Salesforce REST API client written in Go

A very simple REST API wrapper for interacting with Salesforce within the Go programming language.

- [Read about the Salesforce REST API](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/intro_rest.htm)

- [Read about Golang](https://go.dev/doc/)

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

**Username-Password Flow**
- [Create a Connected App in your Salesforce org](https://help.salesforce.com/s/articleView?id=sf.connected_app_create.htm&type=5)

Example:

```go
sf := salesforce.Init(salesforce.Creds{
    Domain:         DOMAIN,
    Username:       USERNAME,
    Password:       PASSWORD,
    SecurityToken:  SECURITY_TOKEN,
    ConsumerKey:    CONSUMER_KEY,
    ConsumerSecret: CONSUMER_SECRET,
})
```

## SOQL Query

https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_query.htm

<br>

`func (sf *Salesforce) Query(query string) *QueryResponse {}`

Examples:

**Passing a query string**
```go
type Opportunity struct {
	Id        string
	Name      string
	StageName string
}
```

```go
opps := []Opportunity{}
err := sf.Query("SELECT Id, Name, StageName FROM Opportunity WHERE StageName = 'Prospecting'", &opps)
if err != nil {
    fmt.Println(err)
} else {
    fmt.Println(opps)
}
```

**Using go-soql**
- Salesforce's package for marshalling go structs into SOQL
- Review [forcedotcom/go-soql](https://github.com/forcedotcom/go-soql) for details
- Eliminates need to separately maintain query string and struct
- Helps prevent SOQL injection

```go
type Opportunity struct {
	Id        string `soql:"selectColumn,fieldName=Id" json:"Id"`
	Name      string `soql:"selectColumn,fieldName=Name" json:"Name"`
	StageName string `soql:"selectColumn,fieldName=StageName" json:"StageName"`
}

type OpportunityQueryCriteria struct {
	StageName string `soql:"equalsOperator,fieldName=StageName"`
}

type OpportunitySoqlQuery struct {
	SelectClause Opportunity              `soql:"selectClause,tableName=Opportunity"`
	WhereClause  OpportunityQueryCriteria `soql:"whereClause"`
}
```
```go
soqlStruct := OpportunitySoqlQuery{
    SelectClause: Opportunity{},
    WhereClause: OpportunityQueryCriteria{
        StageName: "Prospecting",
    },
}

opps := []Opportunity{}
err := sf.QueryStruct(soqlStruct, &opps)
if err != nil {
    fmt.Println(err)
} else {
    fmt.Println(opps)
}
```